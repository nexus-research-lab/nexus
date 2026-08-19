package protocol

import "strings"

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
// 只有不含 Nexus 产品投影块的 assistant 正文快照属于 transcript 原生消息；
// result / system / task_progress 以及资源卡片等都需要由 Nexus overlay 补齐。
func IsTranscriptNativeMessage(message Message) bool {
	if MessageRole(message) != "assistant" {
		return false
	}
	return !messageHasContentBlockType(message, ContentBlockTypeNexusResourceArtifact)
}

func messageHasContentBlockType(message Message, blockType string) bool {
	wanted := strings.TrimSpace(blockType)
	if wanted == "" {
		return false
	}
	switch content := message["content"].(type) {
	case []map[string]any:
		for _, block := range content {
			if value, _ := block["type"].(string); strings.TrimSpace(value) == wanted {
				return true
			}
		}
	case []any:
		for _, item := range content {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if value, _ := block["type"].(string); strings.TrimSpace(value) == wanted {
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
