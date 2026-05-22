package dm

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

func (r *roundRunner) recordGoalUsage(result runtimectx.RoundExecutionResult, finalAssistant protocol.Message) {
	if r.service.goals == nil {
		return
	}
	snapshot, ok := r.finalGoalUsageSnapshot(result, finalAssistant)
	if !ok {
		return
	}
	r.recordGoalUsageSnapshot(snapshot)
}

func (r *roundRunner) recordGoalUsageFromAssistantMessage(message protocol.Message) {
	if r.service.goals == nil {
		return
	}
	observations := messageutil.AssistantToolResults(message)
	if len(observations) == 0 {
		return
	}
	snapshot := r.assistantGoalUsageSnapshot(message)
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
	if hasSuccessfulCreate && (r.goalUsage == nil || !r.goalUsage.Active()) {
		r.ensureGoalUsageAccumulator(false).Reset(snapshot)
		return
	}
	r.recordGoalUsageSnapshot(snapshot)
	if hasSuccessfulUpdate && r.goalUsage != nil {
		r.goalUsage.Close()
	}
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

func (r *roundRunner) recordGoalUsageSnapshot(snapshot goalsvc.RuntimeUsageSnapshot) {
	if r.service.goals == nil {
		return
	}
	if r.goalUsage != nil {
		usage, ok := r.goalUsage.Delta(snapshot)
		if !ok {
			return
		}
		r.recordGoalUsageDelta(usage)
		return
	}
	usage := snapshot.Usage
	usage.RuntimeSeconds = snapshot.ElapsedSeconds
	if isZeroGoalUsage(usage) {
		return
	}
	r.recordGoalUsageDelta(usage)
}

func (r *roundRunner) recordGoalUsageDelta(usage protocol.GoalUsage) {
	if r.service.goals == nil || isZeroGoalUsage(usage) {
		return
	}
	var err error
	if strings.TrimSpace(r.goalIDForUsage) != "" {
		_, err = r.service.goals.RecordUsageForGoal(context.Background(), r.goalIDForUsage, usage, r.roundID)
	} else {
		_, err = r.service.goals.RecordUsageForSession(context.Background(), r.sessionKey, usage, r.roundID)
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

func (r *roundRunner) ensureGoalUsageAccumulator(active bool) *goalsvc.RuntimeUsageAccumulator {
	if r.goalUsage == nil {
		r.goalUsage = goalsvc.NewRuntimeUsageAccumulator(active)
	}
	return r.goalUsage
}

func (r *roundRunner) elapsedGoalUsageSeconds() int64 {
	if r.goalUsageStarted.IsZero() {
		return 0
	}
	elapsed := int64(time.Since(r.goalUsageStarted).Seconds())
	if elapsed < 0 {
		return 0
	}
	return elapsed
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
