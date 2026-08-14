package usage

import "time"

// RecordInput 表示一次可计费 token 用量写入。
type RecordInput struct {
	OwnerUserID      string
	Source           string
	SessionKey       string
	MessageID        string
	RoundID          string
	AgentID          string
	RoomID           string
	ConversationID   string
	CacheAttribution CacheAttribution
	Usage            map[string]any
	OccurredAt       time.Time
}

// CacheSegment 聚合同一低敏 cache correlation surface 的 provider usage。
type CacheSegment struct {
	Source                   string
	CacheAttribution         CacheAttribution
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	TotalTokens              int64
	MessageCount             int
}

// CacheAttributionAggregate 计算 CacheSegment 中 provider 已报告的命中读占比。
// denominator 只包含 cache_read 与 cache_creation；无 provider cache breakdown 时 ok=false。
func (s CacheSegment) CacheReadShare() (share float64, ok bool) {
	denominator := s.CacheReadInputTokens + s.CacheCreationInputTokens
	if denominator <= 0 {
		return 0, false
	}
	return float64(s.CacheReadInputTokens) / float64(denominator), true
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
