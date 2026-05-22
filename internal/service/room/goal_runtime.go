package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (s *RealtimeService) appendGoalRuntimeContext(ctx context.Context, sessionKey string, appendSystemPrompt string) (string, string) {
	if s.goals == nil {
		return appendSystemPrompt, ""
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return appendSystemPrompt, ""
		}
		s.loggerFor(ctx).Warn("读取 Room Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return appendSystemPrompt, ""
	}
	goalID := goalIDForRuntimeUsage(goal)
	if strings.TrimSpace(goalContext) == "" {
		return appendSystemPrompt, goalID
	}
	return appendPromptSection(appendSystemPrompt, goalContext), goalID
}

func goalIDForRuntimeUsage(goal *protocol.Goal) string {
	if goal == nil {
		return ""
	}
	return strings.TrimSpace(goal.ID)
}

func (s *RealtimeService) recordGoalUsageForSlot(ctx context.Context, slot *activeRoomSlot, result runtimectx.RoundExecutionResult) {
	if s.goals == nil || slot == nil || result.Usage.IsZero() {
		return
	}
	usage := protocol.GoalUsage{
		InputTokens:              result.Usage.InputTokens,
		OutputTokens:             result.Usage.OutputTokens,
		CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
		ReasoningTokens:          result.Usage.ReasoningTokens,
		TotalTokens:              result.Usage.TotalTokens,
		RuntimeSeconds:           result.ElapsedTimeSeconds,
	}
	var err error
	if strings.TrimSpace(slot.GoalIDForUsage) != "" {
		_, err = s.goals.RecordUsageForGoal(ctx, slot.GoalIDForUsage, usage, slot.AgentRoundID)
	} else {
		_, err = s.goals.RecordUsageForSession(ctx, slot.RuntimeSessionKey, usage, slot.AgentRoundID)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		s.loggerFor(ctx).Warn("记录 Room Goal usage 失败",
			"session_key", slot.RuntimeSessionKey,
			"goal_id", slot.GoalIDForUsage,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}
