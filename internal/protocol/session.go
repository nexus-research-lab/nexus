// INPUT: Agent workspace meta 或 Room SQL 投影中的会话身份、展示状态与资源版本。
// OUTPUT: 对外统一 Session 视图；workspace session 的 configuration_version 用于 CAS。
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

// ExternalSessionIdentity 是外部 IM Session 的可展示传输身份。
// AccountHint 与 LegacySessionHint 都是不可逆短指纹，不包含账号凭据、完整平台标识或原始 Session key。
type ExternalSessionIdentity struct {
	ChannelType        string `json:"channel_type"`
	AccountHint        string `json:"account_hint,omitempty"`
	LegacySessionHint  string `json:"legacy_session_hint,omitempty"`
	AccountStatus      string `json:"account_status,omitempty"`
	PeerName           string `json:"peer_name,omitempty"`
	PairingStatus      string `json:"pairing_status"`
	CurrentPairing     bool   `json:"current_pairing"`
	CanDelete          bool   `json:"can_delete"`
	TaskReferenceCount int    `json:"task_reference_count,omitempty"`
}

// Session 表示对外暴露的统一会话模型。
type Session struct {
	SessionKey           string                   `json:"session_key"`
	AgentID              string                   `json:"agent_id"`
	SessionID            *string                  `json:"session_id"`
	TranscriptSessionIDs []string                 `json:"transcript_session_ids,omitempty"`
	RoomSessionID        *string                  `json:"room_session_id"`
	RoomID               *string                  `json:"room_id"`
	ConversationID       *string                  `json:"conversation_id"`
	ChannelType          string                   `json:"channel_type"`
	ChatType             string                   `json:"chat_type"`
	Status               string                   `json:"status"`
	CreatedAt            time.Time                `json:"created_at"`
	LastActivity         time.Time                `json:"last_activity"`
	Title                string                   `json:"title"`
	MessageCount         int                      `json:"message_count"`
	Options              map[string]any           `json:"options"`
	ContextUsage         *ContextUsageData        `json:"context_usage,omitempty"`
	ExternalIdentity     *ExternalSessionIdentity `json:"external_identity,omitempty"`
	IsActive             bool                     `json:"is_active"`
	ConfigurationVersion int64                    `json:"configuration_version"`
}
