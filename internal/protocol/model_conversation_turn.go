package protocol

// ConversationMessage 是投影后的对话消息，round_id 永远是 root round。
type ConversationMessage struct {
	MessageID     string         `json:"message_id"`
	SessionKey    string         `json:"session_key,omitempty"`
	Role          string         `json:"role"`
	RoundID       string         `json:"round_id"`
	AgentRoundID  string         `json:"agent_round_id,omitempty"`
	AgentID       string         `json:"agent_id,omitempty"`
	ParentID      string         `json:"parent_id,omitempty"`
	Content       any            `json:"content"`
	Timestamp     int64          `json:"timestamp"`
	StreamStatus  string         `json:"stream_status,omitempty"`
	ResultSummary map[string]any `json:"result_summary,omitempty"`
}

// TurnPendingPermission 是投影输出中挂在 slot 上的待确认权限。
type TurnPendingPermission struct {
	RequestID string `json:"request_id"`
	MessageID string `json:"message_id,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
}

// AgentTurnSlot 表示一个 agent 在某个 turn 中的执行槽位。
type AgentTurnSlot struct {
	AgentID            string                  `json:"agent_id"`
	AgentRoundID       string                  `json:"agent_round_id"`
	MsgID              string                  `json:"msg_id,omitempty"`
	Status             string                  `json:"status"`
	AssistantMessages  []ConversationMessage   `json:"assistant_messages"`
	PendingPermissions []TurnPendingPermission `json:"pending_permissions"`
	ResultSummary      map[string]any          `json:"result_summary,omitempty"`
	StartedAt          *int64                  `json:"started_at,omitempty"`
	FinishedAt         *int64                  `json:"finished_at,omitempty"`
}

// ConversationTurn 是前端时间线主对象，也是历史分页单位。
type ConversationTurn struct {
	RoundID      string                `json:"round_id"`
	Status       string                `json:"status"`
	CreatedAt    int64                 `json:"created_at"`
	UpdatedAt    int64                 `json:"updated_at"`
	UserMessage  *ConversationMessage  `json:"user_message"`
	AgentSlots   []AgentTurnSlot       `json:"agent_slots"`
	SystemEvents []ConversationMessage `json:"system_events"`
	IsLoaded     bool                  `json:"is_loaded"`
}

// ConversationTurnIndexItem 是 navigator / 虚拟列表占位用的 turn 索引项。
type ConversationTurnIndexItem struct {
	RoundID     string   `json:"round_id"`
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
	Status      string   `json:"status"`
	UserPreview string   `json:"user_preview"`
	AgentIDs    []string `json:"agent_ids"`
	Loaded      bool     `json:"loaded"`
}

// TurnPage 是 /turns 历史 API 的分页响应。
type TurnPage struct {
	Turns                 []ConversationTurn `json:"turns"`
	NextBeforeRoundID     string             `json:"next_before_round_id,omitempty"`
	BackwardsAfterRoundID string             `json:"backwards_after_round_id,omitempty"`
}
