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
	Messages       []Message  `json:"messages,omitempty"`
}

type DeliveryOutput struct {
	LeaseID string         `json:"lease_id"`
	Kind    string         `json:"kind"`
	Content MessageContent `json:"content"`
}
