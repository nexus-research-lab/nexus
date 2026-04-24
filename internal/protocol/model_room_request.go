package protocol

// CreateRoomRequest 表示创建房间请求。
type CreateRoomRequest struct {
	AgentIDs    []string `json:"agent_ids"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Title       string   `json:"title,omitempty"`
	Avatar      string   `json:"avatar,omitempty"`
}

// UpdateRoomRequest 表示更新房间请求。
type UpdateRoomRequest struct {
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	Title       string  `json:"title,omitempty"`
	Avatar      *string `json:"avatar,omitempty"`
}

// AddRoomMemberRequest 表示追加成员请求。
type AddRoomMemberRequest struct {
	AgentID string `json:"agent_id"`
}

// CreateConversationRequest 表示创建话题请求。
type CreateConversationRequest struct {
	Title string `json:"title,omitempty"`
}

// UpdateConversationRequest 表示更新话题请求。
type UpdateConversationRequest struct {
	Title string `json:"title,omitempty"`
}
