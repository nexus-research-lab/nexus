// INPUT: Relay 的节点领取、租约和输出 wire 合同。
// OUTPUT: 不依赖闭源 Relay 包的宿主 DTO。
// POS: Node 执行 transport 的跨服务边界。
package relay

import "time"

type PendingDeliveries struct {
	AgentIDs  []string   `json:"agent_ids"`
	NextDueAt *time.Time `json:"next_due_at,omitempty"`
}

type Delivery struct {
	ExecutionState string     `json:"execution_state,omitempty"`
	AgentIDs       []string   `json:"agent_ids,omitempty"`
	ID             string     `json:"id"`
	RoomID         string     `json:"room_id"`
	ConversationID string     `json:"conversation_id"`
	MessageID      string     `json:"message_id"`
	AgentID        string     `json:"agent_id"`
	State          string     `json:"state"`
	NodeID         string     `json:"node_id"`
	LeaseID        string     `json:"lease_id"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at"`
	FailureCode    string     `json:"failure_code,omitempty"`
	// CancelRequested 随续期返回：真人已请求停止，节点中断本机执行后以 fail 收口。
	CancelRequested bool `json:"cancel_requested,omitempty"`
	// RoomInstructions 只随领取下发，是群管理员设置的共同说明，可信度等同群消息。
	RoomInstructions string    `json:"room_instructions,omitempty"`
	Messages         []Message `json:"messages,omitempty"`
}

type DeliveryOutput struct {
	LeaseID  string           `json:"lease_id"`
	Kind     string           `json:"kind"`
	Content  MessageContent   `json:"content"`
	Mentions []MessageMention `json:"mentions,omitempty"`
}
