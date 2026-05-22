package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (s *RealtimeService) appendGoalRuntimeContext(ctx context.Context, sessionKey string, appendSystemPrompt string) string {
	if s.goals == nil {
		return appendSystemPrompt
	}
	goalContext, _, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return appendSystemPrompt
		}
		s.loggerFor(ctx).Warn("读取 Room Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return appendSystemPrompt
	}
	if strings.TrimSpace(goalContext) == "" {
		return appendSystemPrompt
	}
	return appendPromptSection(appendSystemPrompt, goalContext)
}

func (s *RealtimeService) recordGoalUsageForSlot(ctx context.Context, slot *activeRoomSlot, result runtimectx.RoundExecutionResult) {
	if s.goals == nil || slot == nil || result.Usage.IsZero() {
		return
	}
	_, err := s.goals.RecordUsageForSession(ctx, slot.RuntimeSessionKey, protocol.GoalUsage{
		InputTokens:              result.Usage.InputTokens,
		OutputTokens:             result.Usage.OutputTokens,
		CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
		CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
		ReasoningTokens:          result.Usage.ReasoningTokens,
		TotalTokens:              result.Usage.TotalTokens,
	}, slot.AgentRoundID)
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		s.loggerFor(ctx).Warn("记录 Room Goal usage 失败",
			"session_key", slot.RuntimeSessionKey,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}
