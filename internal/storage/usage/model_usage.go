package usage

import "time"

// Record 表示 token usage ledger 的一条持久化记录。
type Record struct {
	OwnerUserID              string
	UsageKey                 string
	Source                   string
	SessionKey               string
	MessageID                string
	RoundID                  string
	AgentID                  string
	RoomID                   string
	ConversationID           string
	GoalScope                string
	ExecutionScope           string
	ResponsibilityLane       string
	RuntimeKind              string
	ProviderFingerprint      string
	ModelFingerprint         string
	GoalToolSurface          string
	ExecutionToolSurface     string
	HostToolSurfaceComplete  bool
	ToolPolicyFingerprint    string
	MCPServersFingerprint    string
	ToolSurfaceFingerprint   string
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	TotalTokens              int64
	OccurredAt               time.Time
}

// CacheSegment 聚合同一 cache correlation surface 的 provider token 回报。
type CacheSegment struct {
	Source                   string
	GoalScope                string
	ExecutionScope           string
	ResponsibilityLane       string
	RuntimeKind              string
	ProviderFingerprint      string
	ModelFingerprint         string
	GoalToolSurface          string
	ExecutionToolSurface     string
	HostToolSurfaceComplete  bool
	ToolPolicyFingerprint    string
	MCPServersFingerprint    string
	ToolSurfaceFingerprint   string
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	TotalTokens              int64
	MessageCount             int
}

// Summary 表示 token usage ledger 的聚合结果。
type Summary struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	TotalTokens              int64
	SessionCount             int
	MessageCount             int
	UpdatedAt                string
}
