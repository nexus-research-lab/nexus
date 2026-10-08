// INPUT: DM parent terminal 后到达的 nxs child lifecycle/usage 消息。
// OUTPUT: child durable history、最终 Goal usage join 与一次性 post-round 派发。
// POS: DM parent 结束后等待 child source 收敛的协调边界。
package dm

import (
	"context"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	subagentParentTerminalNormal      = "normal"
	subagentParentTerminalFailed      = "failed"
	subagentParentTerminalInterrupted = "interrupted"
)

func (r *roundRunner) startIdleSubagentNotificationDrain() {
	if r == nil || r.service == nil || r.service.Runtime == nil || !r.service.Runtime.HasSubagentHistory(r.sessionKey) {
		return
	}
	r.service.Runtime.StartIdleMessageDrain(r.sessionKey, r.handleIdleSubagentMessage)
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
	if r.RememberSubagentTaskMessage(message) {
		r.service.Runtime.MarkSubagentHistory(r.sessionKey)
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
