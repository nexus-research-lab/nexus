// INPUT: 模型 complete/blocked 请求、Room lead 身份、usage 与 objective revision。
// OUTPUT: 受负责人权限、状态机和 revision fence 保护的 Goal 工具结果。
// POS: Goal 模型生命周期工具的服务层入口。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CompleteByModel 允许模型工具把 active Goal 标记为完成。
func (s *Service) CompleteByModel(ctx context.Context, goalID string, request protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	payload := map[string]any{}
	if summary := strings.TrimSpace(request.Summary); summary != "" {
		payload["summary"] = summary
	}
	if agentID := strings.TrimSpace(request.AgentID); agentID != "" {
		payload["source_agent_id"] = agentID
	}
	return s.changeStatusByModel(ctx, goalID, request.AgentID, protocol.GoalStatusComplete, "completed", request.RoundID, payload, request.ExpectedObjectiveRevision, true)
}

// BlockByModel 允许模型工具把 active Goal 标记为阻塞。
func (s *Service) BlockByModel(ctx context.Context, goalID string, request protocol.BlockGoalRequest) (*protocol.Goal, error) {
	reason := strings.TrimSpace(request.Reason)
	payload := map[string]any{}
	if reason != "" {
		payload["reason"] = reason
	}
	if neededInput := strings.TrimSpace(request.NeededInput); neededInput != "" {
		payload["needed_input"] = neededInput
	}
	if agentID := strings.TrimSpace(request.AgentID); agentID != "" {
		payload["source_agent_id"] = agentID
	}
	return s.changeStatusByModel(ctx, goalID, request.AgentID, protocol.GoalStatusBlocked, "blocked", request.RoundID, payload, request.ExpectedObjectiveRevision, false)
}

func (s *Service) changeStatusByModel(
	ctx context.Context,
	goalID string,
	agentID string,
	status protocol.GoalStatus,
	eventType string,
	roundID string,
	payload map[string]any,
	expectedRevision int64,
	requireRoomCollaboration bool,
) (*protocol.Goal, error) {
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if err = authorizeRoomGoalModelMutation(*item, agentID); err != nil {
		return nil, err
	}
	ctx = withBudgetLimitSteeringSuppressed(ctx)
	s.prepareExternalMutation(ctx, strings.TrimSpace(goalID))
	item, err = s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if !protocol.IsCurrentGoalStatus(current.Status) {
			return nil, ErrGoalInvalidState
		}
		if authErr := authorizeRoomGoalModelMutation(*current, agentID); authErr != nil {
			return nil, authErr
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		if requireRoomCollaboration && roomGoalCompletionRequiresCollaboration(*current) {
			return nil, fmt.Errorf("%w: multi-member Room Goal requires a room-visible non-lead collaboration reply before completion", ErrGoalInvalidState)
		}
		return s.persistTransition(ctx, *current, status, protocol.GoalUpdateSourceModel, eventType, roundID, payload)
	})
}

// Events 返回 Goal 审计事件。
func (s *Service) Events(ctx context.Context, goalID string, limit int) ([]protocol.GoalEvent, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.repo.ListEvents(ctx, item.ID, limit)
}

// RecordUsageForSession 把一轮 runtime usage 计入 session 当前 Goal。
func (s *Service) RecordUsageForSession(ctx context.Context, sessionKey string, usage protocol.GoalUsage, roundID string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return nil, ErrGoalInvalidInput
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordUsageForGoal(ctx, item, usage, roundID)
}

// RecordUsageForGoal 把一轮 runtime usage 计入指定 Goal，即使它已在本轮被模型标记为 complete/blocked。
func (s *Service) RecordUsageForGoal(ctx context.Context, goalID string, usage protocol.GoalUsage, roundID string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordUsageForGoal(ctx, item, usage, roundID)
}

// UsageLimitForSession 把 session 当前 Goal 标记为 runtime usage_limited。
func (s *Service) UsageLimitForSession(ctx context.Context, sessionKey string, roundID string, reason string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return nil, ErrGoalInvalidInput
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.limitForSystem(ctx, *item, protocol.GoalStatusUsageLimited, "usage_limited", roundID, firstNonEmptyGoalReason(reason, "Runtime usage limit reached"))
}

func (s *Service) recordUsageForGoal(ctx context.Context, item *protocol.Goal, usage protocol.GoalUsage, roundID string) (*protocol.Goal, error) {
	usage.TotalTokens = usage.BudgetTokens()
	if usage.TotalTokens == 0 && usage.RuntimeSeconds == 0 {
		return item, nil
	}
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := s.recordUsageForLoadedGoal(ctx, current, usage, roundID)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordUsageForLoadedGoal(ctx context.Context, item *protocol.Goal, usage protocol.GoalUsage, roundID string) (*protocol.Goal, error) {
	expectedVersion := item.Version
	item.Usage = item.Usage.Add(usage)
	item.TimeUsedSeconds += usage.RuntimeSeconds
	budgetLimited := protocol.NormalizeGoalStatus(item.Status) == protocol.GoalStatusActive && s.goalBudgetExhausted(*item)
	if budgetLimited {
		item.Status = protocol.GoalStatusBudgetLimited
		item.LastError = "Goal token budget exhausted"
	}
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *updated, "usage_recorded", protocol.GoalUpdateSourceSystem, roundID, map[string]any{"usage": usage}); err != nil {
		return nil, err
	}
	if budgetLimited {
		payload := map[string]any{
			"reason":      item.LastError,
			"usage_total": item.Usage.Total(),
		}
		if item.TokenBudget != nil {
			payload["token_budget"] = *item.TokenBudget
		}
		if err := s.appendEvent(ctx, *updated, "budget_limited", protocol.GoalUpdateSourceSystem, roundID, payload); err != nil {
			return nil, err
		}
	}
	if protocol.NormalizeGoalStatus(updated.Status) == protocol.GoalStatusActive {
		s.recordWallClockGoalUsage(*updated, usage.RuntimeSeconds)
	} else {
		s.clearWallClockGoal(*updated)
	}
	return updated, nil
}

func firstNonEmptyGoalReason(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
