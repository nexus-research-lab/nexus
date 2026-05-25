package room

import (
	"context"
	"errors"
	"strings"
	"time"

	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (s *RealtimeService) appendGoalRuntimeContextForSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	appendSystemPrompt string,
) (string, string, string) {
	for _, sessionKey := range goalSessionCandidates(roundValue, slot) {
		nextPrompt, goalID, ok := s.appendGoalRuntimeContext(ctx, sessionKey, appendSystemPrompt)
		if !ok {
			continue
		}
		return nextPrompt, goalID, sessionKey
	}
	return appendSystemPrompt, "", ""
}

func goalSessionCandidates(roundValue *activeRoomRound, slot *activeRoomSlot) []string {
	candidates := []string{}
	if roundValue != nil {
		candidates = append(candidates, roundValue.SessionKey)
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

func (s *RealtimeService) appendGoalRuntimeContext(ctx context.Context, sessionKey string, appendSystemPrompt string) (string, string, bool) {
	if s.goals == nil {
		return appendSystemPrompt, "", false
	}
	goalContext, goal, err := s.goals.RuntimeContext(ctx, sessionKey)
	if err != nil {
		if errors.Is(err, goalsvc.ErrGoalDisabled) || errors.Is(err, goalsvc.ErrGoalNotFound) {
			return appendSystemPrompt, "", false
		}
		s.loggerFor(ctx).Warn("读取 Room Goal runtime context 失败", "session_key", sessionKey, "err", err)
		return appendSystemPrompt, "", false
	}
	goalID := goalIDForRuntimeUsage(goal)
	if strings.TrimSpace(goalContext) == "" {
		return appendSystemPrompt, goalID, true
	}
	return appendPromptSection(appendSystemPrompt, goalContext), goalID, true
}

func goalIDForRuntimeUsage(goal *protocol.Goal) string {
	if goal == nil {
		return ""
	}
	return strings.TrimSpace(goal.ID)
}

func beginGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.GoalUsage = goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(slot.GoalIDForUsage) != "")
	slot.GoalUsageStartedAt = time.Now()
}

func (s *RealtimeService) registerSlotGoalRuntime(slot *activeRoomSlot) func() {
	if s.runtime == nil || slot == nil {
		return func() {}
	}
	sessionKey := goalSessionKeyForSlot(slot)
	roundID := strings.TrimSpace(slot.AgentRoundID)
	if sessionKey == "" || roundID == "" {
		return func() {}
	}
	s.runtime.StartRound(sessionKey, roundID, nil)
	s.runtime.RegisterGoalAccountingFlush(sessionKey, roundID, func(ctx context.Context) error {
		return s.flushGoalUsageForSlot(ctx, slot)
	})
	s.runtime.RegisterGoalAccountingClear(sessionKey, roundID, func() {
		clearGoalUsageForSlot(slot)
	})
	return func() {
		s.runtime.RegisterGoalAccountingFlush(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingClear(sessionKey, roundID, nil)
		s.runtime.MarkRoundFinished(sessionKey, roundID)
	}
}

func (s *RealtimeService) recordGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result runtimectx.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil {
		return
	}
	snapshot, ok := slotFinalGoalUsageSnapshot(slot, result, finalAssistant)
	if !ok {
		return
	}
	s.recordGoalUsageSnapshotForSlot(ctx, slot, snapshot)
}

func (s *RealtimeService) recordGoalUsageLimitForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result runtimectx.RoundExecutionResult,
) {
	if s.goals == nil || slot == nil || !result.UsageLimitReached {
		return
	}
	_, err := s.goals.UsageLimitForSession(ctx, goalSessionKeyForSlot(slot), slot.AgentRoundID, result.UsageLimitReason)
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) {
		s.loggerFor(ctx).Warn("标记 Room Goal usage limit 失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", slot.GoalIDForUsage,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}

func (s *RealtimeService) flushGoalUsageForSlot(ctx context.Context, slot *activeRoomSlot) error {
	s.recordGoalUsageForSlot(ctx, slot, runtimectx.RoundExecutionResult{}, slot.lastGoalAssistantMessage())
	return nil
}

func (s *RealtimeService) recordGoalUsageFromSlotAssistantMessage(
	ctx context.Context,
	slot *activeRoomSlot,
	message protocol.Message,
) {
	if s.goals == nil || slot == nil {
		return
	}
	observations := messageutil.AssistantToolResults(message)
	if len(observations) == 0 {
		return
	}
	rememberGoalToolProgressForSlot(slot, messageutil.AssistantHasCountedToolProgress(message))
	snapshot := slotAssistantGoalUsageSnapshot(slot, message)
	hasSuccessfulCreate := false
	hasSuccessfulUpdate := false
	for _, observation := range observations {
		if observation.IsError {
			continue
		}
		switch strings.TrimSpace(observation.ToolName) {
		case "create_goal":
			hasSuccessfulCreate = true
		case "update_goal":
			hasSuccessfulUpdate = true
		}
	}
	if hasSuccessfulCreate {
		slot.stateMu.Lock()
		if slot.GoalUsage == nil || !slot.GoalUsage.Active() {
			if slot.GoalUsage == nil {
				slot.GoalUsage = goalsvc.NewRuntimeUsageAccumulator(false)
			}
			slot.GoalUsage.Reset(snapshot)
			slot.stateMu.Unlock()
			return
		}
		slot.stateMu.Unlock()
	}
	s.recordGoalUsageSnapshotForSlot(ctx, slot, snapshot)
	if hasSuccessfulUpdate {
		clearGoalUsageForSlot(slot)
	}
}

func (s *RealtimeService) recordGoalContinuationProgressForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	roundValue *activeRoomRound,
) {
	if s.goals == nil || slot == nil || strings.TrimSpace(slot.GoalIDForUsage) == "" {
		return
	}
	purpose := ""
	if roundValue != nil {
		purpose = strings.TrimSpace(roundValue.InputOptions.Purpose)
	}
	hasProgress := purpose != "goal_continuation" || slotHasGoalToolProgress(slot)
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

func slotFinalGoalUsageSnapshot(
	slot *activeRoomSlot,
	result runtimectx.RoundExecutionResult,
	finalAssistant protocol.Message,
) (goalsvc.RuntimeUsageSnapshot, bool) {
	usage := runtimectx.GoalUsageFromTokenUsage(result.Usage)
	usageOK := !result.Usage.IsZero()
	if !usageOK && protocol.MessageRole(finalAssistant) == "assistant" {
		usage, usageOK = runtimectx.GoalUsageFromRaw(finalAssistant["usage"])
	}
	elapsedSeconds := result.ElapsedTimeSeconds
	if elapsedSeconds <= 0 {
		elapsedSeconds = slotGoalUsageElapsedSeconds(slot)
	}
	return goalsvc.RuntimeUsageSnapshot{
		Usage:          usage,
		ElapsedSeconds: elapsedSeconds,
	}, usageOK || elapsedSeconds > 0
}

func slotAssistantGoalUsageSnapshot(slot *activeRoomSlot, message protocol.Message) goalsvc.RuntimeUsageSnapshot {
	usage, _ := runtimectx.GoalUsageFromRaw(message["usage"])
	return goalsvc.RuntimeUsageSnapshot{
		Usage:          usage,
		ElapsedSeconds: slotGoalUsageElapsedSeconds(slot),
	}
}

func (s *RealtimeService) recordGoalUsageSnapshotForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	snapshot goalsvc.RuntimeUsageSnapshot,
) {
	if s.goals == nil || slot == nil {
		return
	}
	slot.stateMu.Lock()
	if slot.GoalUsage != nil {
		usage, ok := slot.GoalUsage.Delta(snapshot)
		slot.stateMu.Unlock()
		if !ok {
			return
		}
		s.recordGoalUsageDeltaForSlot(ctx, slot, usage)
		return
	}
	slot.stateMu.Unlock()
	usage := snapshot.Usage
	usage.RuntimeSeconds = snapshot.ElapsedSeconds
	if isZeroRoomGoalUsage(usage) {
		return
	}
	s.recordGoalUsageDeltaForSlot(ctx, slot, usage)
}

func (s *RealtimeService) recordGoalUsageDeltaForSlot(ctx context.Context, slot *activeRoomSlot, usage protocol.GoalUsage) {
	if s.goals == nil || slot == nil || isZeroRoomGoalUsage(usage) {
		return
	}
	var err error
	if strings.TrimSpace(slot.GoalIDForUsage) != "" {
		_, err = s.goals.RecordUsageForGoal(ctx, slot.GoalIDForUsage, usage, slot.AgentRoundID)
	} else {
		_, err = s.goals.RecordUsageForSession(ctx, goalSessionKeyForSlot(slot), usage, slot.AgentRoundID)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		s.loggerFor(ctx).Warn("记录 Room Goal usage 失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", slot.GoalIDForUsage,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}

func clearGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.stateMu.Lock()
	defer slot.stateMu.Unlock()
	if slot.GoalUsage != nil {
		slot.GoalUsage.Close()
	}
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

func slotGoalUsageElapsedSeconds(slot *activeRoomSlot) int64 {
	if slot == nil || slot.GoalUsageStartedAt.IsZero() {
		return 0
	}
	elapsed := int64(time.Since(slot.GoalUsageStartedAt).Seconds())
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func isZeroRoomGoalUsage(usage protocol.GoalUsage) bool {
	return usage.InputTokens == 0 &&
		usage.OutputTokens == 0 &&
		usage.CacheCreationInputTokens == 0 &&
		usage.CacheReadInputTokens == 0 &&
		usage.ReasoningTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.RuntimeSeconds == 0
}
