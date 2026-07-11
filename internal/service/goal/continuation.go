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

// ContinuationPlanProvider 提供续跑候选的规划与并发有效性校验。
type ContinuationPlanProvider interface {
	PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error)
	GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error)
}

type continuationPlanReleaser interface {
	ReleaseContinuationPlan(context.Context, protocol.GoalContinuation, string) (*protocol.Goal, error)
}

// PrepareContinuationForDispatch 统一续跑候选在进入运行时前的状态转换。
// shouldDefer 必须读取最新队列状态，因为规划过程可能与显式输入并发。
func PrepareContinuationForDispatch(
	ctx context.Context,
	provider ContinuationPlanProvider,
	sessionKey string,
	previousRoundID string,
	shouldDefer func(protocol.GoalContinuation) bool,
) (*protocol.GoalContinuation, error) {
	if provider == nil {
		return nil, nil
	}
	plan, err := provider.PlanContinuationForSession(ctx, sessionKey, previousRoundID)
	if err != nil || plan == nil {
		return plan, err
	}
	return ValidateContinuationForDispatch(ctx, provider, *plan, shouldDefer)
}

// ValidateContinuationForDispatch 校验已经规划出的候选是否仍可进入运行时。
// 调用方可在规划与校验之间插入自身的目标存在性检查。
func ValidateContinuationForDispatch(
	ctx context.Context,
	provider ContinuationPlanProvider,
	plan protocol.GoalContinuation,
	shouldDefer func(protocol.GoalContinuation) bool,
) (*protocol.GoalContinuation, error) {
	if shouldDefer != nil && shouldDefer(plan) {
		releaseContinuationPlan(ctx, provider, plan, "Goal continuation deferred before dispatch")
		return nil, nil
	}
	current, err := provider.GoalContinuationStillCurrent(ctx, plan)
	if err != nil {
		return nil, err
	}
	if !current {
		releaseContinuationPlan(ctx, provider, plan, "Goal continuation stale before dispatch")
		return nil, nil
	}
	return &plan, nil
}

func releaseContinuationPlan(ctx context.Context, provider ContinuationPlanProvider, plan protocol.GoalContinuation, reason string) {
	if releaser, ok := provider.(continuationPlanReleaser); ok {
		_, _ = releaser.ReleaseContinuationPlan(ctx, plan, reason)
	}
}

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
		if goalCompletionToolRetryCount(item.Metadata) >= goalCompletionToolMaxRetries {
			_, err := s.completeAfterCompletionToolMissRetry(ctx, item, previousRoundID, "Goal completion finalization retry already exhausted")
			return nil, err
		}
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

// ReleaseContinuationPlan 撤销尚未启动的隐藏续跑计划，避免未执行的 candidate 消耗续跑次数。
func (s *Service) ReleaseContinuationPlan(ctx context.Context, plan protocol.GoalContinuation, reason string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	goalID := strings.TrimSpace(plan.Goal.ID)
	if goalID == "" && plan.Metadata != nil {
		goalID = strings.TrimSpace(plan.Metadata["goal_id"])
	}
	if goalID == "" {
		return nil, fmt.Errorf("%w: continuation plan missing goal identity", ErrGoalInvalidInput)
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	if item.ContinuationCount != plan.Goal.ContinuationCount ||
		item.ContinuationCount <= 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.ContinuationCount--
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Goal continuation deferred before dispatch"
	}
	if err := s.appendEvent(ctx, *updated, "continuation_deferred", protocol.GoalUpdateSourceSystem, plan.RoundID, map[string]any{
		"continuation_count": updated.ContinuationCount,
		"reason":             reason,
	}); err != nil {
		return nil, err
	}
	return updated, nil
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
		"objective":                  objective,
		"room_goal_lead_note":        buildRoomGoalLeadNote(item),
		"completion_tool_retry_note": buildCompletionToolRetryNote(item),
		"tokens_used":                fmt.Sprintf("%d", item.Usage.Total()),
		"token_budget":               tokenBudget,
		"remaining_tokens":           remainingTokens,
	})
}

func buildRoomGoalLeadNote(item protocol.Goal) string {
	if !protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return ""
	}
	leadAgentID := RoomLeadAgentID(item)
	if leadAgentID == "" {
		return ""
	}
	leadName := RoomLeadAgentName(item)
	leadLabel := leadAgentID
	if leadName != "" {
		leadLabel = fmt.Sprintf("%s (%s)", leadName, leadAgentID)
	}
	return strings.TrimSpace(fmt.Sprintf(`
Room Goal lead:
- This is a shared Room Goal. You are the assigned lead agent: %s.
- The Goal belongs to the room, not to your private session. You are responsible for driving coordination, evidence gathering, final audit, and completion.
- Follow all Room rules and member roles. When another member should act, publish a normal public Room message that @mentions exactly that member and states a concrete deliverable.
- Public @ delegation is visible to the user and should be the default for ordinary collaboration. Use private Room directed messages only for secrets, private reminders, hidden collection, or explicitly private work.
- For a multi-member Room Goal, visible collaboration is part of completion. If the runtime provides a Room Goal collaboration requirement, satisfy it before attempting completion.
- If room-visible history does not already show substantive work from a non-lead member for this Goal, your next public reply should @ exactly one non-lead member with a concrete deliverable and you must not call the Goal update tool in that same turn.
- If a public @ delegation is the right next step, make that @ message your public reply for this turn and do not mark the Goal complete yet.
- When delegated work returns, inspect the room-visible evidence, continue or delegate again if needed, and only mark the Goal complete after the full room objective is verified.
`, leadLabel))
}

func buildCompletionToolRetryNote(item protocol.Goal) string {
	if goalCompletionToolRetryCount(item.Metadata) <= 0 {
		return ""
	}
	return strings.TrimSpace(
		"Completion finalization retry:\n" +
			"- A previous goal-continuation response stated that the objective was complete but did not call the Goal update tool.\n" +
			"- In Nexus, the model-visible tool name is `mcp__nexus_goal__update_goal`. Do not conclude it is unavailable because bare `update_goal` is absent.\n" +
			"- Redo the completion audit now. If the objective is complete, call `mcp__nexus_goal__update_goal` with status \"complete\" before any final response. If it is not complete, continue the remaining work.",
	)
}

func escapeGoalPromptText(input string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(input)
}
