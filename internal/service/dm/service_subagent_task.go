package dm

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (r *roundRunner) startIdleSubagentNotificationDrain() {
	if r == nil || r.service == nil || r.service.runtime == nil || !r.hasRunningSubagentTask() {
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
	r.dispatchPostRoundWork()
	return false
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
	if subtype == "task_started" && !dmMetadataLooksLikeSubagentTask(metadata) {
		return
	}
	if subtype == "task_updated" && !dmIsTerminalSubagentTaskStatus(status) && !dmMetadataLooksLikeSubagentTask(metadata) {
		return
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	if r.subagentTasks == nil {
		r.subagentTasks = map[string]struct{}{}
	}
	switch subtype {
	case "task_started", "task_updated":
		if dmIsTerminalSubagentTaskStatus(status) {
			delete(r.subagentTasks, taskID)
			return
		}
		r.subagentTasks[taskID] = struct{}{}
	case "task_notification":
		if dmIsTerminalSubagentTaskStatus(status) {
			delete(r.subagentTasks, taskID)
		}
	}
}

func (r *roundRunner) hasRunningSubagentTask() bool {
	if r == nil {
		return false
	}
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return len(r.subagentTasks) > 0
}

func dmMetadataLooksLikeSubagentTask(metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	return strings.TrimSpace(dmAnyString(metadata["agent_id"])) != "" ||
		strings.TrimSpace(dmAnyString(metadata["agent_type"])) != "" ||
		strings.TrimSpace(dmAnyString(metadata["task_type"])) == "local_agent"
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
