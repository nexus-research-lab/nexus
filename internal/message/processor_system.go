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

func (p *Processor) processToolProgressMessage(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.ToolProgress == nil {
		return nil
	}
	return p.buildTaskProgressMessage(
		firstNonEmpty(strings.TrimSpace(message.ToolProgress.TaskID), strings.TrimSpace(message.ToolProgress.ToolUseID)),
		firstNonEmpty(strings.TrimSpace(message.ToolProgress.ToolName)+" 正在执行", "后台任务正在执行"),
		strings.TrimSpace(message.ToolProgress.ToolUseID),
		strings.TrimSpace(message.ToolProgress.ToolName),
		nil,
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
	case "api_retry":
		content = firstNonEmpty(normalizeString(message.Data["message"]), "API 正在重试")
		metadata = cloneMap(message.Data)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["subtype"] = "api_retry"
		explicitMessageID = "system_api_retry_" + p.ctx.RoundID
		ephemeral = true
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
	return firstNonEmpty(strings.TrimSpace(message.TaskProgress.Summary), strings.TrimSpace(message.TaskProgress.Description))
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
	usage := map[string]any{}
	if message.TaskProgress.Usage.TotalTokens > 0 {
		usage["total_tokens"] = message.TaskProgress.Usage.TotalTokens
	}
	if message.TaskProgress.Usage.ToolUses > 0 {
		usage["tool_uses"] = message.TaskProgress.Usage.ToolUses
	}
	if message.TaskProgress.Usage.DurationMS > 0 {
		usage["duration_ms"] = message.TaskProgress.Usage.DurationMS
	}
	return usage
}

func firstTaskStartedDescription(message *sdkprotocol.SystemMessage) string {
	if message == nil || message.TaskStarted == nil {
		return ""
	}
	return firstNonEmpty(
		strings.TrimSpace(message.TaskStarted.Description),
		strings.TrimSpace(message.TaskStarted.Prompt),
	)
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
