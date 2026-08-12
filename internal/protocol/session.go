// INPUT: Agent workspace runtime meta 与 Room SQL 身份、标题、配置投影。
// OUTPUT: 按字段所有权单调合并的 Session 视图；workspace session 的 configuration_version 用于 CAS。
// POS: Session HTTP、目录与 nexus_config 之间的协议模型；Room 生命周期版本仍归 rooms 域。
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
	SessionKey           string            `json:"session_key"`
	AgentID              string            `json:"agent_id"`
	SessionID            *string           `json:"session_id"`
	TranscriptSessionIDs []string          `json:"transcript_session_ids,omitempty"`
	RoomSessionID        *string           `json:"room_session_id"`
	RoomID               *string           `json:"room_id"`
	ConversationID       *string           `json:"conversation_id"`
	ChannelType          string            `json:"channel_type"`
	ChatType             string            `json:"chat_type"`
	Status               string            `json:"status"`
	CreatedAt            time.Time         `json:"created_at"`
	LastActivity         time.Time         `json:"last_activity"`
	Title                string            `json:"title"`
	MessageCount         int               `json:"message_count"`
	Options              map[string]any    `json:"options"`
	ContextUsage         *ContextUsageData `json:"context_usage,omitempty"`
	IsActive             bool              `json:"is_active"`
	ConfigurationVersion int64             `json:"configuration_version"`
}
