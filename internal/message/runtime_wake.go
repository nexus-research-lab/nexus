// INPUT: runtime 空闲时自行开启新 turn 的触发证据（SDK task_notification 或 transcript <task-notification> 用户条目）。
// OUTPUT: live 与历史共用的唤醒触发描述、round marker 元数据与用户行装饰。
// POS: runtime 自唤醒 round 的唯一身份规则；live 和 transcript 投影都从同一后台任务摘要派生内容，保证对齐。
package message

import (
	"maps"
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	// RuntimeWakePurpose 标记 runtime 在上一轮结束后自行续跑（例如后台命令完成）的 round。
	RuntimeWakePurpose = "runtime_wake"
	// RuntimeWakeInputKind 写入用户行 metadata.input_kind，前端据此渲染触发分隔条而非用户气泡。
	RuntimeWakeInputKind = "runtime_wake"

	runtimeWakeTriggerBackgroundTask = "background_task"
	runtimeWakeTriggerRuntime        = "runtime"
	taskNotificationOpenTag          = "<task-notification>"
	taskNotificationCloseTag         = "</task-notification>"
)

// RuntimeWakeTrigger 描述唤醒本轮的后台任务；字段全部来自 runtime 原文，不做翻译。
type RuntimeWakeTrigger struct {
	TaskID  string
	Status  string
	Summary string
}

// Content 是该轮 round marker 与 transcript 用户行共用的对齐文本。
func (t RuntimeWakeTrigger) Content() string {
	if t.Summary != "" {
		return t.Summary
	}
	if t.kind() == runtimeWakeTriggerRuntime {
		return "Agent resumed on its own"
	}
	return "Background task " + textutil.FirstNonEmpty(t.Status, "finished")
}

// kind 区分后台任务唤醒与没有任务证据的其它自唤醒（如定时唤醒）。
func (t RuntimeWakeTrigger) kind() string {
	if t.TaskID == "" && t.Summary == "" && t.Status == "" {
		return runtimeWakeTriggerRuntime
	}
	return runtimeWakeTriggerBackgroundTask
}

// Metadata 返回写入 round marker / 用户行的展示元数据。
func (t RuntimeWakeTrigger) Metadata() map[string]string {
	metadata := map[string]string{
		"input_kind":   RuntimeWakeInputKind,
		"trigger_kind": t.kind(),
	}
	if t.TaskID != "" {
		metadata["task_id"] = t.TaskID
	}
	if t.Status != "" {
		metadata["task_status"] = t.Status
	}
	return metadata
}

// DecorateRuntimeWakeUserMessage 把用户行标记为 runtime 触发：不算用户输入，也不展示为用户气泡。
func DecorateRuntimeWakeUserMessage(row protocol.Message, trigger RuntimeWakeTrigger) protocol.Message {
	if row == nil {
		return row
	}
	metadata := map[string]any{}
	if existing, ok := row["metadata"].(map[string]any); ok {
		maps.Copy(metadata, existing)
	}
	if existing, ok := row["metadata"].(map[string]string); ok {
		for key, value := range existing {
			metadata[key] = value
		}
	}
	for key, value := range trigger.Metadata() {
		metadata[key] = value
	}
	row["metadata"] = metadata
	row["purpose"] = RuntimeWakePurpose
	row["is_synthetic"] = true
	// 内部 round 默认隐藏用户行；自唤醒触发行要作为分隔条展示。
	delete(row, "hidden_from_user")
	row["content"] = trigger.Content()
	return row
}

// IsRuntimeTurnStart 判断空闲期间 runtime 是否自行开启了新的顶层 turn。
// Claude Code 与 nxs 每个 turn 以 system/init 开头；子任务消息带 parent_tool_use_id，不算。
func IsRuntimeTurnStart(incoming sdkprotocol.ReceivedMessage) bool {
	if incoming.Type != sdkprotocol.MessageTypeSystem || incoming.ParentToolUseID != nil {
		return false
	}
	subtype := strings.TrimSpace(incoming.Subtype)
	if subtype == "" && incoming.System != nil {
		subtype = strings.TrimSpace(incoming.System.Subtype)
	}
	return subtype == "init"
}

// RuntimeWakeTriggerFromTaskNotification 从顶层 task_notification 提取唤醒说明；子任务内部通知不算。
func RuntimeWakeTriggerFromTaskNotification(incoming sdkprotocol.ReceivedMessage) (RuntimeWakeTrigger, bool) {
	notification := incoming.TaskNotification
	if notification == nil && incoming.System != nil {
		notification = incoming.System.TaskNotification
	}
	if notification == nil || incoming.ParentToolUseID != nil {
		return RuntimeWakeTrigger{}, false
	}
	return RuntimeWakeTrigger{
		TaskID:  strings.TrimSpace(notification.TaskID),
		Status:  strings.TrimSpace(notification.Status),
		Summary: strings.TrimSpace(notification.Summary),
	}, true
}

// IsRuntimeWakeMetadata 判断 marker metadata 是否声明了 runtime 唤醒。
func IsRuntimeWakeMetadata(metadata map[string]string) bool {
	return strings.TrimSpace(metadata["input_kind"]) == RuntimeWakeInputKind
}

// RuntimeWakeTriggerFromMetadata 从 marker metadata 与对齐内容还原触发描述。
func RuntimeWakeTriggerFromMetadata(content string, metadata map[string]string) RuntimeWakeTrigger {
	return RuntimeWakeTrigger{
		TaskID:  strings.TrimSpace(metadata["task_id"]),
		Status:  strings.TrimSpace(metadata["task_status"]),
		Summary: strings.TrimSpace(content),
	}
}

// ParseTaskNotificationPrompt 解析 Claude Code 注入的 <task-notification> 用户输入。
// 多个通知合并成一条输入时取最后一个，与 live 侧“最近一次 task_notification”保持一致。
func ParseTaskNotificationPrompt(content string) (RuntimeWakeTrigger, bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, taskNotificationOpenTag) {
		return RuntimeWakeTrigger{}, false
	}
	start := strings.LastIndex(trimmed, taskNotificationOpenTag)
	block := trimmed[start+len(taskNotificationOpenTag):]
	if end := strings.Index(block, taskNotificationCloseTag); end >= 0 {
		block = block[:end]
	}
	return RuntimeWakeTrigger{
		TaskID:  taggedValue(block, "task-id"),
		Status:  taggedValue(block, "status"),
		Summary: taggedValue(block, "summary"),
	}, true
}

func taggedValue(content string, tag string) string {
	openTag := "<" + tag + ">"
	start := strings.Index(content, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(content[start:], "</"+tag+">")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(content[start : start+end])
}
