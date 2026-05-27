package goal

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const goalContinuationPurpose = "goal_continuation"

//go:embed templates/continuation.md
var continuationPromptTemplate string

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
	return s.planContinuationForGoal(ctx, item, strings.TrimSpace(previousRoundID))
}

func (s *Service) planContinuationForGoal(ctx context.Context, item *protocol.Goal, previousRoundID string) (*protocol.GoalContinuation, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		plan, err := s.planContinuationForLoadedGoal(ctx, current, previousRoundID)
		if !errors.Is(err, ErrGoalVersionStale) {
			return plan, err
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

func (s *Service) planContinuationForLoadedGoal(ctx context.Context, item *protocol.Goal, previousRoundID string) (*protocol.GoalContinuation, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return nil, nil
	}
	if s.goalBudgetExhausted(*item) {
		_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusBudgetLimited, "budget_limited", previousRoundID, "Goal token budget exhausted")
		return nil, err
	}
	if item.EmptyProgressCount > 0 {
		return nil, nil
	}
	if max := s.config.GoalMaxContinuationsPerRun; max > 0 && item.ContinuationCount >= max {
		_, err := s.limitForSystem(ctx, *item, protocol.GoalStatusUsageLimited, "usage_limited", previousRoundID, "Goal auto-continuation limit reached")
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

// GoalContinuationStillCurrent 判断已生成的隐藏续跑是否仍指向当前 active Goal。
func (s *Service) GoalContinuationStillCurrent(ctx context.Context, plan protocol.GoalContinuation) (bool, error) {
	if err := s.ensureEnabled(); err != nil {
		return false, err
	}
	goalID := strings.TrimSpace(plan.Goal.ID)
	sessionKey := strings.TrimSpace(plan.Goal.SessionKey)
	if sessionKey == "" && plan.Metadata != nil {
		sessionKey = strings.TrimSpace(plan.Metadata["session_key"])
	}
	if goalID == "" && plan.Metadata != nil {
		goalID = strings.TrimSpace(plan.Metadata["goal_id"])
	}
	if goalID == "" || sessionKey == "" {
		return false, fmt.Errorf("%w: continuation plan missing goal identity", ErrGoalInvalidInput)
	}
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	item, err := s.repo.GetCurrentGoal(ctx, normalized)
	if err != nil {
		return false, err
	}
	if item == nil || protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return false, nil
	}
	return item.ID == goalID, nil
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
	objective := escapeGoalPromptText(strings.TrimSpace(item.Objective))
	tokenBudget := "none"
	if item.TokenBudget != nil {
		tokenBudget = fmt.Sprintf("%d", *item.TokenBudget)
	}
	remainingTokens := "unbounded"
	if remaining := item.RemainingTokens(); remaining != nil {
		remainingTokens = fmt.Sprintf("%d", *remaining)
	}
	return renderGoalPromptTemplate(continuationPromptTemplate, map[string]string{
		"objective":        objective,
		"tokens_used":      fmt.Sprintf("%d", item.Usage.Total()),
		"token_budget":     tokenBudget,
		"remaining_tokens": remainingTokens,
	})
}

func escapeGoalPromptText(input string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(input)
}
