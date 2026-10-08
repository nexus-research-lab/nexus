// INPUT: DM parent terminal 后到达的 nxs child lifecycle/usage 消息。
// OUTPUT: child durable history、最终 Goal usage join 与一次性 post-round 派发。
// POS: DM parent 结束后等待 child source 收敛的协调边界。
package dm

import (
	"context"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

const (
	subagentParentTerminalNormal      = "normal"
	subagentParentTerminalFailed      = "failed"
	subagentParentTerminalInterrupted = "interrupted"
)

func (r *roundRunner) startIdleSubagentNotificationDrain() {
	if r == nil || r.service == nil || r.service.runtime == nil || !r.service.runtime.HasSubagentHistory(r.sessionKey) {
		return
	}
	r.service.runtime.StartIdleMessageDrain(r.sessionKey, r.handleIdleSubagentMessage)
}

func (r *roundRunner) handleIdleSubagentMessage(ctx context.Context, incoming sdkprotocol.ReceivedMessage) bool {
	r.service.ExecutionObserver().ObserveMessage(r.orchestrationActor(), incoming)
	events, durableMessages, _, _, err := r.mapper.Map(incoming)
	if err != nil {
		r.service.LoggerFor(ctx).Warn("处理 DM idle subagent 通知失败",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
			"err", err,
		)
		return true
	}
	for _, message := range durableMessages {
		if message == nil {
			continue
		}
		if err := r.handleDurableMessage(message); err != nil {
			r.service.LoggerFor(ctx).Warn("写入 DM idle subagent 通知失败",
				"session_key", r.sessionKey,
				"round_id", r.roundID,
				"err", err,
			)
			return true
		}
	}
	for _, event := range events {
		r.service.broadcastEventWithTimeout(context.Background(), r.sessionKey, event)
	}
	if r.HasRunningSubagentTask() {
		return true
	}
	r.completeSubagentJoinAfterParentTerminal()
	// nxs 支持用同 task ID 唤醒终态 task，因此 idle drain 不能在首次完成时退出。
	return true
}

func (r *roundRunner) annotateSubagentTaskRuntimeKind(message protocol.Message) {
	if r == nil || message == nil {
		return
	}
	metadata, _ := message["metadata"].(map[string]any)
	if strings.TrimSpace(dmAnyString(metadata["task_id"])) == "" {
		return
	}
	switch strings.TrimSpace(dmAnyString(metadata["subtype"])) {
	case "task_started", "task_progress", "task_updated", "task_notification":
		if runtimeKind := r.runtimeKind; runtimeKind != "" {
			metadata["runtime_kind"] = runtimeKind
		}
	}
}

func (r *roundRunner) rememberSubagentTaskMessage(message protocol.Message) {
	if r == nil {
		return
	}
	metadata, _ := message["metadata"].(map[string]any)
	taskID := strings.TrimSpace(dmAnyString(metadata["task_id"]))
	if taskID == "" {
		return
	}
	subtype := strings.TrimSpace(dmAnyString(metadata["subtype"]))
	status := strings.TrimSpace(dmAnyString(metadata["status"]))
	if !messageutil.IsSubagentTaskMetadata(metadata) && !r.KnowsSubagentTask(taskID) {
		return
	}
	r.Mu.Lock()
	if r.SubagentTasks == nil {
		r.SubagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_progress", "task_updated":
		if messageutil.IsTerminalSubagentTaskStatus(status) {
			delete(r.SubagentTasks, taskID)
			break
		}
		r.SubagentTasks[taskID] = struct{}{}
	case "task_notification":
		if messageutil.IsTerminalSubagentTaskStatus(status) {
			delete(r.SubagentTasks, taskID)
		}
	}
	r.Mu.Unlock()
	if r.service != nil && r.service.runtime != nil {
		r.service.runtime.MarkSubagentHistory(r.sessionKey)
	}
}

func (r *roundRunner) markSubagentUsageObservationPending(
	taskID string,
	observation goalsvc.SubagentUsageObservation,
) {
	if r == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	r.Mu.Lock()
	r.markSubagentUsageObservationPendingLocked(taskID, observation)
	r.Mu.Unlock()
}

func (r *roundRunner) markSubagentUsageObservationPendingLocked(
	taskID string,
	observation goalsvc.SubagentUsageObservation,
) {
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	}
	if r.SubagentUsagePending == nil {
		r.SubagentUsagePending = make(map[string]goalsvc.SubagentUsageObservation)
	}
	taskID = strings.TrimSpace(taskID)
	r.SubagentUsagePending[taskID] = r.SubagentUsagePending[taskID].Merge(observation)
}

func (r *roundRunner) clearSubagentUsageObservationPending(
	taskID string,
	settled goalsvc.SubagentUsageObservation,
) {
	if r == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	r.Mu.Lock()
	r.clearSubagentUsageObservationPendingLocked(taskID, settled)
	r.Mu.Unlock()
}

func (r *roundRunner) clearSubagentUsageObservationPendingLocked(
	taskID string,
	settled goalsvc.SubagentUsageObservation,
) {
	taskID = strings.TrimSpace(taskID)
	if pending, ok := r.SubagentUsagePending[taskID]; ok &&
		pending.CoveredBy(settled) {
		delete(r.SubagentUsagePending, taskID)
	}
}

func (r *roundRunner) markSubagentParentTerminal(status string) {
	if r == nil {
		return
	}
	r.Mu.Lock()
	r.subagentParentTerminal = strings.TrimSpace(status)
	r.Mu.Unlock()
}

func (r *roundRunner) subagentParentTerminalStatus() string {
	if r == nil {
		return ""
	}
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.subagentParentTerminal
}

func (r *roundRunner) completeSubagentJoinAfterParentTerminal() bool {
	switch r.subagentParentTerminalStatus() {
	case subagentParentTerminalNormal:
		return r.dispatchPostRoundWorkAfterSubagents()
	case subagentParentTerminalFailed, subagentParentTerminalInterrupted:
		if !r.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
			r.startGoalUsageRetryWorker()
			r.service.LoggerFor(context.Background()).Warn(
				"DM 异常终态 Goal usage 等待后续重试",
				"session_key", r.sessionKey,
				"round_id", r.roundID,
			)
			return false
		}
	}
	return true
}

func (r *roundRunner) dispatchPostRoundWorkAfterSubagents() bool {
	if r.subagentPostRoundWasDispatched() {
		return true
	}
	if !r.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		r.startGoalUsageRetryWorker()
		r.service.LoggerFor(context.Background()).Warn(
			"DM Goal usage 等待后续重试，暂不派发 post-round work",
			"session_key", r.sessionKey,
			"round_id", r.roundID,
		)
		return false
	}
	if !r.claimSubagentPostRoundDispatch() {
		return true
	}
	r.dispatchPostRoundWork()
	return true
}

func (r *roundRunner) subagentPostRoundWasDispatched() bool {
	if r == nil {
		return false
	}
	r.Mu.Lock()
	defer r.Mu.Unlock()
	return r.subagentPostRoundDispatched
}

func (r *roundRunner) claimSubagentPostRoundDispatch() bool {
	if r == nil {
		return false
	}
	r.Mu.Lock()
	defer r.Mu.Unlock()
	if len(r.SubagentTasks) > 0 ||
		len(r.SubagentUsagePending) > 0 ||
		r.subagentPostRoundDispatched {
		return false
	}
	r.subagentPostRoundDispatched = true
	return true
}

func dmAnyString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
