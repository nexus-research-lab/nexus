// =====================================================
// @File   ：model.go
// @Date   ：2026/04/10 23:10:00
// @Author ：leemysw
// 2026/04/10 23:10:00   Create
// =====================================================

package launcher

// QueryRequest 表示 Launcher 查询请求。
type QueryRequest struct {
	Query string `json:"query"`
}

// QueryResponse 表示 Launcher 查询响应。
type QueryResponse struct {
	ActionType     string `json:"action_type"`
	TargetID       string `json:"target_id"`
	InitialMessage string `json:"initial_message,omitempty"`
}

// Suggestion 表示 Launcher 推荐项。
type Suggestion struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	Avatar       string `json:"avatar,omitempty"`
	LastActivity string `json:"last_activity,omitempty"`
}

// SuggestionsResponse 表示 Launcher 推荐列表。
type SuggestionsResponse struct {
	Agents []Suggestion `json:"agents"`
	Rooms  []Suggestion `json:"rooms"`
}

// BootstrapAgent 表示 Launcher 首屏所需的 Agent 摘要。
type BootstrapAgent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

// BootstrapRoom 表示 Launcher 首屏所需的 Room 摘要。
type BootstrapRoom struct {
	ID              string `json:"id"`
	RoomType        string `json:"room_type"`
	Name            string `json:"name,omitempty"`
	Avatar          string `json:"avatar,omitempty"`
	DMTargetAgentID string `json:"dm_target_agent_id,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

// BootstrapConversation 表示 Launcher 推荐区使用的会话摘要。
type BootstrapConversation struct {
	SessionKey     string `json:"session_key"`
	AgentID        string `json:"agent_id,omitempty"`
	RoomID         string `json:"room_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	RoomType       string `json:"room_type"`
	Title          string `json:"title"`
	LastActivity   string `json:"last_activity"`
}

// BootstrapResponse 表示 Launcher 首屏数据。
type BootstrapResponse struct {
	Agents        []BootstrapAgent        `json:"agents"`
	Rooms         []BootstrapRoom         `json:"rooms"`
	Conversations []BootstrapConversation `json:"conversations"`
}
