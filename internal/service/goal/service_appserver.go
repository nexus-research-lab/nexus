package goal

import (
	"context"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SetFromThreadGoalParams 按 Codex app-server thread/goal/set 语义创建或更新当前 Goal。
func (s *Service) SetFromThreadGoalParams(ctx context.Context, request protocol.ThreadGoalSetParams) (*protocol.Goal, error) {
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
		return s.createFromThreadGoalParams(ctx, sessionKey, targetStatus, hasStatus, request)
	}
	s.prepareExternalMutation(ctx, current.ID)
	refreshed, err := s.repo.GetGoal(ctx, current.ID)
	if err != nil {
		return nil, err
	}
	if refreshed == nil {
		return nil, ErrGoalNotFound
	}
	return s.updateFromThreadGoalParams(ctx, *refreshed, targetStatus, hasStatus, request)
}

// ClearFromThreadGoalParams 按 Codex app-server thread/goal/clear 语义清除当前 Goal。
func (s *Service) ClearFromThreadGoalParams(ctx context.Context, request protocol.ThreadGoalClearParams) (bool, error) {
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
	if _, err := s.persistTransition(ctx, *refreshed, protocol.GoalStatusCleared, protocol.GoalUpdateSourceExternal, "cleared", "", nil); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) createFromThreadGoalParams(
	ctx context.Context,
	sessionKey string,
	targetStatus protocol.GoalStatus,
	hasStatus bool,
	request protocol.ThreadGoalSetParams,
) (*protocol.Goal, error) {
	if request.Objective == nil {
		return nil, ErrGoalNotFound
	}
	objective, err := normalizeObjective(*request.Objective)
	if err != nil {
		return nil, err
	}
	tokenBudget, err := normalizeThreadGoalBudget(request.TokenBudget)
	if err != nil {
		return nil, err
	}
	now := s.nowFn()
	item := protocol.Goal{
		ID:          s.idFactory("goal"),
		SessionKey:  sessionKey,
		Objective:   objective,
		Status:      protocol.GoalStatusActive,
		TokenBudget: tokenBudget,
		Version:     1,
		CreatedBy:   "app_server",
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata: map[string]any{
			"created_via": "thread_goal_set",
		},
	}
	created, err := s.repo.CreateGoal(ctx, item)
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *created, "created", protocol.GoalUpdateSourceUser, "", map[string]any{"objective": created.Objective}); err != nil {
		return nil, err
	}
	status := statusAfterThreadGoalBudget(*created, targetStatus, hasStatus)
	if !hasStatus || status == protocol.GoalStatusActive {
		return created, nil
	}
	return s.persistTransition(ctx, *created, status, protocol.GoalUpdateSourceExternal, threadGoalStatusEventType(status), "", nil)
}

func (s *Service) updateFromThreadGoalParams(
	ctx context.Context,
	item protocol.Goal,
	targetStatus protocol.GoalStatus,
	hasStatus bool,
	request protocol.ThreadGoalSetParams,
) (*protocol.Goal, error) {
	changed := false
	payload := map[string]any{}
	if request.Objective != nil {
		objective, err := normalizeObjective(*request.Objective)
		if err != nil {
			return nil, err
		}
		item.Objective = objective
		changed = true
		payload["objective_updated"] = true
	}
	if request.TokenBudget.Present {
		tokenBudget, err := normalizeThreadGoalBudget(request.TokenBudget)
		if err != nil {
			return nil, err
		}
		item.TokenBudget = tokenBudget
		changed = true
		if tokenBudget != nil {
			payload["token_budget"] = *tokenBudget
		} else {
			payload["token_budget"] = nil
		}
	}
	nextStatus := protocol.NormalizeGoalStatus(item.Status)
	if hasStatus {
		nextStatus = targetStatus
	}
	nextStatus = statusAfterThreadGoalBudget(item, nextStatus, hasStatus)
	if !changed && nextStatus == protocol.NormalizeGoalStatus(item.Status) {
		return &item, nil
	}
	eventType := "updated"
	if hasStatus && !changed {
		eventType = threadGoalStatusEventType(nextStatus)
	}
	return s.persistTransition(ctx, item, nextStatus, protocol.GoalUpdateSourceExternal, eventType, "", payload)
}

func validateThreadGoalSetRequest(request protocol.ThreadGoalSetParams) (string, protocol.GoalStatus, bool, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.ThreadID)
	if err != nil {
		return "", "", false, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	if request.Status == nil {
		return sessionKey, protocol.GoalStatusActive, false, nil
	}
	status, ok := protocol.GoalStatusFromThreadGoalStatus(*request.Status)
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
