package goal

import (
	"context"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

// SetFromThreadGoalParams 按 Codex app-server thread/goal/set 语义创建或更新当前 Goal。
func (s *Service) SetFromThreadGoalParams(ctx context.Context, request goalappserver.ThreadGoalSetParams) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	sessionKey, targetStatus, hasStatus, err := validateThreadGoalSetRequest(request)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetCurrentGoal(ctx, sessionKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		created, err := s.createFromThreadGoalParams(ctx, sessionKey, targetStatus, hasStatus, request)
		if err != nil {
			return nil, err
		}
		s.activateExternalGoalAccounting(ctx, *created)
		s.maybeDispatchActiveGoalContinuation(ctx, *created)
		return created, nil
	}
	s.prepareExternalMutation(ctx, current.ID)
	refreshed, err := s.repo.GetGoal(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if refreshed == nil {
		return nil, ErrGoalNotFound
	}
	updated, err := s.updateFromThreadGoalParams(ctx, *refreshed, targetStatus, hasStatus, request)
	if err != nil {
		return nil, err
	}
	s.maybeDispatchActiveGoalContinuation(ctx, *updated)
	return updated, nil
}

// ClearFromThreadGoalParams 按 Codex app-server thread/goal/clear 语义清除当前 Goal。
func (s *Service) ClearFromThreadGoalParams(ctx context.Context, request goalappserver.ThreadGoalClearParams) (bool, error) {
	if err := s.ensureEnabled(); err != nil {
		return false, err
	}
	sessionKey, err := protocol.RequireStructuredSessionKey(request.ThreadID)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	current, err := s.repo.GetCurrentGoal(ctx, sessionKey)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, nil
	}
	s.prepareExternalMutation(ctx, current.ID)
	refreshed, err := s.repo.GetGoal(ctx, current.ID)
	if err != nil {
		return false, err
	}
	if refreshed == nil {
		return false, nil
	}
	deleted, err := s.deleteGoal(ctx, *refreshed, protocol.GoalUpdateSourceExternal)
	if err != nil {
		return false, err
	}
	return deleted, nil
}

func (s *Service) createFromThreadGoalParams(
	ctx context.Context,
	sessionKey string,
	targetStatus protocol.GoalStatus,
	hasStatus bool,
	request goalappserver.ThreadGoalSetParams,
) (*protocol.Goal, error) {
	if request.Objective == nil {
		return nil, newGoalNotFoundError(fmt.Sprintf(
			"cannot update goal for thread %s: no goal exists",
			sessionKey,
		))
	}
	objective, err := normalizeObjective(*request.Objective)
	if err != nil {
		return nil, err
	}
	objective, metadata := s.rewriteCreateObjective(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  objective,
		CreatedBy:  "app_server",
		Metadata: map[string]any{
			"created_via": "thread_goal_set",
		},
	}, objective)
	tokenBudget, err := normalizeThreadGoalBudget(request.TokenBudget)
	if err != nil {
		return nil, err
	}
	now := s.nowFn()
	status := statusAfterThreadGoalBudget(protocol.Goal{
		Status:      protocol.GoalStatusActive,
		TokenBudget: tokenBudget,
	}, targetStatus, hasStatus)
	item := protocol.Goal{
		ID:          s.idFactory("goal"),
		SessionKey:  sessionKey,
		Objective:   objective,
		Status:      status,
		TokenBudget: tokenBudget,
		Version:     1,
		CreatedBy:   "app_server",
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    metadata,
	}
	applyInitialGoalStatusTime(&item, now)
	created, err := s.repo.CreateGoal(ctx, item)
	if err != nil {
		return nil, err
	}
	s.fillEmptyPreviewFromGoal(ctx, *created)
	if err := s.appendEvent(ctx, *created, "created", protocol.GoalUpdateSourceExternal, "", map[string]any{"objective": created.Objective}); err != nil {
		return nil, err
	}
	return created, nil
}

func applyInitialGoalStatusTime(item *protocol.Goal, now time.Time) {
	switch protocol.NormalizeGoalStatus(item.Status) {
	case protocol.GoalStatusComplete:
		item.CompletedAt = &now
	case protocol.GoalStatusBlocked:
		item.BlockedAt = &now
	}
}

func (s *Service) updateFromThreadGoalParams(
	ctx context.Context,
	item protocol.Goal,
	targetStatus protocol.GoalStatus,
	hasStatus bool,
	request goalappserver.ThreadGoalSetParams,
) (*protocol.Goal, error) {
	hasUpdateFields := false
	valueChanged := false
	payload := map[string]any{}
	if request.Objective != nil {
		hasUpdateFields = true
		objective, err := normalizeObjective(*request.Objective)
		if err != nil {
			return nil, err
		}
		objective, payload = s.rewriteUpdateObjective(ctx, protocol.UpdateGoalRequest{Objective: &objective}, item.SessionKey, objective, payload)
		if item.Objective != objective {
			item.Objective = objective
			valueChanged = true
			payload["objective_updated"] = true
		}
	}
	if request.TokenBudget.Present {
		hasUpdateFields = true
		tokenBudget, err := normalizeThreadGoalBudget(request.TokenBudget)
		if err != nil {
			return nil, err
		}
		if !goalTokenBudgetEqual(item.TokenBudget, tokenBudget) {
			item.TokenBudget = tokenBudget
			valueChanged = true
			if tokenBudget != nil {
				payload["token_budget"] = *tokenBudget
			} else {
				payload["token_budget"] = nil
			}
		}
	}
	currentStatus := protocol.NormalizeGoalStatus(item.Status)
	nextStatus := currentStatus
	if hasStatus {
		hasUpdateFields = true
		nextStatus = targetStatus
	}
	nextStatus = statusAfterThreadGoalBudget(item, nextStatus, hasStatus)
	if !hasUpdateFields {
		return &item, nil
	}
	eventType := "updated"
	if hasStatus && !valueChanged && nextStatus != currentStatus {
		eventType = threadGoalStatusEventType(nextStatus)
	}
	updated, err := s.persistThreadGoalSetTransition(ctx, item, nextStatus, eventType, payload)
	if err != nil {
		return nil, err
	}
	if request.Objective != nil {
		s.fillEmptyPreviewFromGoal(ctx, *updated)
	}
	return updated, nil
}

func (s *Service) persistThreadGoalSetTransition(
	ctx context.Context,
	item protocol.Goal,
	status protocol.GoalStatus,
	eventType string,
	payload map[string]any,
) (*protocol.Goal, error) {
	return s.persistTransitionWithOptions(
		ctx,
		item,
		status,
		protocol.GoalUpdateSourceExternal,
		eventType,
		"",
		payload,
		transitionOptions{persistBudgetLimitedStopRequest: true},
	)
}

func validateThreadGoalSetRequest(request goalappserver.ThreadGoalSetParams) (string, protocol.GoalStatus, bool, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.ThreadID)
	if err != nil {
		return "", "", false, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	if request.Status == nil {
		return sessionKey, protocol.GoalStatusActive, false, nil
	}
	status, ok := goalappserver.GoalStatusFromThreadGoalStatus(*request.Status)
	if !ok {
		return "", "", false, ErrGoalInvalidInput
	}
	return sessionKey, status, true, nil
}

func normalizeThreadGoalBudget(input protocol.OptionalInt64) (*int64, error) {
	if !input.Present {
		return nil, nil
	}
	return normalizeUpdateBudget(input.Value)
}

func statusAfterThreadGoalBudget(item protocol.Goal, status protocol.GoalStatus, explicitStatus bool) protocol.GoalStatus {
	status = protocol.NormalizeGoalStatus(status)
	currentStatus := protocol.NormalizeGoalStatus(item.Status)
	if currentStatus == protocol.GoalStatusBudgetLimited &&
		(status == protocol.GoalStatusPaused || status == protocol.GoalStatusBlocked) {
		return protocol.GoalStatusBudgetLimited
	}
	if status == protocol.GoalStatusActive && item.TokenBudget != nil && item.Usage.Total() >= *item.TokenBudget {
		return protocol.GoalStatusBudgetLimited
	}
	if !explicitStatus && currentStatus == protocol.GoalStatusActive && item.TokenBudget != nil && item.Usage.Total() >= *item.TokenBudget {
		return protocol.GoalStatusBudgetLimited
	}
	return status
}

func threadGoalStatusEventType(status protocol.GoalStatus) string {
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusPaused:
		return "paused"
	case protocol.GoalStatusComplete:
		return "completed"
	case protocol.GoalStatusBlocked:
		return "blocked"
	case protocol.GoalStatusBudgetLimited:
		return "budget_limited"
	case protocol.GoalStatusUsageLimited:
		return "usage_limited"
	default:
		return "resumed"
	}
}
