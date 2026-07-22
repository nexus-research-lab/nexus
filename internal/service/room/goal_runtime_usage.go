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
	if slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	slot.beginGoalUsage()
}

func (s *RealtimeService) registerSlotGoalRuntime(slot *activeRoomSlot) func() {
	if s.runtime == nil || slot == nil || slot.goalRuntimeIgnored() {
		return func() {}
	}
	sessionKey := goalSessionKeyForSlot(slot)
	roundID := strings.TrimSpace(slot.AgentRoundID)
	if sessionKey == "" || roundID == "" {
		return func() {}
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
	}
}

func (s *RealtimeService) recordGoalUsageForSlot(
	ctx context.Context,
	slot *activeRoomSlot,
	result exec.RoundExecutionResult,
	finalAssistant protocol.Message,
) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
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
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() || !result.UsageLimitReached {
		return
	}
	_, err := s.goals.UsageLimitForSession(ctx, goalSessionKeyForSlot(slot), slot.AgentRoundID, result.UsageLimitReason)
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) {
		s.loggerFor(ctx).Warn("标记 Room Goal usage limit 失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", slot.goalIDForUsage(),
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
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
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
		if slot.resetGoalUsageIfInactive(snapshot) {
			return
		}
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
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	if usage, ok, tracked := slot.goalUsageDelta(snapshot); tracked {
		if ok {
			s.recordGoalUsageDeltaForSlot(ctx, slot, usage)
		}
		return
	}
	usage := snapshot.Usage
	usage.RuntimeSeconds = snapshot.ElapsedSeconds
	if isZeroRoomGoalUsage(usage) {
		return
	}
	s.recordGoalUsageDeltaForSlot(ctx, slot, usage)
}

func (s *RealtimeService) recordGoalUsageDeltaForSlot(ctx context.Context, slot *activeRoomSlot, usage protocol.GoalUsage) {
	if s.goals == nil || slot == nil || slot.goalRuntimeIgnored() || isZeroRoomGoalUsage(usage) {
		return
	}
	var err error
	goalID := slot.goalIDForUsage()
	if strings.TrimSpace(goalID) != "" {
		_, err = s.goals.RecordUsageForGoal(ctx, goalID, usage, slot.AgentRoundID)
	} else {
		_, err = s.goals.RecordUsageForSession(ctx, goalSessionKeyForSlot(slot), usage, slot.AgentRoundID)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		s.loggerFor(ctx).Warn("记录 Room Goal usage 失败",
			"session_key", goalSessionKeyForSlot(slot),
			"goal_id", goalID,
			"round_id", slot.AgentRoundID,
			"err", err,
		)
	}
}

func clearGoalUsageForSlot(slot *activeRoomSlot) {
	if slot == nil {
		return
	}
	slot.closeGoalUsage()
}

func activateGoalUsageForSlot(_ context.Context, slot *activeRoomSlot) {
	if slot == nil || slot.goalRuntimeIgnored() {
		return
	}
	snapshot := slotAssistantGoalUsageSnapshot(slot, slot.lastGoalAssistantMessage())
	slot.resetGoalUsage(snapshot)
}

func slotGoalUsageElapsedSeconds(slot *activeRoomSlot) int64 {
	startedAt := slot.goalUsageStartedAt()
	if startedAt.IsZero() {
		return 0
	}
	elapsed := int64(time.Since(startedAt).Seconds())
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
