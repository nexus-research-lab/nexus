package protocol

import "time"

// Session 表示对外暴露的统一会话模型。
type Session struct {
	SessionKey     string         `json:"session_key"`
	AgentID        string         `json:"agent_id"`
	SessionID      *string        `json:"session_id"`
	RoomSessionID  *string        `json:"room_session_id"`
	RoomID         *string        `json:"room_id"`
	ConversationID *string        `json:"conversation_id"`
	ChannelType    string         `json:"channel_type"`
	ChatType       string         `json:"chat_type"`
	Status         string         `json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	LastActivity   time.Time      `json:"last_activity"`
	Title          string         `json:"title"`
	MessageCount   int            `json:"message_count"`
	Options        map[string]any `json:"options"`
	IsActive       bool           `json:"is_active"`
}
