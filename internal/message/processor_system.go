package message

import (
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (p *Processor) processSystemMessage(message sdkprotocol.ReceivedMessage) ([]protocol.Message, []protocol.Message) {
	if message.System == nil {
		return nil, nil
	}
	subtype := strings.TrimSpace(message.System.Subtype)
	if subtype == "task_progress" {
		progressMessage := p.buildTaskProgressMessage(
			firstNonEmpty(
				normalizeString(message.System.Data["task_id"]),
				firstTaskProgressTaskID(message.System),
			),
			firstNonEmpty(
				normalizeString(message.System.Data["description"]),
				firstTaskProgressDescription(message.System),
			),
			firstNonEmpty(
				normalizeString(message.System.Data["tool_use_id"]),
				firstTaskProgressToolUseID(message.System),
			),
			firstNonEmpty(
				normalizeString(message.System.Data["last_tool_name"]),
				firstTaskProgressToolName(message.System),
			),
			firstNonNilMap(
				mapValue(message.System.Data["usage"]),
				firstTaskProgressUsage(message.System),
			),
		)
		if progressMessage == nil {
			return nil, nil
		}
		return []protocol.Message{*progressMessage}, nil
	}

	if visible, ephemeral := p.buildVisibleSystemMessage(message.System); visible != nil {
		if ephemeral {
			return nil, []protocol.Message{*visible}
		}
		return []protocol.Message{*visible}, nil
	}
	return nil, nil
}

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
	)
}

func (p *Processor) processToolProgressMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.ToolProgress == nil {
		return nil
	}
	progress := message.ToolProgress
	data := mapValue(progress.Additional["data"])
	if normalizeString(data["type"]) != "agent_progress" {
		return nil
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
	)
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
		started.Additional,
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
		notification.Additional,
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

func (p *Processor) buildTaskProgressMessage(taskID string, description string, toolUseID string, lastToolName string, usage map[string]any) *protocol.Message {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	p.segment.AppendTaskProgress(map[string]any{
		"type":           "task_progress",
		"task_id":        taskID,
		"description":    description,
		"tool_use_id":    emptyToNil(toolUseID),
		"last_tool_name": emptyToNil(lastToolName),
		"usage":          firstNonNilMap(usage, map[string]any{}),
	})
	return p.buildAssistantDurableMessage(false, false, "")
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
	for _, key := range []string{"agent_id", "agent_type", "description", "model", "name", "output_file", "parent_task_id", "team_name", "transcript_path"} {
		if value := normalizeString(additional[key]); value != "" {
			metadata[key] = value
		}
	}
}

func (p *Processor) buildVisibleSystemMessage(message *sdkprotocol.SystemMessage) (*protocol.Message, bool) {
	if message == nil {
		return nil, false
	}
	subtype := strings.TrimSpace(message.Subtype)
	var (
		content           string
		metadata          map[string]any
		explicitMessageID string
		ephemeral         bool
	)
	switch subtype {
	case "task_started":
		return p.buildTaskStartedMessage(
			firstNonEmpty(normalizeString(message.Data["task_id"]), firstTaskStartedTaskID(message)),
			firstNonEmpty(
				normalizeString(message.Data["description"]),
				normalizeString(message.Data["prompt"]),
				firstTaskStartedDescription(message),
				"任务已开始",
			),
			firstNonEmpty(normalizeString(message.Data["task_type"]), firstTaskStartedTaskType(message)),
			firstNonEmpty(normalizeString(message.Data["tool_use_id"]), firstTaskStartedToolUseID(message)),
			message.Data,
		), false
	case "task_notification":
		return p.buildTaskNotificationMessage(
			firstNonEmpty(normalizeString(message.Data["task_id"]), firstTaskNotificationTaskID(message)),
			firstNonEmpty(
				normalizeString(message.Data["summary"]),
				firstTaskNotificationSummary(message),
				taskNotificationDefaultContent(firstNonEmpty(normalizeString(message.Data["status"]), firstTaskNotificationStatus(message))),
			),
			firstNonEmpty(normalizeString(message.Data["tool_use_id"]), firstTaskNotificationToolUseID(message)),
			firstNonEmpty(normalizeString(message.Data["status"]), firstTaskNotificationStatus(message)),
			firstNonEmpty(normalizeString(message.Data["output_file"]), firstTaskNotificationOutputFile(message)),
			firstNonNilMap(mapValue(message.Data["usage"]), firstTaskNotificationUsage(message)),
			message.Data,
		), false
	case "task_updated":
		patch := mapValue(message.Data["patch"])
		return p.buildTaskUpdatedMessage(
			normalizeString(message.Data["task_id"]),
			normalizeString(patch["status"]),
			patch,
			message.Data,
		), false
	case "api_retry", "api_error":
		metadata = normalizeAPIRetryMetadata(message.Data)
		content = firstNonEmpty(normalizeString(metadata["message"]), apiRetryDefaultMessage(metadata))
		explicitMessageID = "system_api_retry_" + p.ctx.RoundID
		ephemeral = true
	case "compact_boundary":
		metadata = normalizeCompactBoundaryMetadata(message.Data)
		content = firstNonEmpty(normalizeString(message.Data["content"]), "上下文已压缩")
		explicitMessageID = "system_compact_boundary_" + p.ctx.RoundID
	default:
		return nil, false
	}
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(explicitMessageID, fmt.Sprintf("system_%s_%d", p.ctx.RoundID, time.Now().UnixMilli())),
		"system",
	)
	payload["content"] = content
	payload["metadata"] = metadata
	messageValue := protocol.Message(payload)
	return &messageValue, ephemeral
}

func taskUpdatedContent(status string) string {
	switch strings.TrimSpace(status) {
	case "running":
		return "后台子 Agent 正在运行"
	case "completed":
		return "后台子 Agent 已完成"
	case "failed", "error":
		return "后台子 Agent 执行失败"
	case "killed", "stopped", "cancelled":
		return "后台子 Agent 已停止"
	default:
		return "后台子 Agent 状态已更新"
	}
}

func normalizeAPIRetryMetadata(data map[string]any) map[string]any {
	metadata := cloneMap(data)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["subtype"] = "api_retry"
	setAPIRetryInt(metadata, data, "attempt", "attempt", "retryAttempt", "retry_attempt")
	setAPIRetryInt(metadata, data, "max_retries", "max_retries", "maxRetries")
	setAPIRetryInt(metadata, data, "retry_delay_ms", "retry_delay_ms", "retryInMs")
	setAPIRetryInt(metadata, data, "error_status", "error_status", "status")
	if normalizeInt(metadata["error_status"]) <= 0 {
		if status := normalizeInt(mapValue(data["error"])["status"]); status > 0 {
			metadata["error_status"] = status
		}
	}
	if rawError, ok := data["error"]; ok && rawError != nil {
		if _, isString := rawError.(string); !isString {
			metadata["raw_error"] = rawError
		}
		metadata["error"] = normalizeAPIRetryError(fmt.Sprint(rawError))
	}
	return metadata
}

func setAPIRetryInt(metadata map[string]any, data map[string]any, target string, keys ...string) {
	if normalizeInt(metadata[target]) > 0 {
		return
	}
	for _, key := range keys {
		if value := normalizeInt(data[key]); value > 0 {
			metadata[target] = value
			return
		}
	}
}

func apiRetryDefaultMessage(metadata map[string]any) string {
	if normalizeString(metadata["error"]) == "rate_limit" {
		return "模型请求暂时受限，正在自动重试。"
	}
	return "API 请求失败，正在自动重试。"
}

func normalizeAPIRetryError(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(normalized, "rate_limit"),
		strings.Contains(normalized, "rate limit"),
		strings.Contains(normalized, "overloaded_error"),
		strings.Contains(normalized, "529"),
		strings.Contains(normalized, "429"):
		return "rate_limit"
	case strings.Contains(normalized, "timeout") || strings.Contains(normalized, "timed out"):
		return "timeout"
	case strings.Contains(normalized, "connection") || strings.Contains(normalized, "connect"):
		return "connection"
	default:
		return firstNonEmpty(normalized, "api_error")
	}
}

func normalizeCompactBoundaryMetadata(data map[string]any) map[string]any {
	metadata := cloneMap(data)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["subtype"] = "compact_boundary"
	if metadata["compact_metadata"] == nil {
		if compactMetadata := mapValue(data["compactMetadata"]); len(compactMetadata) > 0 {
			metadata["compact_metadata"] = compactMetadata
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
