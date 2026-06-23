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
		content = firstNonEmpty(
			normalizeString(message.Data["description"]),
			normalizeString(message.Data["prompt"]),
			firstTaskStartedDescription(message),
			"任务已开始",
		)
		metadata = map[string]any{
			"subtype":     "task_started",
			"task_id":     firstNonEmpty(normalizeString(message.Data["task_id"]), firstTaskStartedTaskID(message)),
			"task_type":   firstNonEmpty(normalizeString(message.Data["task_type"]), firstTaskStartedTaskType(message)),
			"tool_use_id": firstNonEmpty(normalizeString(message.Data["tool_use_id"]), firstTaskStartedToolUseID(message)),
		}
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
