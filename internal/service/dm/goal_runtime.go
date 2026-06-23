package dm

import (
	"context"
	"errors"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func (r *roundRunner) recordGoalUsage(ctx context.Context, result runtimectx.RoundExecutionResult, finalAssistant protocol.Message) {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return
	}
	snapshot, ok := r.finalGoalUsageSnapshot(result, finalAssistant)
	if !ok {
		return
	}
	r.recordGoalUsageSnapshot(ctx, snapshot)
}

func (r *roundRunner) recordGoalUsageLimit(result runtimectx.RoundExecutionResult) {
	if r.service.goals == nil || r.ignoreGoalRuntime() || !result.UsageLimitReached {
		return
	}
	_, err := r.service.goals.UsageLimitForSession(context.Background(), r.sessionKey, r.roundID, result.UsageLimitReason)
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) {
		r.service.loggerFor(context.Background()).Warn("标记 Goal usage limit 失败",
			"session_key", r.sessionKey,
			"goal_id", r.goalIDForUsage,
			"round_id", r.roundID,
			"err", err,
		)
	}
}

func (r *roundRunner) flushGoalUsage(ctx context.Context) error {
	r.recordGoalUsage(ctx, runtimectx.RoundExecutionResult{}, r.lastGoalAssistantMessage())
	return nil
}

func (r *roundRunner) clearGoalUsage() {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if r.goalUsage != nil {
		r.goalUsage.Close()
	}
}

func (r *roundRunner) activateGoalUsage(context.Context) error {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return nil
	}
	snapshot := r.assistantGoalUsageSnapshot(r.lastGoalAssistantMessage())
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if r.goalUsage == nil {
		r.goalUsage = goalsvc.NewRuntimeUsageAccumulator(false)
	}
	r.goalUsage.Reset(snapshot)
	return nil
}

func (r *roundRunner) rememberGoalAssistantMessage(message protocol.Message) {
	if protocol.MessageRole(message) != "assistant" {
		return
	}
	r.goalUsageMu.Lock()
	r.goalLastAssistant = protocol.Clone(message)
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) lastGoalAssistantMessage() protocol.Message {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return protocol.Clone(r.goalLastAssistant)
}

func (r *roundRunner) recordGoalUsageFromAssistantMessage(message protocol.Message) {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return
	}
	observations := messageutil.AssistantToolResults(message)
	if len(observations) == 0 {
		return
	}
	r.rememberGoalToolProgress(messageutil.AssistantHasCountedToolProgress(message))
	snapshot := r.assistantGoalUsageSnapshot(message)
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
		r.goalUsageMu.Lock()
		if r.goalUsage == nil || !r.goalUsage.Active() {
			if r.goalUsage == nil {
				r.goalUsage = goalsvc.NewRuntimeUsageAccumulator(false)
			}
			r.goalUsage.Reset(snapshot)
			r.goalUsageMu.Unlock()
			return
		}
		r.goalUsageMu.Unlock()
	}
	r.recordGoalUsageSnapshot(context.Background(), snapshot)
	if hasSuccessfulUpdate {
		r.clearGoalUsage()
	}
}

func (r *roundRunner) recordGoalContinuationProgress(result runtimectx.RoundExecutionResult) {
	if r.service.goals == nil || r.ignoreGoalRuntime() || strings.TrimSpace(r.goalIDForUsage) == "" {
		return
	}
	if strings.TrimSpace(r.inputOptions.Purpose) == "goal_continuation" && result.TerminalStatus == "error" {
		assistantText := ""
		if r.mapper != nil {
			assistantText = messageutil.ExtractAssistantDisplayText(r.mapper.LastAssistantMessage())
		}
		reason := dmdomain.FirstNonEmpty(
			strings.TrimSpace(result.ErrorMessage),
			assistantText,
			"Goal continuation runtime failed",
		)
		_, err := r.service.goals.RecordContinuationFailure(context.Background(), r.goalIDForUsage, r.roundID, reason)
		if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
			r.service.loggerFor(context.Background()).Warn("记录 Goal 续跑失败原因失败",
				"session_key", r.sessionKey,
				"goal_id", r.goalIDForUsage,
				"round_id", r.roundID,
				"err", err,
			)
		}
		return
	}
	if strings.TrimSpace(r.inputOptions.Purpose) != "goal_continuation" {
		_, err := r.service.goals.RecordGoalActivity(context.Background(), r.goalIDForUsage, r.roundID)
		if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
			r.service.loggerFor(context.Background()).Warn("记录 Goal 显式活动失败",
				"session_key", r.sessionKey,
				"goal_id", r.goalIDForUsage,
				"round_id", r.roundID,
				"err", err,
			)
		}
		return
	}
	if messageutil.AssistantMissedGoalCompletionTool(r.lastGoalAssistantMessage()) {
		reason := "assistant claimed goal completion but did not call mcp__nexus_goal__update_goal"
		_, err := r.service.goals.RecordCompletionToolMiss(context.Background(), r.goalIDForUsage, r.roundID, reason)
		if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
			r.service.loggerFor(context.Background()).Warn("记录 Goal 完成工具漏调用失败",
				"session_key", r.sessionKey,
				"goal_id", r.goalIDForUsage,
				"round_id", r.roundID,
				"err", err,
			)
		}
		return
	}
	progressed := r.hasGoalToolProgress()
	_, err := r.service.goals.RecordContinuationProgress(context.Background(), r.goalIDForUsage, r.roundID, progressed)
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) && !errors.Is(err, goalsvc.ErrGoalInvalidState) && !errors.Is(err, goalsvc.ErrGoalVersionStale) {
		r.service.loggerFor(context.Background()).Warn("记录 Goal 续跑进展失败",
			"session_key", r.sessionKey,
			"goal_id", r.goalIDForUsage,
			"round_id", r.roundID,
			"progressed", progressed,
			"err", err,
		)
	}
}

func (r *roundRunner) rememberGoalToolProgress(progressed bool) {
	if !progressed {
		return
	}
	r.goalUsageMu.Lock()
	r.goalToolProgress = true
	r.goalUsageMu.Unlock()
}

func (r *roundRunner) hasGoalToolProgress() bool {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.goalToolProgress
}

func (r *roundRunner) finalGoalUsageSnapshot(
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
		elapsedSeconds = r.elapsedGoalUsageSeconds()
	}
	return goalsvc.RuntimeUsageSnapshot{
		Usage:          usage,
		ElapsedSeconds: elapsedSeconds,
	}, usageOK || elapsedSeconds > 0
}

func (r *roundRunner) assistantGoalUsageSnapshot(message protocol.Message) goalsvc.RuntimeUsageSnapshot {
	usage, _ := runtimectx.GoalUsageFromRaw(message["usage"])
	return goalsvc.RuntimeUsageSnapshot{
		Usage:          usage,
		ElapsedSeconds: r.elapsedGoalUsageSeconds(),
	}
}

func (r *roundRunner) recordGoalUsageSnapshot(ctx context.Context, snapshot goalsvc.RuntimeUsageSnapshot) {
	if r.service.goals == nil {
		return
	}
	r.goalUsageMu.Lock()
	if r.goalUsage != nil {
		usage, ok := r.goalUsage.Delta(snapshot)
		r.goalUsageMu.Unlock()
		if !ok {
			return
		}
		r.recordGoalUsageDelta(ctx, usage)
		return
	}
	r.goalUsageMu.Unlock()
	usage := snapshot.Usage
	usage.RuntimeSeconds = snapshot.ElapsedSeconds
	if isZeroGoalUsage(usage) {
		return
	}
	r.recordGoalUsageDelta(ctx, usage)
}

func (r *roundRunner) recordGoalUsageDelta(ctx context.Context, usage protocol.GoalUsage) {
	if r.service.goals == nil || r.ignoreGoalRuntime() || isZeroGoalUsage(usage) {
		return
	}
	var err error
	if strings.TrimSpace(r.goalIDForUsage) != "" {
		_, err = r.service.goals.RecordUsageForGoal(ctx, r.goalIDForUsage, usage, r.roundID)
	} else {
		_, err = r.service.goals.RecordUsageForSession(ctx, r.sessionKey, usage, r.roundID)
	}
	if err != nil && !errors.Is(err, goalsvc.ErrGoalDisabled) && !errors.Is(err, goalsvc.ErrGoalNotFound) {
		r.service.loggerFor(context.Background()).Warn("记录 Goal usage 失败",
			"session_key", r.sessionKey,
			"goal_id", r.goalIDForUsage,
			"round_id", r.roundID,
			"err", err,
		)
	}
}

func (r *roundRunner) elapsedGoalUsageSeconds() int64 {
	if r.goalUsageStarted.IsZero() {
		return 0
	}
	elapsed := int64(time.Since(r.goalUsageStarted).Seconds())
	return max(elapsed, 0)
}

func (r *roundRunner) ignoreGoalRuntime() bool {
	if r == nil {
		return false
	}
	return goalsvc.ShouldIgnoreRuntimeForPermissionMode(string(r.permissionMode))
}

func isZeroGoalUsage(usage protocol.GoalUsage) bool {
	return usage.InputTokens == 0 &&
		usage.OutputTokens == 0 &&
		usage.CacheCreationInputTokens == 0 &&
		usage.CacheReadInputTokens == 0 &&
		usage.ReasoningTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.RuntimeSeconds == 0
}
