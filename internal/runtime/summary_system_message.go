package runtime

import (
	"fmt"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// SystemMessageSummary 表示系统消息面向前端的统一摘要。
type SystemMessageSummary struct {
	Subtype  string
	Content  string
	Metadata map[string]any
}

// SummarizeSystemMessage 把 SDK system 消息收口成前端可消费的系统事件。
func SummarizeSystemMessage(message *sdkprotocol.SystemMessage) (SystemMessageSummary, bool) {
	if message == nil {
		return SystemMessageSummary{}, false
	}

	switch strings.TrimSpace(message.Subtype) {
	case "task_started":
		if message.TaskStarted == nil {
			return SystemMessageSummary{}, false
		}
		content := firstNonEmptySummary(
			message.TaskStarted.Description,
			message.TaskStarted.Prompt,
			"任务已开始",
		)
		return SystemMessageSummary{
			Subtype: "task_started",
			Content: content,
			Metadata: compactMetadata(map[string]any{
				"subtype":       "task_started",
				"task_id":       strings.TrimSpace(message.TaskStarted.TaskID),
				"tool_use_id":   strings.TrimSpace(message.TaskStarted.ToolUseID),
				"task_type":     strings.TrimSpace(message.TaskStarted.TaskType),
				"workflow_name": strings.TrimSpace(message.TaskStarted.WorkflowName),
			}),
		}, true
	case "task_progress":
		if message.TaskProgress == nil {
			return SystemMessageSummary{}, false
		}
		content := firstNonEmptySummary(
			message.TaskProgress.Summary,
			message.TaskProgress.Description,
			formatTaskProgressContent(message.TaskProgress.LastToolName),
			"任务正在执行",
		)
		return SystemMessageSummary{
			Subtype: "task_progress",
			Content: content,
			Metadata: compactMetadata(map[string]any{
				"subtype":        "task_progress",
				"task_id":        strings.TrimSpace(message.TaskProgress.TaskID),
				"tool_use_id":    strings.TrimSpace(message.TaskProgress.ToolUseID),
				"last_tool_name": strings.TrimSpace(message.TaskProgress.LastToolName),
				"usage":          summarizeTaskProgressUsage(message.TaskProgress.Usage),
			}),
		}, true
	case "task_notification":
		if message.TaskNotification == nil {
			return SystemMessageSummary{}, false
		}
		content := firstNonEmptySummary(
			message.TaskNotification.Summary,
			formatTaskNotificationContent(message.TaskNotification.Status),
			"任务状态已更新",
		)
		return SystemMessageSummary{
			Subtype: "task_notification",
			Content: content,
			Metadata: compactMetadata(map[string]any{
				"subtype":     "task_notification",
				"task_id":     strings.TrimSpace(message.TaskNotification.TaskID),
				"tool_use_id": strings.TrimSpace(message.TaskNotification.ToolUseID),
				"status":      strings.TrimSpace(message.TaskNotification.Status),
				"output_file": strings.TrimSpace(message.TaskNotification.OutputFile),
				"usage":       summarizeTaskProgressUsage(message.TaskNotification.Usage),
			}),
		}, true
	case "status":
		if message.Status == nil {
			return SystemMessageSummary{}, false
		}
		status := strings.TrimSpace(message.Status.Status)
		if status == "" {
			return SystemMessageSummary{}, false
		}
		return SystemMessageSummary{
			Subtype: "status",
			Content: fmt.Sprintf("运行状态：%s", status),
			Metadata: compactMetadata(map[string]any{
				"subtype":         "status",
				"status":          status,
				"permission_mode": strings.TrimSpace(string(message.Status.PermissionMode)),
			}),
		}, true
	default:
		return SystemMessageSummary{}, false
	}
}

func formatTaskProgressContent(lastToolName string) string {
	toolName := strings.TrimSpace(lastToolName)
	if toolName == "" {
		return ""
	}
	return toolName + " 正在执行"
}

func formatTaskNotificationContent(status string) string {
	normalized := strings.TrimSpace(status)
	if normalized == "" {
		return ""
	}
	return "任务状态：" + normalized
}

func summarizeTaskProgressUsage(usage sdkprotocol.TaskProgressUsage) map[string]any {
	return compactMetadata(map[string]any{
		"total_tokens": usage.TotalTokens,
		"tool_uses":    usage.ToolUses,
		"duration_ms":  usage.DurationMS,
	})
}

func compactMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string]any, len(metadata))
	for key, value := range metadata {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
			result[key] = strings.TrimSpace(typed)
		case int:
			if typed == 0 {
				continue
			}
			result[key] = typed
		case map[string]any:
			if len(typed) == 0 {
				continue
			}
			result[key] = typed
		case nil:
			continue
		default:
			result[key] = typed
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func firstNonEmptySummary(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
