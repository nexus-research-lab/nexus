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
			message.System.Data,
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
	case "memory_saved":
		if message.MemorySaved == nil {
			return nil, false
		}
		metadata = cloneMap(message.MemorySaved.Additional)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["subtype"] = "memory_saved"
		metadata["verb"] = strings.TrimSpace(message.MemorySaved.Verb)
		metadata["written_paths"] = append([]string(nil), message.MemorySaved.WrittenPaths...)
		content = memorySavedContent(message.MemorySaved.Verb)
		explicitMessageID = "system_memory_saved_" + p.ctx.RoundID
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

// projectRuntimeStatus 只接受 SDK 的公开状态集合；null 由空字符串表示并用于结束状态。
func projectRuntimeStatus(message *sdkprotocol.SystemMessage) (protocol.RuntimeStatus, bool) {
	if message == nil || strings.TrimSpace(message.Subtype) != "status" || message.Status == nil {
		return "", false
	}
	status := protocol.RuntimeStatus(strings.TrimSpace(message.Status.Status))
	switch status {
	case protocol.RuntimeStatusCompacting, "":
		return status, true
	default:
		return "", false
	}
}

// memorySavedContent 把 runtime 动词收口成稳定的产品文案。
func memorySavedContent(verb string) string {
	if strings.EqualFold(strings.TrimSpace(verb), "Improved") {
		return "长期记忆已整理"
	}
	return "长期记忆已保存"
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
