package dm

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (r *roundRunner) startIdleSubagentNotificationDrain() {
	if r == nil || r.service == nil || r.service.runtime == nil || !r.service.runtime.HasSubagentHistory(r.sessionKey) {
		return
	}
	r.service.runtime.StartIdleMessageDrain(r.sessionKey, r.handleIdleSubagentMessage)
}

func (r *roundRunner) handleIdleSubagentMessage(ctx context.Context, incoming sdkprotocol.ReceivedMessage) bool {
	events, durableMessages, _, _, err := r.mapper.Map(incoming)
	if err != nil {
		r.service.loggerFor(ctx).Warn("处理 DM idle subagent 通知失败",
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
			r.service.loggerFor(ctx).Warn("写入 DM idle subagent 通知失败",
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
	if r.hasRunningSubagentTask() {
		return true
	}
	r.dispatchPostRoundWorkAfterSubagents()
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
		if runtimeKind := strings.TrimSpace(r.runtimeKind); runtimeKind != "" {
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
	if !dmMetadataLooksLikeSubagentTask(metadata) && !r.knowsSubagentTask(taskID) {
		return
	}
	r.goalUsageMu.Lock()
	if r.subagentTasks == nil {
		r.subagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_progress", "task_updated":
		if dmIsTerminalSubagentTaskStatus(status) {
			delete(r.subagentTasks, taskID)
			break
		}
		r.subagentTasks[taskID] = struct{}{}
	case "task_notification":
		if dmIsTerminalSubagentTaskStatus(status) {
			delete(r.subagentTasks, taskID)
		}
	}
	r.goalUsageMu.Unlock()
	if r.service != nil && r.service.runtime != nil {
		r.service.runtime.MarkSubagentHistory(r.sessionKey)
	}
}

func (r *roundRunner) knowsSubagentTask(taskID string) bool {
	if r == nil || strings.TrimSpace(taskID) == "" {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	_, ok := r.subagentTasks[strings.TrimSpace(taskID)]
	return ok
}

func (r *roundRunner) hasRunningSubagentTask() bool {
	if r == nil {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return len(r.subagentTasks) > 0
}

func (r *roundRunner) dispatchPostRoundWorkAfterSubagents() {
	if !r.claimSubagentPostRoundDispatch() {
		return
	}
	r.dispatchPostRoundWork()
}

func (r *roundRunner) claimSubagentPostRoundDispatch() bool {
	if r == nil {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if len(r.subagentTasks) > 0 || r.subagentPostRoundDispatched {
		return false
	}
	r.subagentPostRoundDispatched = true
	return true
}

func dmMetadataLooksLikeSubagentTask(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	taskType := strings.ToLower(strings.TrimSpace(dmAnyString(metadata["task_type"])))
	if taskType == "local_shell" {
		return false
	}
	if taskType != "" {
		return taskType == "local_agent"
	}
	return strings.TrimSpace(dmAnyString(metadata["agent_id"])) != "" ||
		strings.TrimSpace(dmAnyString(metadata["agent_type"])) != ""
}

func dmIsTerminalSubagentTaskStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "error", "stopped", "killed", "cancelled":
		return true
	default:
		return false
	}
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
