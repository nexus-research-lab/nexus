// INPUT: Goal 创建/读取、model round durable usage scope、Room creator/lead 身份、用户更新请求与 Execution/Room 终态 readiness。
// OUTPUT: owner 授权先于 runtime accounting 的原子 Goal/created 事件/usage scope、统一清理外部 metadata、future Execution reserved phase、creator/lead 审计身份与受 WorkGraph/运行中工作保护的后续 runtime 决策。
// POS: Goal 应用服务主入口。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
	config              config.Config
	repo                Repository
	events              eventBroadcaster
	guidance            guidanceDispatcher
	preview             previewFiller
	rewriter            objectiveRewriter
	objectiveRetarget   ObjectiveRetargetCoordinator
	externalMutation    externalMutationAccountant
	runtimeInterrupt    runtimeInterrupter
	executionCompletion executionGoalCompletionReadiness
	sessionOwnership    GoalSessionOwnershipVerifier
	roomCompletion      roomGoalCompletionReadiness
	continuations       ContinuationDispatcher
	wallClock           *goalWallClockAccounting
	nowFn               func() time.Time
	idFactory           func(string) string
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
	runtimeAgentID := strings.TrimSpace(request.AgentID)
	ownershipAgentID := runtimeAgentID
	if protocol.IsRoomSharedSessionKey(sessionKey) && ownershipAgentID == "" {
		ownershipAgentID = strings.TrimSpace(request.RoomLeadAgentID)
	}
	ownerUserID, verifiedAgentID, verifiedAgentName, roomCollaborationRequired, err := s.verifyGoalSessionOwnership(
		ctx,
		sessionKey,
		request.OwnerUserID,
		ownershipAgentID,
	)
	if err != nil {
		return nil, err
	}
	if protocol.IsRoomSharedSessionKey(sessionKey) {
		if request.RoomCollaborationRequired == nil {
			_, explicitlySet := request.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequired]
			if !explicitlySet {
				request.RoomCollaborationRequired = &roomCollaborationRequired
			}
		} else if !*request.RoomCollaborationRequired && roomCollaborationRequired {
			return nil, fmt.Errorf(
				"%w: Room Goal collaboration requirement conflicts with the verified member directory",
				ErrGoalInvalidState,
			)
		}
	}
	request.OwnerUserID = ownerUserID
	request.AgentID = runtimeAgentID
	if strings.TrimSpace(request.CreatedBy) == "user" {
		request.Metadata = sanitizeExternalGoalMetadata(request.Metadata)
	}
	if protocol.IsRoomSharedSessionKey(sessionKey) && request.RoomCollaborationRequired != nil {
		if request.Metadata == nil {
			request.Metadata = map[string]any{}
		}
		request.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequired] =
			*request.RoomCollaborationRequired
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
			Objective:                 &objective,
			TokenBudget:               protocol.OptionalInt64{Present: true, Value: request.TokenBudget},
			OwnerUserID:               request.OwnerUserID,
			Metadata:                  metadata,
			RoomLeadAgentID:           verifiedRoomLeadAgentID(request.AgentID, verifiedAgentID),
			RoomLeadAgentName:         verifiedAgentName,
			RoomCollaborationRequired: request.RoomCollaborationRequired,
			RoomCollaborationRoundID:  strings.TrimSpace(request.RoundID),
		})
	}
	scopeRoundID := ""
	if strings.TrimSpace(request.CreatedBy) == "model" {
		scopeRoundID = strings.TrimSpace(request.RoundID)
	}
	if err := s.preflightGoalCreate(sessionKey, scopeRoundID); err != nil {
		return nil, err
	}
	objective, metadata := s.rewriteCreateObjective(ctx, request, objective)
	if metadata != nil {
		metadata = cloneMap(metadata)
		delete(metadata, protocol.GoalMetadataObjectiveRevision)
		delete(metadata, protocol.GoalMetadataObjectiveAlignment)
	}
	metadata = initializeRoomGoalOwnershipMetadata(
		sessionKey,
		metadata,
		request.AgentID,
		verifiedRoomAgentName(request.AgentID, verifiedAgentID, verifiedAgentName),
		verifiedRoomLeadAgentID(request.AgentID, verifiedAgentID),
		verifiedAgentName,
	)
	ownerUserID = strings.TrimSpace(request.OwnerUserID)
	if ownerUserID == "" {
		ownerUserID = strings.TrimSpace(authctx.OwnerUserID(ctx))
	}
	if ownerUserID != "" {
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata[protocol.GoalMetadataOwnerUserID] = ownerUserID
	}

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
	createdEvent := s.newGoalEvent(
		item,
		"created",
		createGoalEventSource(item.CreatedBy),
		request.RoundID,
		map[string]any{"objective": item.Objective},
		now,
	)
	created, usageEvent, err := s.createGoalWithUsageScope(ctx, request, item, createdEvent, now)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(created.CreatedBy) == "model" {
		s.markWallClockGoalActive(*created)
	} else {
		if err := s.activateExternalGoalAccounting(ctx, *created); err != nil {
			return nil, s.rollbackFailedGoalCreate(ctx, *created, err)
		}
	}
	s.updatePreviewFromGoal(ctx, *created, request.OwnerUserID)
	s.publishGoalEvent(ctx, *created, createdEvent)
	if usageEvent != nil {
		s.publishGoalEvent(ctx, *created, *usageEvent)
	}
	s.maybeDispatchActiveGoalContinuation(ctx, *created)
	return created, nil
}

// sanitizeExternalGoalMetadata 在 Goal 服务信任边界统一移除所有只能由
// owner/session、Execution binding、objective transition 或 Room runtime 写入的键。
// Transport handler 不能成为这组不变量的唯一守门人。
func sanitizeExternalGoalMetadata(metadata map[string]any) map[string]any {
	metadata = cloneMap(metadata)
	for _, key := range []string{
		protocol.GoalMetadataOwnerUserID,
		protocol.GoalMetadataSourceObjective,
		protocol.GoalMetadataObjectiveNormalized,
		protocol.GoalMetadataExecutionID,
		protocol.GoalMetadataExecutionBindingState,
		protocol.GoalMetadataPromotionCommand,
		protocol.GoalMetadataActivationOrigin,
		protocol.GoalMetadataActivationReason,
		protocol.GoalMetadataCompletionCriteria,
		protocol.GoalMetadataObjectiveAlignment,
		protocol.GoalMetadataExplicitCommand,
		protocol.GoalMetadataObjectiveTransition,
		protocol.GoalMetadataObjectiveRevision,
		protocol.GoalMetadataRoomGoalScope,
		protocol.GoalMetadataRoomGoalCreatorAgentID,
		protocol.GoalMetadataRoomGoalLeadAgentID,
		protocol.GoalMetadataRoomGoalLeadAgentName,
		protocol.GoalMetadataRoomGoalCollaborationRequired,
		protocol.GoalMetadataRoomGoalCollaborationObserved,
		protocol.GoalMetadataRoomGoalCollaborationAgentID,
		protocol.GoalMetadataRoomGoalCollaborationRoundID,
		protocol.GoalMetadataRoomGoalCollaborationObservedAt,
		protocol.GoalMetadataRoomGoalCollaborationRequirementRound,
	} {
		delete(metadata, key)
	}
	return metadata
}

// reserveExternalGoalExecution 为外部 Goal 预留稳定 Execution。
func reserveExternalGoalExecution(metadata map[string]any, goalID string) map[string]any {
	metadata = cloneMap(metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	commandID := "external_goal_" + strings.TrimSpace(goalID)
	metadata[protocol.GoalMetadataExplicitCommand] = commandID
	metadata[protocol.GoalMetadataExecutionID] = protocol.ExplicitGoalReservedExecutionID(commandID)
	metadata[protocol.GoalMetadataExecutionBindingState] =
		string(protocol.GoalExecutionBindingStateReserved)
	metadata[protocol.GoalMetadataActivationOrigin] = string(protocol.GoalActivationOriginUserExplicit)
	metadata[protocol.GoalMetadataActivationReason] = string(protocol.GoalActivationReasonPersistenceRequested)
	return metadata
}

// ensureExternalGoalExecutionReservation 为历史外部 Goal 补齐缺失的稳定 reservation。
func (s *Service) ensureExternalGoalExecutionReservation(
	ctx context.Context,
	item *protocol.Goal,
) (*protocol.Goal, error) {
	if item == nil || strings.TrimSpace(item.CreatedBy) == "model" ||
		protocol.GoalReservedExecutionID(*item) != "" {
		return item, nil
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if strings.TrimSpace(current.CreatedBy) == "model" ||
			protocol.GoalReservedExecutionID(*current) != "" {
			return current, nil
		}
		expectedVersion := current.Version
		current.Metadata = reserveExternalGoalExecution(current.Metadata, current.ID)
		current.Version++
		current.UpdatedAt = s.nowFn()
		updated, err := s.repo.UpdateGoal(ctx, *current, expectedVersion)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrGoalVersionStale
		}
		if err != nil {
			return nil, err
		}
		return updated, nil
	})
}

func (s *Service) createGoalWithUsageScope(
	ctx context.Context,
	request protocol.CreateGoalRequest,
	item protocol.Goal,
	createdEvent protocol.GoalEvent,
	now time.Time,
) (*protocol.Goal, *protocol.GoalEvent, error) {
	roundID := strings.TrimSpace(request.RoundID)
	repository, scoped := s.repo.(usageScopeCreateRepository)
	if strings.TrimSpace(request.CreatedBy) == "model" && roundID != "" && scoped {
		ownerUserID := strings.TrimSpace(request.OwnerUserID)
		if ownerUserID == "" {
			ownerUserID = strings.TrimSpace(authctx.OwnerUserID(ctx))
		}
		result, err := repository.CreateGoalWithUsageScope(ctx, item, createdEvent, protocol.GoalUsageScopeBinding{
			OwnerUserID:    ownerUserID,
			GoalSessionKey: item.SessionKey,
			SourceKind:     protocol.GoalUsageSourceKindNXSTask,
			ScopeRoundID:   roundID,
			GoalID:         item.ID,
			BoundAt:        now,
			UsageEventID:   s.idFactory("goal_event"),
		})
		if err != nil {
			return nil, nil, s.classifyGoalCreateError(ctx, item, err)
		}
		if result.Goal == nil {
			return nil, nil, fmt.Errorf("%w: scoped Goal creation returned no Goal", ErrGoalInvalidState)
		}
		return result.Goal, result.UsageEvent, nil
	}

	created, err := s.repo.CreateGoalWithEvent(ctx, item, createdEvent)
	if err != nil {
		return nil, nil, s.classifyGoalCreateError(ctx, item, err)
	}
	return created, nil, nil
}

// classifyGoalCreateError turns the storage-level unique race into the stable
// domain conflict returned by the preflight path. Other transaction failures
// retain their original cause.
func (s *Service) classifyGoalCreateError(
	ctx context.Context,
	attempted protocol.Goal,
	createErr error,
) error {
	current, readErr := s.repo.GetCurrentGoal(ctx, strings.TrimSpace(attempted.SessionKey))
	if readErr == nil && current != nil && strings.TrimSpace(current.ID) != strings.TrimSpace(attempted.ID) {
		return ErrGoalConflict
	}
	return createErr
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

// CurrentOptionalForOwner returns the current Goal only when its durable owner
// provenance exactly matches the authenticated caller. Ownerless legacy rows
// are claimed once only after session and Execution provenance are proven.
func (s *Service) CurrentOptionalForOwner(
	ctx context.Context,
	sessionKey string,
	ownerUserID string,
) (*protocol.Goal, error) {
	item, err := s.CurrentOptional(ctx, sessionKey)
	if err != nil || item == nil {
		return item, err
	}
	return s.authorizeOwnerScopedGoal(ctx, item, ownerUserID)
}

// Update 更新当前 Goal 文本、预算或 metadata。
func (s *Service) Update(ctx context.Context, goalID string, request protocol.UpdateGoalRequest) (*protocol.Goal, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	item, err = s.authorizeGoalMutation(ctx, item, request.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if err = s.prepareExternalMutation(ctx, strings.TrimSpace(goalID)); err != nil {
		return nil, err
	}
	item, err = s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	item, err = s.authorizeGoalMutation(ctx, item, request.OwnerUserID)
	if err != nil {
		return nil, err
	}
	managedBinding := false
	if request.Objective != nil {
		managedBinding, err = s.goalHasManagedExecutionBinding(ctx, *item)
		if err != nil {
			return nil, err
		}
	}
	if request.Objective != nil && managedBinding {
		requestedObjective, objectiveErr := normalizeObjective(*request.Objective)
		if objectiveErr != nil {
			return nil, objectiveErr
		}
		if objectiveRetargetRequestAlreadyApplied(*item, requestedObjective) {
			request.Objective = nil
			if !request.TokenBudget.Present && request.Metadata == nil {
				return item, nil
			}
		} else {
			objective, _ := s.rewriteUpdateObjective(
				ctx,
				request,
				item.SessionKey,
				requestedObjective,
				nil,
			)
			if item.Objective != objective && s.objectiveRetarget != nil {
				retargeted, retargetErr := s.objectiveRetarget.RetargetGoalObjective(ctx, ObjectiveRetargetCommand{
					Goal:                      *item,
					RequestedObjective:        requestedObjective,
					Objective:                 objective,
					Reason:                    "user updated the Goal objective",
					ExpectedObjectiveRevision: item.ObjectiveRevision(),
					Source:                    protocol.GoalUpdateSourceUser,
					OwnerUserID:               strings.TrimSpace(request.OwnerUserID),
				})
				if retargetErr != nil {
					return nil, retargetErr
				}
				request.Objective = nil
				if !request.TokenBudget.Present && request.Metadata == nil {
					s.updatePreviewFromGoal(ctx, *retargeted, request.OwnerUserID)
					s.maybeDispatchActiveGoalContinuation(ctx, *retargeted)
					return retargeted, nil
				}
				updated, updateErr := s.Update(ctx, retargeted.ID, request)
				if updateErr != nil {
					return nil, updateErr
				}
				s.updatePreviewFromGoal(ctx, *updated, request.OwnerUserID)
				return updated, nil
			}
			if item.Objective != objective {
				return nil, fmt.Errorf(
					"%w: Goal objective retarget coordinator is unavailable for a managed Execution",
					ErrGoalInvalidState,
				)
			}
		}
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
		item.Metadata = preserveServerOwnedGoalMetadata(
			*item,
			preserveRoomGoalOwnershipMetadata(*item, request.Metadata),
		)
		delete(item.Metadata, protocol.GoalMetadataObjectiveRevision)
		if objectiveRevision > 1 {
			item.Metadata[protocol.GoalMetadataObjectiveRevision] = objectiveRevision
		}
		mutation.changed = true
		mutation.payload["metadata_updated"] = true
	}
	applyServerRoomGoalUpdate(item, request, &mutation)
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
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	item, err = s.authorizeGoalMutation(ctx, item, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	paused, err := s.changeStatus(ctx, goalID, protocol.GoalStatusPaused, protocol.GoalUpdateSourceUser, "paused", "", nil)
	if err != nil {
		return nil, err
	}
	s.interruptGoalRuntimeAfterPause(ctx, *paused)
	return paused, nil
}

// Resume 恢复 paused/blocked/usage_limited Goal；预算耗尽时需要先调整预算。
func (s *Service) Resume(ctx context.Context, goalID string) (*protocol.Goal, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	item, err = s.authorizeGoalMutation(ctx, item, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	if err = s.prepareExternalMutation(ctx, strings.TrimSpace(goalID)); err != nil {
		return nil, err
	}
	item, err = s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	item, err = s.authorizeGoalMutation(ctx, item, authctx.OwnerUserID(ctx))
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
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return false, err
	}
	item, err = s.authorizeGoalMutation(ctx, item, authctx.OwnerUserID(ctx))
	if err != nil {
		return false, err
	}
	if err = s.ensureGoalClearAllowed(ctx, *item); err != nil {
		return false, err
	}
	if err := s.prepareExternalMutationAtSettlementBoundary(ctx, strings.TrimSpace(goalID)); err != nil {
		return false, err
	}
	item, err = s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return false, err
	}
	item, err = s.authorizeGoalMutation(ctx, item, authctx.OwnerUserID(ctx))
	if err != nil {
		return false, err
	}
	return s.clearGoal(ctx, *item, protocol.GoalUpdateSourceUser)
}
