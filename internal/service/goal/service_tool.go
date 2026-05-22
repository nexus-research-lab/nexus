package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CompleteByModel 允许模型工具把 active Goal 标记为完成。
func (s *Service) CompleteByModel(ctx context.Context, goalID string, request protocol.CompleteGoalRequest) (*protocol.Goal, error) {
	payload := map[string]any{}
	if summary := strings.TrimSpace(request.Summary); summary != "" {
		payload["summary"] = summary
	}
	return s.changeStatus(ctx, goalID, protocol.GoalStatusComplete, protocol.GoalUpdateSourceModel, "completed", request.RoundID, payload)
}

// BlockByModel 允许模型工具把 active Goal 标记为阻塞。
func (s *Service) BlockByModel(ctx context.Context, goalID string, request protocol.BlockGoalRequest) (*protocol.Goal, error) {
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		return nil, ErrGoalInvalidInput
	}
	payload := map[string]any{"reason": reason}
	if neededInput := strings.TrimSpace(request.NeededInput); neededInput != "" {
		payload["needed_input"] = neededInput
	}
	return s.changeStatus(ctx, goalID, protocol.GoalStatusBlocked, protocol.GoalUpdateSourceModel, "blocked", request.RoundID, payload)
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
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens + usage.ReasoningTokens
	}
	if usage.TotalTokens == 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.Usage = item.Usage.Add(usage)
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
	return updated, nil
}
