package protocol

// RoomActionType 表示 Room 内部协作动作类型。
type RoomActionType string

const (
	RoomActionTypePrivateMessage RoomActionType = "private_message"
	RoomActionTypeRequestReply   RoomActionType = "request_reply"
	RoomActionTypePrivateNote    RoomActionType = "private_note"
	RoomActionTypeMarker         RoomActionType = "marker"
)

// RoomReplyTarget 表示 Room action 后续回复投影位置。
type RoomReplyTarget string

const (
	RoomReplyTargetPublicFeed    RoomReplyTarget = "public_feed"
	RoomReplyTargetSenderPrivate RoomReplyTarget = "sender_private"
	RoomReplyTargetTargetPrivate RoomReplyTarget = "target_private"
	RoomReplyTargetAudience      RoomReplyTarget = "audience"
	RoomReplyTargetNone          RoomReplyTarget = "none"
)

const (
	RoomActionVisibilityPublic  = "public"
	RoomActionVisibilityPrivate = "private"
)

// RoomWakePolicy 表示 Room action 是否触发目标成员运行。
type RoomWakePolicy string

const (
	RoomWakePolicyNone      RoomWakePolicy = "none"
	RoomWakePolicyImmediate RoomWakePolicy = "immediate"
	RoomWakePolicyDelayed   RoomWakePolicy = "delayed"
)

// CreateRoomActionRequest 表示创建 Room action 的请求。
type CreateRoomActionRequest struct {
	ActionType RoomActionType `json:"action_type"`
	// SourceAgentID 只能由受控运行时注入，不能从 action JSON body 写入。
	SourceAgentID    string          `json:"-"`
	TargetAgentID    string          `json:"target_agent_id,omitempty"`
	AudienceAgentIDs []string        `json:"audience_agent_ids,omitempty"`
	Content          string          `json:"content"`
	Visibility       string          `json:"visibility,omitempty"`
	ReplyTarget      RoomReplyTarget `json:"reply_target,omitempty"`
	WakePolicy       RoomWakePolicy  `json:"wake_policy,omitempty"`
	DelaySeconds     int             `json:"delay_seconds,omitempty"`
}

// RoomActionRecord 表示 Room action 的 append-only 持久化记录。
type RoomActionRecord struct {
	ActionID         string          `json:"action_id"`
	RoomID           string          `json:"room_id"`
	ConversationID   string          `json:"conversation_id"`
	ActionType       RoomActionType  `json:"action_type"`
	RequestID        string          `json:"request_id,omitempty"`
	SourceAgentID    string          `json:"source_agent_id"`
	TargetAgentID    string          `json:"target_agent_id,omitempty"`
	AudienceAgentIDs []string        `json:"audience_agent_ids,omitempty"`
	Content          string          `json:"content,omitempty"`
	Visibility       string          `json:"visibility"`
	ReplyTarget      RoomReplyTarget `json:"reply_target"`
	WakePolicy       RoomWakePolicy  `json:"wake_policy,omitempty"`
	DelaySeconds     int             `json:"delay_seconds,omitempty"`
	Timestamp        int64           `json:"timestamp"`
}
