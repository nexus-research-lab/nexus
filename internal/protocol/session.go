package protocol

import "time"

// ContextUsageData 表示 runtime 在一轮结束后确认的上下文占用快照。
type ContextUsageData struct {
	TotalTokens int     `json:"total_tokens"`
	MaxTokens   int     `json:"max_tokens"`
	Percentage  float64 `json:"percentage"`
	Model       string  `json:"model,omitempty"`
}

// Session 表示对外暴露的统一会话模型。
type Session struct {
	SessionKey     string            `json:"session_key"`
	AgentID        string            `json:"agent_id"`
	SessionID      *string           `json:"session_id"`
	RoomSessionID  *string           `json:"room_session_id"`
	RoomID         *string           `json:"room_id"`
	ConversationID *string           `json:"conversation_id"`
	ChannelType    string            `json:"channel_type"`
	ChatType       string            `json:"chat_type"`
	Status         string            `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	LastActivity   time.Time         `json:"last_activity"`
	Title          string            `json:"title"`
	MessageCount   int               `json:"message_count"`
	Options        map[string]any    `json:"options"`
	ContextUsage   *ContextUsageData `json:"context_usage,omitempty"`
	IsActive       bool              `json:"is_active"`
}
