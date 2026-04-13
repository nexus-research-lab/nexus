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
