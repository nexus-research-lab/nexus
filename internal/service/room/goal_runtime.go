package room

import (
	"cmp"
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (s *RealtimeService) resolveGoalRuntimeContextForSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	appendSystemPrompt string,
) (string, string, string, string) {
	defaultGoalSessionKey := ""
	if roundValue != nil {
		defaultGoalSessionKey = strings.TrimSpace(roundValue.SessionKey)
	}
	for _, sessionKey := range goalSessionCandidates(roundValue, slot) {
		goalContext, goalID, ok := s.goalRuntimeContext(ctx, sessionKey)
		if !ok {
			continue
		}
		return appendSystemPrompt, goalContext, goalID, sessionKey
	}
	return appendSystemPrompt, "", "", defaultGoalSessionKey
}

func goalSessionCandidates(roundValue *activeRoomRound, slot *activeRoomSlot) []string {
	candidates := []string{}
	if roundValue != nil {
		roundSessionKey := strings.TrimSpace(roundValue.SessionKey)
		if protocol.IsRoomSharedSessionKey(roundSessionKey) {
			return []string{roundSessionKey}
		}
		candidates = append(candidates, roundSessionKey)
	}
	if slot != nil {
		candidates = append(candidates, slot.RuntimeSessionKey)
	}
	result := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		sessionKey := strings.TrimSpace(candidate)
		if sessionKey == "" {
			continue
		}
		if _, exists := seen[sessionKey]; exists {
			continue
		}
		seen[sessionKey] = struct{}{}
		result = append(result, sessionKey)
	}
	return result
}

func (s *RealtimeService) goalRuntimeContext(ctx context.Context, sessionKey string) (string, string, bool) {
	if s.goals == nil {
		return "", "", false
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return "", "", false
		}
		s.loggerFor(ctx).Warn("读取 Room Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return "", "", false
	}
	goalID := ""
	if goal != nil {
		goalID = strings.TrimSpace(goal.ID)
	}
	if strings.TrimSpace(goalContext) == "" {
		return "", goalID, true
	}
	return strings.TrimSpace(goalContext), goalID, true
}

func (s *RealtimeService) recordGoalContinuationProgressForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	roundValue *activeRoomRound,
	result runtimectx.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.GoalRuntimeIgnored || strings.TrimSpace(slot.GoalIDForUsage) == "" {
		return
	}
	s.recordRoomGoalCollaborationEvidenceForSlot(ctx, slot, finalAssistant)
	purpose := ""
	if roundValue != nil {
		purpose = strings.TrimSpace(roundValue.InputOptions.Purpose)
	}
	if purpose == "goal_continuation" && result.TerminalStatus == "error" {
		reason := cmp.Or(
			strings.TrimSpace(result.ErrorMessage),
			messageutil.ExtractAssistantDisplayText(finalAssistant),
			"Goal continuation runtime failed",
		)
		_, err := s.goals.RecordContinuationFailure(ctx, slot.GoalIDForUsage, slot.AgentRoundID, reason)
		if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
			s.loggerFor(ctx).Warn("记录 Room Goal 续跑失败原因失败",
				"session_key", goalSessionKeyForSlot(slot),
				"goal_id", slot.GoalIDForUsage,
				"round_id", slot.AgentRoundID,
				"err", err,
			)
		}
		return
	}
	if purpose != "goal_continuation" {
		_, err := s.goals.RecordGoalActivity(ctx, slot.GoalIDForUsage, slot.AgentRoundID)
		if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
			s.loggerFor(ctx).Warn("记录 Room Goal 显式活动失败",
				"session_key", goalSessionKeyForSlot(slot),
				"goal_id", slot.GoalIDForUsage,
				"round_id", slot.AgentRoundID,
				"err", err,
			)
		}
		return
	}
	if messageutil.AssistantMissedGoalCompletionTool(finalAssistant) {
		reason := "assistant claimed goal completion but did not call mcp__nexus_goal__update_goal"
		_, err := s.goals.RecordCompletionToolMiss(ctx, slot.GoalIDForUsage, slot.AgentRoundID, reason)
		if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
			s.loggerFor(ctx).Warn("记录 Room Goal 完成工具漏调用失败",
				"session_key", goalSessionKeyForSlot(slot),
				"goal_id", slot.GoalIDForUsage,
				"round_id", slot.AgentRoundID,
				"err", err,
			)
		}
		return
	}
	hasProgress := slotHasGoalToolProgress(slot)
	_, err := s.goals.RecordContinuationProgress(ctx, slot.GoalIDForUsage, slot.AgentRoundID, hasProgress)
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
		s.loggerFor(ctx).Warn("记录 Room Goal 续跑进展失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", slot.GoalIDForUsage,
			"round_id", slot.AgentRoundID,
			"progressed", hasProgress,
			"err", err,
		)
	}
}

func (s *RealtimeService) recordRoomGoalCollaborationEvidenceForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	finalAssistant protocol.Message,
) {
	if s == nil || s.goals == nil || slot == nil || !protocol.IsRoomSharedSessionKey(goalSessionKeyForSlot(slot)) {
		return
	}
	if roomdomain.IsNoReplyAssistantMessage(finalAssistant) {
		return
	}
	if strings.TrimSpace(messageutil.ExtractAssistantDisplayText(finalAssistant)) == "" {
		return
	}
	if _, err := s.goals.RecordRoomGoalCollaborationEvidence(ctx, slot.GoalIDForUsage, slot.AgentRoundID, slot.AgentID); err != nil &&
		!errors.Is(err, goalsvc.ErrGoalDisabled) &&
		!errors.Is(err, goalsvc.ErrGoalNotFound) &&
		!errors.Is(err, goalsvc.ErrGoalInvalidState) &&
		!errors.Is(err, goalsvc.ErrGoalVersionStale) {
		s.loggerFor(ctx).Warn("记录 Room Goal 协作证据失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", slot.GoalIDForUsage,
			"round_id", slot.AgentRoundID,
			"agent_id", slot.AgentID,
			"err", err,
		)
	}
}

func rememberGoalToolProgressForSlot(slot *activeRoomSlot, progressed bool) {
	if slot == nil || !progressed {
		return
	}
	slot.stateMu.Lock()
	slot.GoalToolProgress = true
	slot.stateMu.Unlock()
}

func slotHasGoalToolProgress(slot *activeRoomSlot) bool {
	if slot == nil {
		return false
	}
	slot.stateMu.RLock()
	defer slot.stateMu.RUnlock()
	return slot.GoalToolProgress
}

func goalSessionKeyForSlot(slot *activeRoomSlot) string {
	if slot == nil {
		return ""
	}
	if sessionKey := strings.TrimSpace(slot.GoalSessionKey); sessionKey != "" {
		return sessionKey
	}
	return strings.TrimSpace(slot.RuntimeSessionKey)
}
