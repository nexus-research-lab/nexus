package protocol

// RoomReplyRouteMode 表示 directed message 唤醒后的回复投影位置。
type RoomReplyRouteMode string

const (
	RoomReplyRoutePublic  RoomReplyRouteMode = "public"
	RoomReplyRoutePrivate RoomReplyRouteMode = "private"
	RoomReplyRouteNone    RoomReplyRouteMode = "none"
)

// RoomWakePolicy 表示 directed message 是否触发目标成员运行。
type RoomWakePolicy string

const (
	RoomWakePolicyNone      RoomWakePolicy = "none"
	RoomWakePolicyImmediate RoomWakePolicy = "immediate"
	RoomWakePolicyDelayed   RoomWakePolicy = "delayed"
)

// RoomReplyRoute 表示 directed message 触发后的 final reply 投影规则。
type RoomReplyRoute struct {
	Mode           RoomReplyRouteMode `json:"mode"`
	Recipients     []string           `json:"recipients,omitempty"`
	WakePolicy     RoomWakePolicy     `json:"wake_policy,omitempty"`
	NextReplyRoute *RoomReplyRoute    `json:"next_reply_route,omitempty"`
}

// CreateRoomDirectedMessageRequest 表示创建 Room directed message 的请求。
type CreateRoomDirectedMessageRequest struct {
	// SourceAgentID 只能由受控运行时注入，不能从 JSON body 写入。
	SourceAgentID string         `json:"-"`
	Recipients    []string       `json:"recipients"`
	Content       string         `json:"content"`
	WakePolicy    RoomWakePolicy `json:"wake_policy,omitempty"`
	ReplyRoute    RoomReplyRoute `json:"reply_route"`
	DelaySeconds  int            `json:"delay_seconds,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}

// RoomDirectedMessageRecord 表示 Room directed message 的 append-only 持久化记录。
type RoomDirectedMessageRecord struct {
	MessageID      string         `json:"message_id"`
	RoomID         string         `json:"room_id"`
	ConversationID string         `json:"conversation_id"`
	SourceAgentID  string         `json:"source_agent_id"`
	Recipients     []string       `json:"recipients"`
	Content        string         `json:"content,omitempty"`
	WakePolicy     RoomWakePolicy `json:"wake_policy,omitempty"`
	ReplyRoute     RoomReplyRoute `json:"reply_route"`
	DelaySeconds   int            `json:"delay_seconds,omitempty"`
	CorrelationID  string         `json:"correlation_id,omitempty"`
	Timestamp      int64          `json:"timestamp"`
}

// CreateRoomPublicMessageRequest 表示 Room 成员主动发布公区消息的请求。
type CreateRoomPublicMessageRequest struct {
	// SourceAgentID 只能由受控运行时注入，不能从 JSON body 写入。
	SourceAgentID string `json:"-"`
	Content       string `json:"content"`
	CorrelationID string `json:"correlation_id,omitempty"`
}
