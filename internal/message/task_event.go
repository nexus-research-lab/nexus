package message

import (
	"fmt"
	"math"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const shellToolProgressThrottleSeconds = 30

func (p *Processor) processTaskProgressMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.TaskProgress == nil {
		return nil
	}
	progress := message.TaskProgress
	toolName := strings.TrimSpace(progress.LastToolName)
	description := firstNonEmpty(progress.Summary, progress.Description)
	if description == "" && toolName != "" {
		description = toolName + " 正在执行"
	}
	return p.buildTaskProgressMessage(
		firstNonEmpty(progress.TaskID, progress.ToolUseID),
		firstNonEmpty(description, "后台任务正在执行"),
		strings.TrimSpace(progress.ToolUseID),
		toolName,
		taskUsageMap(progress.Usage),
		mergeTaskEventMetadata(progress.Additional, map[string]string{
			"agent_id":       progress.AgentID,
			"agent_type":     progress.AgentType,
			"description":    progress.Description,
			"last_tool_name": progress.LastToolName,
			"parent_task_id": progress.ParentTaskID,
			"summary":        progress.Summary,
		}),
	)
}

func (p *Processor) processToolProgressMessage(message sdkprotocol.ReceivedMessage) (*protocol.Message, bool) {
	if message.ToolProgress == nil {
		return nil, false
	}
	progress := message.ToolProgress
	data := mapValue(progress.Additional["data"])
	progressType := normalizeString(data["type"])
	if shellType := resolveShellProgressType(progressType, progress.ToolName); shellType != "" {
		trackingID := firstNonEmpty(
			normalizePointerString(progress.ParentToolUseID),
			strings.TrimSpace(progress.ToolUseID),
		)
		if trackingID == "" || !p.shouldEmitShellToolProgress(trackingID, progress.ElapsedTimeSeconds) {
			return nil, true
		}
		toolName := firstNonEmpty(strings.TrimSpace(progress.ToolName), shellProgressToolName(shellType))
		elapsedSeconds := int(math.Max(0, progress.ElapsedTimeSeconds))
		description := fmt.Sprintf("%s 已运行 %d 秒", toolName, elapsedSeconds)
		return p.buildEphemeralTaskProgressMessage(
			firstNonEmpty(strings.TrimSpace(progress.TaskID), "tool_progress_"+trackingID),
			description,
			trackingID,
			toolName,
			map[string]any{"duration_ms": elapsedSeconds * 1000},
			mergeTaskEventMetadata(data, map[string]string{
				"progress_type": shellType,
			}),
		), true
	}
	if progressType != "agent_progress" {
		return nil, false
	}
	taskID := firstNonEmpty(
		strings.TrimSpace(progress.TaskID),
		normalizeString(data["agent_id"]),
		strings.TrimSpace(progress.ToolUseID),
	)
	description := firstNonEmpty(
		normalizeString(data["description"]),
		normalizeString(data["agent_type"]),
		"子 Agent 正在执行",
	)
	return p.buildTaskProgressMessage(
		taskID,
		description,
		firstNonEmpty(normalizePointerString(progress.ParentToolUseID), strings.TrimSpace(progress.ToolUseID)),
		firstNonEmpty(agentProgressLastToolName(data), strings.TrimSpace(progress.ToolName)),
		mapValue(data["usage"]),
		data,
	), false
}

func resolveShellProgressType(progressType string, toolName string) string {
	switch progressType {
	case "bash_progress", "powershell_progress":
		return progressType
	}
	switch {
	case strings.EqualFold(strings.TrimSpace(toolName), "Bash"):
		return "bash_progress"
	case strings.EqualFold(strings.TrimSpace(toolName), "PowerShell"):
		return "powershell_progress"
	default:
		return ""
	}
}

func (p *Processor) shouldEmitShellToolProgress(trackingID string, elapsedSeconds float64) bool {
	lastElapsed, exists := p.toolProgressElapsed[trackingID]
	if exists && elapsedSeconds-lastElapsed < shellToolProgressThrottleSeconds {
		return false
	}
	p.toolProgressElapsed[trackingID] = elapsedSeconds
	return true
}

func shellProgressToolName(progressType string) string {
	if progressType == "powershell_progress" {
		return "PowerShell"
	}
	return "Bash"
}

func (p *Processor) processTaskStartedMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.TaskStarted == nil {
		return nil
	}
	started := message.TaskStarted
	return p.buildTaskStartedMessage(
		firstNonEmpty(started.TaskID, started.ToolUseID),
		firstNonEmpty(started.Description, started.Prompt, "任务已开始"),
		strings.TrimSpace(started.TaskType),
		strings.TrimSpace(started.ToolUseID),
		mergeTaskEventMetadata(started.Additional, map[string]string{
			"agent_id":       started.AgentID,
			"agent_type":     started.AgentType,
			"description":    started.Description,
			"output_file":    started.OutputFile,
			"parent_task_id": started.ParentTaskID,
			"prompt":         started.Prompt,
			"task_type":      started.TaskType,
			"workflow_name":  started.WorkflowName,
		}),
	)
}

func (p *Processor) processTaskNotificationMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.TaskNotification == nil {
		return nil
	}
	notification := message.TaskNotification
	return p.buildTaskNotificationMessage(
		firstNonEmpty(notification.TaskID, notification.ToolUseID),
		firstNonEmpty(notification.Summary, taskNotificationDefaultContent(notification.Status)),
		strings.TrimSpace(notification.ToolUseID),
		strings.TrimSpace(notification.Status),
		strings.TrimSpace(notification.OutputFile),
		taskUsageMap(notification.Usage),
		mergeTaskEventMetadata(notification.Additional, map[string]string{
			"agent_id":        notification.AgentID,
			"agent_type":      notification.AgentType,
			"output_file":     notification.OutputFile,
			"parent_task_id":  notification.ParentTaskID,
			"summary":         notification.Summary,
			"transcript_path": notification.TranscriptPath,
		}),
	)
}

func (p *Processor) buildTaskUpdatedMessage(taskID string, status string, patch map[string]any, additional map[string]any) *protocol.Message {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	status = strings.TrimSpace(status)
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		fmt.Sprintf("system_task_updated_%s_%s_%s", p.ctx.RoundID, strings.TrimSpace(taskID), firstNonEmpty(status, "patch")),
		"system",
	)
	payload["content"] = taskUpdatedContent(status)
	payload["metadata"] = map[string]any{
		"subtype": "task_updated",
		"task_id": strings.TrimSpace(taskID),
		"status":  emptyToNil(status),
		"patch":   firstNonNilMap(patch, map[string]any{}),
	}
	copyTaskEventMetadata(payload["metadata"].(map[string]any), additional)
	messageValue := protocol.Message(payload)
	return &messageValue
}

func (p *Processor) buildTaskStartedMessage(taskID string, content string, taskType string, toolUseID string, additional map[string]any) *protocol.Message {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		fmt.Sprintf("system_task_started_%s_%s", p.ctx.RoundID, strings.TrimSpace(taskID)),
		"system",
	)
	payload["content"] = firstNonEmpty(content, "任务已开始")
	payload["metadata"] = map[string]any{
		"subtype":     "task_started",
		"task_id":     strings.TrimSpace(taskID),
		"task_type":   emptyToNil(taskType),
		"tool_use_id": emptyToNil(toolUseID),
	}
	copyTaskEventMetadata(payload["metadata"].(map[string]any), additional)
	messageValue := protocol.Message(payload)
	return &messageValue
}

func (p *Processor) buildTaskProgressMessage(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) *protocol.Message {
	if !p.appendTaskProgress(taskID, description, toolUseID, lastToolName, usage, additional) {
		return nil
	}
	return p.buildAssistantDurableMessage(false, false, p.parentToolUseID)
}

func (p *Processor) buildEphemeralTaskProgressMessage(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) *protocol.Message {
	progress := newTaskProgressBlock(taskID, description, toolUseID, lastToolName, usage, additional)
	if progress == nil {
		return nil
	}
	// Shell 进度只构造运行态快照，不写回主 segment；否则下一条最终
	// assistant 会把临时进度块带进 transcript。
	temporary := p.segment
	temporary.content = cloneBlockSlice(p.segment.content)
	temporary.usage = cloneMap(p.segment.usage)
	temporary.AppendTaskProgress(progress)
	payload := protocol.Message(temporary.BuildAssistantMessage(p.ctx, p.sessionID, false))
	delete(payload, "stop_reason")
	payload["is_complete"] = false
	if parentID := strings.TrimSpace(p.parentToolUseID); parentID != "" {
		payload["parent_id"] = parentID
		payload["parent_tool_use_id"] = parentID
	}
	return &payload
}

func (p *Processor) appendTaskProgress(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) bool {
	progress := newTaskProgressBlock(taskID, description, toolUseID, lastToolName, usage, additional)
	if progress == nil {
		return false
	}
	p.segment.AppendTaskProgress(progress)
	return true
}

func newTaskProgressBlock(
	taskID string,
	description string,
	toolUseID string,
	lastToolName string,
	usage map[string]any,
	additional map[string]any,
) map[string]any {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	progress := map[string]any{
		"type":           "task_progress",
		"task_id":        taskID,
		"description":    description,
		"tool_use_id":    emptyToNil(toolUseID),
		"last_tool_name": emptyToNil(lastToolName),
		"usage":          firstNonNilMap(usage, map[string]any{}),
	}
	copyTaskEventMetadata(progress, additional)
	return progress
}

func (p *Processor) buildTaskNotificationMessage(taskID string, content string, toolUseID string, status string, outputFile string, usage map[string]any, additional map[string]any) *protocol.Message {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		fmt.Sprintf("system_task_notification_%s_%s", p.ctx.RoundID, strings.TrimSpace(taskID)),
		"system",
	)
	payload["content"] = firstNonEmpty(content, "任务状态已更新")
	payload["metadata"] = map[string]any{
		"subtype":     "task_notification",
		"task_id":     strings.TrimSpace(taskID),
		"tool_use_id": emptyToNil(toolUseID),
		"status":      emptyToNil(status),
		"output_file": emptyToNil(outputFile),
		"usage":       firstNonNilMap(usage, map[string]any{}),
	}
	copyTaskEventMetadata(payload["metadata"].(map[string]any), additional)
	messageValue := protocol.Message(payload)
	return &messageValue
}

func copyTaskEventMetadata(metadata map[string]any, additional map[string]any) {
	for _, key := range []string{
		"agent_id", "agent_type", "child_session_id", "description", "last_tool_name", "model", "name",
		"output_file", "parent_task_id", "prompt", "summary", "task_type", "team_name",
		"transcript_path", "workflow_name", "progress_type",
	} {
		if value := normalizeString(additional[key]); value != "" {
			metadata[key] = value
		}
	}
}

func mergeTaskEventMetadata(additional map[string]any, fields map[string]string) map[string]any {
	metadata := cloneMap(additional)
	if metadata == nil {
		metadata = map[string]any{}
	}
	for key, value := range fields {
		if normalized := strings.TrimSpace(value); normalized != "" {
			metadata[key] = normalized
		}
	}
	return metadata
}

func firstTaskProgressTaskID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskProgress.TaskID)
}

func firstTaskProgressDescription(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return firstNonEmpty(message.TaskProgress.Summary, message.TaskProgress.Description)
}

func firstTaskProgressToolUseID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskProgress.ToolUseID)
}

func firstTaskProgressToolName(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskProgress == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskProgress.LastToolName)
}

func firstTaskProgressUsage(message *sdkprotocol.SystemMessage) map[string]any {
	if message == nil || message.TaskProgress == nil {
		return nil
	}
	return taskUsageMap(message.TaskProgress.Usage)
}

func taskUsageMap(usage sdkprotocol.TaskUsage) map[string]any {
	values := map[string]any{}
	if usage.TotalTokens > 0 {
		values["total_tokens"] = usage.TotalTokens
	}
	if usage.ToolUses > 0 {
		values["tool_uses"] = usage.ToolUses
	}
	if usage.DurationMS > 0 {
		values["duration_ms"] = usage.DurationMS
	}
	return values
}

func agentProgressLastToolName(data map[string]any) string {
	message := mapValue(data["message"])
	if normalizeString(message["type"]) != "assistant" {
		return ""
	}
	envelope := mapValue(message["message"])
	items, ok := envelope["content"].([]any)
	if !ok {
		return ""
	}
	for index := len(items) - 1; index >= 0; index-- {
		block := mapValue(items[index])
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		if name := normalizeString(block["name"]); name != "" {
			return name
		}
	}
	return ""
}

func firstTaskStartedDescription(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return firstNonEmpty(message.TaskStarted.Description, message.TaskStarted.Prompt)
}

func firstTaskStartedTaskID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskStarted.TaskID)
}

func firstTaskStartedTaskType(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskStarted.TaskType)
}

func firstTaskStartedToolUseID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskStarted.ToolUseID)
}

func firstTaskNotificationTaskID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskNotification == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskNotification.TaskID)
}

func firstTaskNotificationToolUseID(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskNotification == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskNotification.ToolUseID)
}

func firstTaskNotificationStatus(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskNotification == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskNotification.Status)
}

func firstTaskNotificationSummary(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskNotification == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskNotification.Summary)
}

func firstTaskNotificationOutputFile(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskNotification == nil {
		return ""
	}
	return strings.TrimSpace(message.TaskNotification.OutputFile)
}

func firstTaskNotificationUsage(message *sdkprotocol.SystemMessage) map[string]any {
	if message == nil || message.TaskNotification == nil {
		return nil
	}
	return taskUsageMap(message.TaskNotification.Usage)
}

func taskNotificationDefaultContent(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "success", "done":
		return "任务已完成"
	case "stopped", "cancelled", "canceled", "killed", "interrupted":
		return "任务已停止"
	case "failed", "error":
		return "任务执行失败"
	default:
		return "任务状态已更新"
	}
}
