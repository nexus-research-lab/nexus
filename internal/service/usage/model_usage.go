package usage

import "time"

// RecordInput 表示一次可计费 token 用量写入。
type RecordInput struct {
	OwnerUserID    string
	Source         string
	SessionKey     string
	MessageID      string
	RoundID        string
	AgentID        string
	RoomID         string
	ConversationID string
	Usage          map[string]any
	OccurredAt     time.Time
}

// Summary 表示用户级 token 用量汇总。
type Summary struct {
	InputTokens              int64  `json:"input_tokens"`
	OutputTokens             int64  `json:"output_tokens"`
	CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
	TotalTokens              int64  `json:"total_tokens"`
	QuotaLimitTokens         *int64 `json:"quota_limit_tokens"`
	SessionCount             int    `json:"session_count"`
	MessageCount             int    `json:"message_count"`
	UpdatedAt                string `json:"updated_at"`
}
