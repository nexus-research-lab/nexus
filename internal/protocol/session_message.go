// INPUT: runtime/历史层产出的会话消息。
// OUTPUT: 跨层共享的消息模型、身份读取与 transcript 原生性判定。
// POS: 会话消息协议及其持久化来源边界。
package protocol

import "strings"

const GoalCompletionReceiptField = "goal_completion_receipt"

// GoalCompletionReceipt 是宿主附着到完成回复的内部结算收据。
// GoalID/RoundID 只用于精确归属和历史合并；展示层不得直接呈现它们。
// 指针字段区分“权威零值”和“尚不可得”，未知项必须省略。
type GoalCompletionReceipt struct {
	GoalID          string `json:"goal_id"`
	RoundID         string `json:"round_id"`
	TimeUsedSeconds *int64 `json:"time_used_seconds,omitempty"`
	ActualTokens    *int64 `json:"actual_tokens,omitempty"`
}

// Equal 比较收据的稳定身份与可展示结算字段。
func (r GoalCompletionReceipt) Equal(other GoalCompletionReceipt) bool {
	return strings.TrimSpace(r.GoalID) == strings.TrimSpace(other.GoalID) &&
		strings.TrimSpace(r.RoundID) == strings.TrimSpace(other.RoundID) &&
		equalOptionalInt64(r.TimeUsedSeconds, other.TimeUsedSeconds) &&
		equalOptionalInt64(r.ActualTokens, other.ActualTokens)
}

// GoalCompletionReceiptFromAny 解码 durable JSON 或进程内强类型收据。
func GoalCompletionReceiptFromAny(value any) (GoalCompletionReceipt, bool) {
	switch typed := value.(type) {
	case GoalCompletionReceipt:
		return typed, strings.TrimSpace(typed.GoalID) != "" && strings.TrimSpace(typed.RoundID) != ""
	case *GoalCompletionReceipt:
		if typed == nil {
			return GoalCompletionReceipt{}, false
		}
		return *typed, strings.TrimSpace(typed.GoalID) != "" && strings.TrimSpace(typed.RoundID) != ""
	case map[string]any:
		receipt := GoalCompletionReceipt{
			GoalID:  strings.TrimSpace(stringValueFromAny(typed["goal_id"])),
			RoundID: strings.TrimSpace(stringValueFromAny(typed["round_id"])),
		}
		if _, ok := typed["time_used_seconds"]; ok {
			seconds := max(Int64FromAny(typed["time_used_seconds"]), 0)
			receipt.TimeUsedSeconds = &seconds
		}
		if _, ok := typed["actual_tokens"]; ok {
			tokens := max(Int64FromAny(typed["actual_tokens"]), 0)
			receipt.ActualTokens = &tokens
		}
		return receipt, receipt.GoalID != "" && receipt.RoundID != ""
	default:
		return GoalCompletionReceipt{}, false
	}
}

func stringValueFromAny(value any) string {
	text, _ := value.(string)
	return text
}

func equalOptionalInt64(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// Message 表示历史消息行。
type Message map[string]any

// Clone 复制消息，避免不同层直接共享 map 引用。
func Clone(message Message) Message {
	if len(message) == 0 {
		return Message{}
	}
	cloned := make(Message, len(message))
	for key, value := range message {
		cloned[key] = value
	}
	return cloned
}

// MessageRole 返回消息角色。
func MessageRole(message Message) string {
	if len(message) == 0 {
		return ""
	}
	value, _ := message["role"].(string)
	return strings.TrimSpace(value)
}

// MessageRoundID 返回消息所属 round_id。
func MessageRoundID(message Message) string {
	if len(message) == 0 {
		return ""
	}
	value, _ := message["round_id"].(string)
	return strings.TrimSpace(value)
}

// IsTranscriptNativeMessage 表示该 durable message 是否属于 cc transcript 原生真相。
// 普通 assistant 正文快照属于 transcript 原生消息；Nexus 注入到 assistant
// 快照里的 task_progress 不会被 runtime transcript 保存，必须进入 overlay。
// result / system 等同样需要由 Nexus overlay 补齐。
func IsTranscriptNativeMessage(message Message) bool {
	return MessageRole(message) == "assistant" && !messageHasContentBlockType(message, "task_progress")
}

func messageHasContentBlockType(message Message, blockType string) bool {
	switch content := message["content"].(type) {
	case []any:
		for _, item := range content {
			block, _ := item.(map[string]any)
			if value, _ := block["type"].(string); strings.EqualFold(strings.TrimSpace(value), blockType) {
				return true
			}
		}
	case []map[string]any:
		for _, block := range content {
			if value, _ := block["type"].(string); strings.EqualFold(strings.TrimSpace(value), blockType) {
				return true
			}
		}
	}
	return false
}

// MessagePage 表示按 round 分页的消息历史结果。
type MessagePage struct {
	Items                    []Message `json:"items"`
	HasMore                  bool      `json:"has_more"`
	NextBeforeRoundID        *string   `json:"next_before_round_id,omitempty"`
	NextBeforeRoundTimestamp *int64    `json:"next_before_round_timestamp,omitempty"`
}

// SessionRoundIndex 表示 session 级 round 导航索引。
type SessionRoundIndex struct {
	Items []SessionRoundIndexItem `json:"items"`
}

// SessionRoundIndexItem 表示一个 round 的轻量导航元数据。
type SessionRoundIndexItem struct {
	RoundID        string   `json:"round_id"`
	Title          string   `json:"title,omitempty"`
	Timestamp      int64    `json:"timestamp,omitempty"`
	Status         string   `json:"status,omitempty"`
	DurationMS     *int64   `json:"duration_ms,omitempty"`
	IsLive         bool     `json:"is_live,omitempty"`
	HasUserMessage bool     `json:"has_user_message,omitempty"`
	AgentIDs       []string `json:"agent_ids,omitempty"`
}
