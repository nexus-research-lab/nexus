package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const goalContinuationPurpose = "goal_continuation"

// PlanContinuationForSession 在当前 Goal 仍需推进时生成下一轮隐藏输入。
func (s *Service) PlanContinuationForSession(ctx context.Context, sessionKey string, previousRoundID string) (*protocol.GoalContinuation, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if !s.config.GoalAutoContinueEnabled {
		return nil, nil
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if item == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil, nil
	}
	if s.goalBudgetExhausted(*item) {
		_, err := s.pauseForSystem(ctx, *item, "budget_exhausted", strings.TrimSpace(previousRoundID), "Goal token budget exhausted")
		return nil, err
	}
	if max := s.config.GoalMaxContinuationsPerRun; max > 0 && item.ContinuationCount >= max {
		_, err := s.pauseForSystem(ctx, *item, "continuation_limit_reached", strings.TrimSpace(previousRoundID), "Goal auto-continuation limit reached")
		return nil, err
	}

	roundID := s.idFactory("goal_continuation")
	expectedVersion := item.Version
	now := s.nowFn()
	item.ContinuationCount++
	item.Version++
	item.UpdatedAt = now
	item.LastError = ""
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"continuation_count": updated.ContinuationCount}
	if previous := strings.TrimSpace(previousRoundID); previous != "" {
		payload["previous_round_id"] = previous
	}
	if err := s.appendEvent(ctx, *updated, "continuation_scheduled", protocol.GoalUpdateSourceSystem, roundID, payload); err != nil {
		return nil, err
	}
	return &protocol.GoalContinuation{
		Goal:           *updated,
		RoundID:        roundID,
		Prompt:         buildContinuationPrompt(*updated, previousRoundID),
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        goalContinuationPurpose,
		Metadata: map[string]string{
			"goal_id":           updated.ID,
			"session_key":       updated.SessionKey,
			"previous_round_id": strings.TrimSpace(previousRoundID),
		},
	}, nil
}

func (s *Service) goalBudgetExhausted(item protocol.Goal) bool {
	if item.TokenBudget == nil || *item.TokenBudget <= 0 {
		return false
	}
	total := item.Usage.TotalTokens
	if total == 0 {
		total = item.Usage.InputTokens + item.Usage.OutputTokens + item.Usage.CacheCreationInputTokens + item.Usage.CacheReadInputTokens + item.Usage.ReasoningTokens
	}
	return total >= *item.TokenBudget
}

func (s *Service) pauseForSystem(ctx context.Context, item protocol.Goal, eventType string, roundID string, reason string) (*protocol.Goal, error) {
	item.LastError = strings.TrimSpace(reason)
	payload := map[string]any{"reason": item.LastError}
	return s.persistTransition(ctx, item, protocol.GoalStatusPaused, protocol.GoalUpdateSourceSystem, eventType, roundID, payload)
}

func buildContinuationPrompt(item protocol.Goal, previousRoundID string) string {
	lines := []string{
		"<nexus_goal_continuation>",
		"这是 Nexus 系统为当前 Goal 触发的隐藏续跑输入，不是用户的新消息。",
		"继续推进当前 Goal；如果已完成，调用 Goal 工具标记完成；如果没有用户输入或外部状态就无法继续，调用 Goal 工具标记阻塞。",
		"Objective: " + strings.TrimSpace(item.Objective),
		fmt.Sprintf("ContinuationCount: %d", item.ContinuationCount),
		fmt.Sprintf("Usage: input=%d output=%d total=%d", item.Usage.InputTokens, item.Usage.OutputTokens, item.Usage.TotalTokens),
	}
	if previous := strings.TrimSpace(previousRoundID); previous != "" {
		lines = append(lines, "PreviousRoundID: "+previous)
	}
	if item.TokenBudget != nil {
		lines = append(lines, fmt.Sprintf("TokenBudget: %d", *item.TokenBudget))
	}
	lines = append(lines, "</nexus_goal_continuation>")
	return strings.Join(lines, "\n")
}
