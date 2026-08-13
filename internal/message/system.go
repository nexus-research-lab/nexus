package message

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type systemMessageProjector func(*Processor, sdkprotocol.SystemMessage) *protocol.Message

type systemMessageDelivery uint8

const (
	systemMessageDurable systemMessageDelivery = iota
	systemMessageEphemeral
)

type systemMessageProjection struct {
	projector systemMessageProjector
	delivery  systemMessageDelivery
}

var systemMessageProjections = map[string]systemMessageProjection{
	"api_retry":         {projector: (*Processor).projectAPIRetrySystemMessage, delivery: systemMessageEphemeral},
	"compact_boundary":  {projector: (*Processor).projectCompactBoundarySystemMessage, delivery: systemMessageDurable},
	"memory_saved":      {projector: (*Processor).projectMemorySavedSystemMessage, delivery: systemMessageDurable},
	"task_notification": {projector: (*Processor).projectSystemTaskNotification, delivery: systemMessageDurable},
	"task_progress":     {projector: (*Processor).projectSystemTaskProgress, delivery: systemMessageDurable},
	"task_started":      {projector: (*Processor).projectSystemTaskStarted, delivery: systemMessageDurable},
	"task_updated":      {projector: (*Processor).projectSystemTaskUpdated, delivery: systemMessageDurable},
}

func (p *Processor) processSystemMessage(system sdkprotocol.SystemMessage) ([]protocol.Message, []protocol.Message) {
	projection, supported := systemMessageProjections[strings.TrimSpace(system.Subtype)]
	if !supported {
		return nil, nil
	}
	projected := projection.projector(p, system)
	if projected == nil {
		return nil, nil
	}
	if projection.delivery == systemMessageEphemeral {
		return nil, []protocol.Message{*projected}
	}
	return []protocol.Message{*projected}, nil
}

func (p *Processor) projectSystemTaskStarted(message sdkprotocol.SystemMessage) *protocol.Message {
	return p.projectTaskStarted(*message.TaskStarted)
}

func (p *Processor) projectSystemTaskProgress(message sdkprotocol.SystemMessage) *protocol.Message {
	return p.projectTaskProgress(*message.TaskProgress)
}

func (p *Processor) projectSystemTaskNotification(message sdkprotocol.SystemMessage) *protocol.Message {
	return p.projectTaskNotification(*message.TaskNotification)
}

func (p *Processor) projectSystemTaskUpdated(message sdkprotocol.SystemMessage) *protocol.Message {
	return p.projectTaskUpdated(*message.TaskUpdated)
}

func (p *Processor) projectMemorySavedSystemMessage(message sdkprotocol.SystemMessage) *protocol.Message {
	saved := *message.MemorySaved
	metadata := cloneMapOrEmpty(saved.Additional)
	metadata["subtype"] = "memory_saved"
	metadata["verb"] = strings.TrimSpace(saved.Verb)
	metadata["written_paths"] = append([]string(nil), saved.WrittenPaths...)
	return p.buildSystemEventMessage(
		"system_memory_saved_"+p.ctx.RoundID,
		memorySavedContent(saved.Verb),
		metadata,
	)
}

func (p *Processor) projectAPIRetrySystemMessage(message sdkprotocol.SystemMessage) *protocol.Message {
	metadata := normalizeAPIRetryMetadata(message.Data)
	return p.buildSystemEventMessage(
		"system_api_retry_"+p.ctx.RoundID,
		firstNonEmpty(normalizeString(metadata["message"]), apiRetryDefaultMessage(metadata)),
		metadata,
	)
}

func (p *Processor) projectCompactBoundarySystemMessage(message sdkprotocol.SystemMessage) *protocol.Message {
	return p.buildSystemEventMessage(
		"system_compact_boundary_"+p.ctx.RoundID,
		firstNonEmpty(normalizeString(message.Data["content"]), "上下文已压缩"),
		normalizeCompactBoundaryMetadata(message.Data),
	)
}

func (p *Processor) buildSystemEventMessage(messageID string, content string, metadata map[string]any) *protocol.Message {
	payload := baseMessageEnvelope(p.ctx, p.sessionID, messageID, "system")
	payload["content"] = content
	payload["metadata"] = metadata
	messageValue := protocol.Message(payload)
	return &messageValue
}

// projectRuntimeStatus 只接受 SDK 的公开状态集合；null 由空字符串表示并用于结束状态。
func projectRuntimeStatus(message *sdkprotocol.SystemMessage) (protocol.RuntimeStatus, bool) {
	if message == nil || strings.TrimSpace(message.Subtype) != "status" {
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
	metadata := cloneMapOrEmpty(data)
	metadata["subtype"] = "api_retry"
	for _, field := range []string{"attempt", "max_retries", "retry_delay_ms", "error_status"} {
		if value := normalizeInt(data[field]); value > 0 {
			metadata[field] = value
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
	metadata := cloneMapOrEmpty(data)
	metadata["subtype"] = "compact_boundary"
	if metadata["compact_metadata"] == nil {
		if compactMetadata := mapValue(data["compactMetadata"]); len(compactMetadata) > 0 {
			metadata["compact_metadata"] = compactMetadata
		}
	}
	return metadata
}
