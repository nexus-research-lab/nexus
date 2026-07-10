package room

import (
	"context"
	"errors"
	"strings"
	"time"

	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func beginGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil || slot.GoalRuntimeIgnored {
		return
	}
	slot.GoalUsage = goalsvc.NewRuntimeUsageAccumulator(strings.TrimSpace(slot.GoalIDForUsage) != "")
	slot.GoalUsageStartedAt = time.Now()
}

func (s *RealtimeService) registerSlotGoalRuntime(slot *activeRoomSlot) func() {
	if s.runtime == nil || slot == nil || slot.GoalRuntimeIgnored {
		return func() {}
	}
	sessionKey := goalSessionKeyForSlot(slot)
	roundID := strings.TrimSpace(slot.AgentRoundID)
	if sessionKey == "" || roundID == "" {
		return func() {}
	}
	// execution.go 会先登记 slot runtime；独立调用本 helper 的测试/工具路径仍需自行占用 round。
	ownRound := true
	for _, runningRoundID := range s.runtime.GetRunningRoundIDs(sessionKey) {
		if runningRoundID == roundID {
			ownRound = false
			break
		}
	}
	if ownRound {
		s.runtime.StartRound(sessionKey, roundID, nil)
	}
	s.runtime.RegisterGoalAccountingFlush(sessionKey, roundID, func(ctx context.Context) error {
		return s.flushGoalUsageForSlot(ctx, slot)
	})
	s.runtime.RegisterGoalAccountingClear(sessionKey, roundID, func() {
		clearGoalUsageForSlot(slot)
	})
	s.runtime.RegisterGoalAccountingActivate(sessionKey, roundID, func(ctx context.Context) error {
		activateGoalUsageForSlot(ctx, slot)
		return nil
	})
	return func() {
		s.runtime.RegisterGoalAccountingFlush(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingClear(sessionKey, roundID, nil)
		s.runtime.RegisterGoalAccountingActivate(sessionKey, roundID, nil)
		if ownRound {
			s.runtime.MarkRoundFinished(sessionKey, roundID)
		}
	}
}

func (s *RealtimeService) recordGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.GoalRuntimeIgnored {
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
	result exec.RoundExecutionResult,
) {
	if s.goals == nil || slot == nil || slot.GoalRuntimeIgnored || !result.UsageLimitReached {
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
	s.recordGoalUsageForSlot(ctx, slot, exec.RoundExecutionResult{}, slot.lastGoalAssistantMessage())
	return nil
}

func (s *RealtimeService) recordGoalUsageFromSlotAssistantMessage(
	ctx context.Context,
	slot *activeRoomSlot,
	message protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.GoalRuntimeIgnored {
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
		switch messageutil.CanonicalToolName(observation.ToolName) {
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

func slotFinalGoalUsageSnapshot(
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
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
	if s.goals == nil || slot == nil || slot.GoalRuntimeIgnored {
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
	if s.goals == nil || slot == nil || slot.GoalRuntimeIgnored || isZeroRoomGoalUsage(usage) {
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

func activateGoalUsageForSlot(_ context.Context, slot *activeRoomSlot) {
	if slot == nil || slot.GoalRuntimeIgnored {
		return
	}
	snapshot := slotAssistantGoalUsageSnapshot(slot, slot.lastGoalAssistantMessage())
	slot.stateMu.Lock()
	defer slot.stateMu.Unlock()
	if slot.GoalUsage == nil {
		slot.GoalUsage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	slot.GoalUsage.Reset(snapshot)
}

func slotGoalUsageElapsedSeconds(slot *activeRoomSlot) int64 {
	if slot == nil || slot.GoalUsageStartedAt.IsZero() {
		return 0
	}
	elapsed := int64(time.Since(slot.GoalUsageStartedAt).Seconds())
	return max(elapsed, 0)
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
