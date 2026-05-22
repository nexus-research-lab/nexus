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
		_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusBudgetLimited, "budget_limited", strings.TrimSpace(previousRoundID), "Goal token budget exhausted")
		return nil, err
	}
	if max := s.config.GoalMaxContinuationsPerRun; max > 0 && item.ContinuationCount >= max {
		_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusUsageLimited, "usage_limited", strings.TrimSpace(previousRoundID), "Goal auto-continuation limit reached")
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
	return item.Usage.Total() >= *item.TokenBudget
}

func (s *Service) limitForSystem(
	ctx context.Context,
	item protocol.Goal,
	status protocol.GoalStatus,
	eventType string,
	roundID string,
	reason string,
) (*protocol.Goal, error) {
	item.LastError = strings.TrimSpace(reason)
	payload := map[string]any{
		"reason":      item.LastError,
		"usage_total": item.Usage.Total(),
	}
	if item.TokenBudget != nil {
		payload["token_budget"] = *item.TokenBudget
	}
	return s.persistTransition(ctx, item, status, protocol.GoalUpdateSourceSystem, eventType, roundID, payload)
}

func buildContinuationPrompt(item protocol.Goal, previousRoundID string) string {
	lines := []string{
		"<nexus_goal_continuation>",
		"Continue working toward the active Nexus Goal. This is an internal hidden continuation, not a new user message.",
		"Objective: " + strings.TrimSpace(item.Objective),
		fmt.Sprintf("ContinuationCount: %d", item.ContinuationCount),
		fmt.Sprintf("Usage: input=%d output=%d reasoning=%d total=%d", item.Usage.InputTokens, item.Usage.OutputTokens, item.Usage.ReasoningTokens, item.Usage.Total()),
		fmt.Sprintf("TimeUsedSeconds: %d", item.TimeUsedSeconds),
	}
	if previous := strings.TrimSpace(previousRoundID); previous != "" {
		lines = append(lines, "PreviousRoundID: "+previous)
	}
	if item.TokenBudget != nil {
		lines = append(lines, fmt.Sprintf("TokenBudget: %d", *item.TokenBudget))
		if remaining := item.RemainingTokens(); remaining != nil {
			lines = append(lines, fmt.Sprintf("RemainingTokens: %d", *remaining))
		}
	}
	lines = append(lines,
		"ContinuationRules:",
		"- Inspect the current state before acting; preserve work already done.",
		"- Keep the user-visible progress concise when you eventually respond.",
		"- If the goal is actually done, call update_goal with status=complete and include a short summary.",
		"- If the same blocker has recurred for three consecutive goal turns and you cannot make meaningful progress, call update_goal with status=blocked.",
		"- If neither condition is true, keep working and record a checkpoint after durable progress.",
		"- Do not ask for confirmation merely because the next step is non-trivial.",
		"</nexus_goal_continuation>",
	)
	return strings.Join(lines, "\n")
}
