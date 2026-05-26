package protocol

// AgentPrivateParticipant 表示私域投影里的 Agent 参与者摘要。
type AgentPrivateParticipant struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name,omitempty"`
	Avatar  string `json:"avatar,omitempty"`
}

// AgentPrivateThread 表示从某个 Agent 视角聚合出来的一条私域会话线索。
type AgentPrivateThread struct {
	ThreadID            string                    `json:"thread_id"`
	AgentID             string                    `json:"agent_id"`
	Scope               string                    `json:"scope"`
	ParticipantAgentIDs []string                  `json:"participant_agent_ids"`
	PeerAgentIDs        []string                  `json:"peer_agent_ids"`
	Participants        []AgentPrivateParticipant `json:"participants"`
	RoomID              string                    `json:"room_id,omitempty"`
	RoomName            string                    `json:"room_name,omitempty"`
	RoomType            string                    `json:"room_type,omitempty"`
	ConversationID      string                    `json:"conversation_id,omitempty"`
	ConversationTitle   string                    `json:"conversation_title,omitempty"`
	LastActionID        string                    `json:"last_action_id,omitempty"`
	LastActionType      RoomActionType            `json:"last_action_type,omitempty"`
	LastContentPreview  string                    `json:"last_content_preview,omitempty"`
	LastTimestamp       int64                     `json:"last_timestamp,omitempty"`
	ActionCount         int                       `json:"action_count"`
}

// AgentPrivateEvent 表示私域线程内的一条可见 action。
type AgentPrivateEvent struct {
	ActionID          string                    `json:"action_id"`
	ThreadID          string                    `json:"thread_id"`
	Direction         string                    `json:"direction"`
	ActionType        RoomActionType            `json:"action_type"`
	RequestID         string                    `json:"request_id,omitempty"`
	SourceAgentID     string                    `json:"source_agent_id"`
	TargetAgentID     string                    `json:"target_agent_id,omitempty"`
	AudienceAgentIDs  []string                  `json:"audience_agent_ids,omitempty"`
	Content           string                    `json:"content,omitempty"`
	Visibility        string                    `json:"visibility"`
	ReplyTarget       RoomReplyTarget           `json:"reply_target"`
	WakePolicy        RoomWakePolicy            `json:"wake_policy,omitempty"`
	DelaySeconds      int                       `json:"delay_seconds,omitempty"`
	RoomID            string                    `json:"room_id,omitempty"`
	RoomName          string                    `json:"room_name,omitempty"`
	RoomType          string                    `json:"room_type,omitempty"`
	ConversationID    string                    `json:"conversation_id,omitempty"`
	ConversationTitle string                    `json:"conversation_title,omitempty"`
	Participants      []AgentPrivateParticipant `json:"participants"`
	Timestamp         int64                     `json:"timestamp"`
}

// AgentPrivateThreadPage 表示私域线程列表响应。
type AgentPrivateThreadPage struct {
	Items []AgentPrivateThread `json:"items"`
}

// AgentPrivateEventPage 表示私域线程事件列表响应。
type AgentPrivateEventPage struct {
	Thread AgentPrivateThread  `json:"thread"`
	Items  []AgentPrivateEvent `json:"items"`
}
