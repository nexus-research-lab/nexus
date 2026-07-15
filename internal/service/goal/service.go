// INPUT: Goal 创建、读取、Room creator/lead 身份与用户更新请求。
// OUTPUT: 持久化 Goal、不可变 creator/可转移 lead 审计身份与后续 runtime 决策。
// POS: Goal 应用服务主入口。
package goal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	maxGoalObjectiveRunes = 4000
	goalUpdateMaxAttempts = 3

	goalObjectiveEmptyMessage   = "goal objective must not be empty"
	goalObjectiveTooLongMessage = "goal objective must be at most 4000 characters"
	goalBudgetPositiveMessage   = "goal budgets must be positive when provided"
)

// Service 负责 Goal 状态机、审计事件和后续运行时决策。
type Service struct {
	config           config.Config
	repo             Repository
	events           eventBroadcaster
	guidance         guidanceDispatcher
	preview          previewFiller
	rewriter         objectiveRewriter
	externalMutation externalMutationAccountant
	runtimeInterrupt runtimeInterrupter
	continuations    ContinuationDispatcher
	wallClock        *goalWallClockAccounting
	nowFn            func() time.Time
	idFactory        func(string) string
}

// NewService 创建 Goal 服务。
func NewService(cfg config.Config, repo Repository) *Service {
	return &Service{
		config:    cfg,
		repo:      repo,
		wallClock: newGoalWallClockAccounting(),
		nowFn:     func() time.Time { return time.Now().UTC() },
		idFactory: newID,
	}
}

// Create 创建当前 Goal。
func (s *Service) Create(ctx context.Context, request protocol.CreateGoalRequest) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	sessionKey, objective, err := validateCreateRequest(request)
	if err != nil {
		return nil, err
	}
	if protocol.IsRoomSharedSessionKey(sessionKey) && strings.TrimSpace(request.CreatedBy) == "model" && strings.TrimSpace(request.AgentID) == "" {
		return nil, newGoalInvalidInputError("model-created Room Goal requires the current agent identity")
	}
	current, err := s.repo.GetCurrentGoal(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if current != nil {
		if !request.ReplaceExisting {
			return nil, ErrGoalConflict
		}
		metadata := request.Metadata
		if metadata == nil {
			metadata = map[string]any{}
		}
		return s.Update(ctx, current.ID, protocol.UpdateGoalRequest{
			Objective:   &objective,
			TokenBudget: protocol.OptionalInt64{Present: true, Value: request.TokenBudget},
			OwnerUserID: request.OwnerUserID,
			Metadata:    metadata,
		})
	}
	objective, metadata := s.rewriteCreateObjective(ctx, request, objective)
	if metadata != nil {
		metadata = cloneMap(metadata)
		delete(metadata, protocol.GoalMetadataObjectiveRevision)
	}
	metadata = initializeRoomGoalOwnershipMetadata(sessionKey, metadata, request.AgentID)

	now := s.nowFn()
	tokenBudget, err := normalizeCreateBudget(request.TokenBudget)
	if err != nil {
		return nil, err
	}
	item := protocol.Goal{
		ID:          s.idFactory("goal"),
		SessionKey:  sessionKey,
		Objective:   objective,
		Status:      protocol.GoalStatusActive,
		TokenBudget: tokenBudget,
		Version:     1,
		CreatedBy:   strings.TrimSpace(request.CreatedBy),
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    metadata,
	}
	created, err := s.repo.CreateGoal(ctx, item)
	if err != nil {
		return nil, err
	}
	s.updatePreviewFromGoal(ctx, *created, request.OwnerUserID)
	if err := s.appendEvent(ctx, *created, "created", createGoalEventSource(created.CreatedBy), strings.TrimSpace(request.RoundID), map[string]any{"objective": created.Objective}); err != nil {
		return nil, err
	}
	if strings.TrimSpace(created.CreatedBy) == "model" {
		s.markWallClockGoalActive(*created)
	} else {
		s.activateExternalGoalAccounting(ctx, *created)
	}
	s.maybeDispatchActiveGoalContinuation(ctx, *created)
	return created, nil
}

// Current 返回 session 当前 Goal。
func (s *Service) Current(ctx context.Context, sessionKey string) (*protocol.Goal, error) {
	item, err := s.CurrentOptional(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return item, nil
}

// CurrentOptional 返回 session 当前 Goal；没有 Goal 时返回 nil。
func (s *Service) CurrentOptional(ctx context.Context, sessionKey string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// Update 更新当前 Goal 文本、预算或 metadata。
func (s *Service) Update(ctx context.Context, goalID string, request protocol.UpdateGoalRequest) (*protocol.Goal, error) {
	s.prepareExternalMutation(ctx, strings.TrimSpace(goalID))
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	mutation, err := s.buildGoalUpdateMutation(ctx, item, request)
	if err != nil {
		return nil, err
	}
	if !mutation.changed {
		return item, nil
	}
	nextStatus := statusAfterUserGoalUpdate(item.Status, mutation.objectiveRequested)
	updated, err := s.persistTransition(ctx, *item, nextStatus, protocol.GoalUpdateSourceUser, "updated", "", mutation.payload)
	if err != nil {
		return nil, err
	}
	if mutation.objectiveRequested {
		s.updatePreviewFromGoal(ctx, *updated, request.OwnerUserID)
	}
	return s.reconcileUpdatedGoalBudget(ctx, updated)
}

type goalUpdateMutation struct {
	changed            bool
	objectiveRequested bool
	payload            map[string]any
}

func (s *Service) buildGoalUpdateMutation(
	ctx context.Context,
	item *protocol.Goal,
	request protocol.UpdateGoalRequest,
) (goalUpdateMutation, error) {
	objectiveRevision := item.ObjectiveRevision()
	mutation := goalUpdateMutation{
		objectiveRequested: request.Objective != nil,
		payload:            make(map[string]any),
	}
	if err := s.applyGoalObjectiveUpdate(ctx, item, request, &mutation); err != nil {
		return goalUpdateMutation{}, err
	}
	if err := applyGoalBudgetUpdate(item, request, &mutation); err != nil {
		return goalUpdateMutation{}, err
	}
	if request.Metadata != nil {
		item.Metadata = preserveRoomGoalOwnershipMetadata(*item, request.Metadata)
		delete(item.Metadata, protocol.GoalMetadataObjectiveRevision)
		if objectiveRevision > 1 {
			item.Metadata[protocol.GoalMetadataObjectiveRevision] = objectiveRevision
		}
		mutation.changed = true
		mutation.payload["metadata_updated"] = true
	}
	if eventPayloadBool(mutation.payload, "objective_updated") {
		item.Metadata = cloneMap(item.Metadata)
		if item.Metadata == nil {
			item.Metadata = map[string]any{}
		}
		item.Metadata[protocol.GoalMetadataObjectiveRevision] = objectiveRevision + 1
		mutation.payload["objective_revision"] = objectiveRevision + 1
	}
	return mutation, nil
}

func (s *Service) applyGoalObjectiveUpdate(
	ctx context.Context,
	item *protocol.Goal,
	request protocol.UpdateGoalRequest,
	mutation *goalUpdateMutation,
) error {
	if request.Objective == nil {
		return nil
	}
	objective, err := normalizeObjective(*request.Objective)
	if err != nil {
		return err
	}
	objective, mutation.payload = s.rewriteUpdateObjective(ctx, request, item.SessionKey, objective, mutation.payload)
	if item.Objective == objective {
		return nil
	}
	item.Objective = objective
	resetGoalContinuationForObjectiveReplacement(item)
	mutation.changed = true
	mutation.payload["objective_updated"] = true
	return nil
}

func applyGoalBudgetUpdate(
	item *protocol.Goal,
	request protocol.UpdateGoalRequest,
	mutation *goalUpdateMutation,
) error {
	if !request.TokenBudget.Present {
		return nil
	}
	tokenBudget, err := normalizeUpdateBudget(request.TokenBudget.Value)
	if err != nil {
		return err
	}
	if goalTokenBudgetEqual(item.TokenBudget, tokenBudget) {
		return nil
	}
	item.TokenBudget = tokenBudget
	mutation.changed = true
	mutation.payload["token_budget"] = nil
	if tokenBudget != nil {
		mutation.payload["token_budget"] = *tokenBudget
	}
	return nil
}

func (s *Service) reconcileUpdatedGoalBudget(ctx context.Context, updated *protocol.Goal) (*protocol.Goal, error) {
	status := protocol.NormalizeGoalStatus(updated.Status)
	exhausted := s.goalBudgetExhausted(*updated)
	if status == protocol.GoalStatusBudgetLimited && !exhausted {
		resumed, err := s.persistTransition(ctx, *updated, protocol.GoalStatusActive, protocol.GoalUpdateSourceUser, "resumed", "", map[string]any{
			"reason": "token budget updated",
		})
		if err != nil {
			return nil, err
		}
		s.maybeDispatchActiveGoalContinuation(ctx, *resumed)
		return resumed, nil
	}
	if status == protocol.GoalStatusActive && exhausted {
		return s.limitForSystem(ctx, *updated, protocol.GoalStatusBudgetLimited, "budget_limited", "", "Goal token budget exhausted")
	}
	s.maybeDispatchActiveGoalContinuation(ctx, *updated)
	return updated, nil
}

// Pause 暂停 active Goal。
func (s *Service) Pause(ctx context.Context, goalID string) (*protocol.Goal, error) {
	paused, err := s.changeStatus(ctx, goalID, protocol.GoalStatusPaused, protocol.GoalUpdateSourceUser, "paused", "", nil)
	if err != nil {
		return nil, err
	}
	s.interruptGoalRuntimeAfterPause(ctx, *paused)
	return paused, nil
}

// Resume 恢复 paused/blocked/usage_limited Goal；预算耗尽时需要先调整预算。
func (s *Service) Resume(ctx context.Context, goalID string) (*protocol.Goal, error) {
	s.prepareExternalMutation(ctx, strings.TrimSpace(goalID))
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	switch protocol.NormalizeGoalStatus(item.Status) {
	case protocol.GoalStatusComplete:
		return nil, ErrGoalInvalidState
	case protocol.GoalStatusBudgetLimited:
		if s.goalBudgetExhausted(*item) {
			return item, nil
		}
	}
	resumed, err := s.persistTransition(ctx, *item, protocol.GoalStatusActive, protocol.GoalUpdateSourceUser, "resumed", "", nil)
	if err != nil {
		return nil, err
	}
	s.maybeDispatchActiveGoalContinuation(ctx, *resumed)
	return resumed, nil
}

// Clear 删除当前 Goal。
func (s *Service) Clear(ctx context.Context, goalID string) (bool, error) {
	s.prepareExternalMutation(ctx, strings.TrimSpace(goalID))
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return false, err
	}
	return s.deleteGoal(ctx, *item, protocol.GoalUpdateSourceUser)
}
