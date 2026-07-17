// INPUT: 输入队列项、变更结果与客户端请求关联 ID。
// OUTPUT: 对外提供输入队列协议模型、快照事件与持久接受 ACK。
// POS: protocol 包的输入队列跨边界真相源。
package protocol

import "strings"

// InputQueueScope 表示待发送队列所在的会话面。
type InputQueueScope string

const (
	InputQueueScopeDM   InputQueueScope = "dm"
	InputQueueScopeRoom InputQueueScope = "room"
)

// InputQueueSource 表示队列项来源。
type InputQueueSource string

const (
	InputQueueSourceUser               InputQueueSource = "user"
	InputQueueSourceAgentPublicMention InputQueueSource = "agent_public_mention"
	InputQueueSourceAgentRoomMessage   InputQueueSource = "agent_room_directed_message"
)

// InputQueueItem 表示后端同步的待发送队列项。
type InputQueueItem struct {
	ID              string             `json:"id"`
	Scope           InputQueueScope    `json:"scope"`
	SessionKey      string             `json:"session_key"`
	RoomID          string             `json:"room_id,omitempty"`
	ConversationID  string             `json:"conversation_id,omitempty"`
	AgentID         string             `json:"agent_id,omitempty"`
	SourceAgentID   string             `json:"source_agent_id,omitempty"`
	SourceMessageID string             `json:"source_message_id,omitempty"`
	HandoffID       string             `json:"handoff_id,omitempty"`
	TargetAgentIDs  []string           `json:"target_agent_ids,omitempty"`
	Source          InputQueueSource   `json:"source"`
	Content         string             `json:"content"`
	Attachments     []ChatAttachment   `json:"attachments,omitempty"`
	DeliveryPolicy  ChatDeliveryPolicy `json:"delivery_policy"`
	ReplyRoute      RoomReplyRoute     `json:"reply_route,omitempty"`
	OwnerUserID     string             `json:"owner_user_id,omitempty"`
	RootRoundID     string             `json:"root_round_id,omitempty"`
	HopIndex        int                `json:"hop_index,omitempty"`
	QueueOrder      int64              `json:"queue_order,omitempty"`
	ExpiresAt       int64              `json:"expires_at,omitempty"`
	CreatedAt       int64              `json:"created_at"`
	UpdatedAt       int64              `json:"updated_at"`
}

// InputQueueMutationResult 表示一次已被服务端持久接受的输入队列变更。
type InputQueueMutationResult struct {
	Action    string `json:"action"`
	ItemID    string `json:"item_id,omitempty"`
	Duplicate bool   `json:"duplicate"`
}

// NormalizeInputQueueScope 归一化队列作用域。
func NormalizeInputQueueScope(value string) InputQueueScope {
	switch InputQueueScope(strings.ToLower(strings.TrimSpace(value))) {
	case InputQueueScopeRoom:
		return InputQueueScopeRoom
	default:
		return InputQueueScopeDM
	}
}

// NormalizeInputQueueSource 归一化队列来源。
func NormalizeInputQueueSource(value string) InputQueueSource {
	normalized := InputQueueSource(strings.ToLower(strings.TrimSpace(value)))
	switch normalized {
	case InputQueueSourceAgentPublicMention, InputQueueSourceAgentRoomMessage:
		return normalized
	default:
		return InputQueueSourceUser
	}
}

// NewInputQueueEvent 构造 input_queue 快照事件。
func NewInputQueueEvent(sessionKey string, items []InputQueueItem) EventMessage {
	if items == nil {
		items = []InputQueueItem{}
	}
	scope := string(InputQueueScopeDM)
	roomID := ""
	conversationID := ""
	if len(items) > 0 {
		scope = string(items[0].Scope)
		roomID = strings.TrimSpace(items[0].RoomID)
		conversationID = strings.TrimSpace(items[0].ConversationID)
	}
	event := NewEvent(EventTypeInputQueue, map[string]any{
		"scope": scope,
		"items": items,
	})
	event.SessionKey = strings.TrimSpace(sessionKey)
	event.RoomID = roomID
	event.ConversationID = conversationID
	return event
}

// NewInputQueueAckEvent 构造 input_queue_ack 事件。
// client_request_id / client_message_id 原样回传；duplicate 表示同一幂等请求此前已持久接受。
func NewInputQueueAckEvent(
	sessionKey string,
	clientRequestID string,
	clientMessageID string,
	result InputQueueMutationResult,
) EventMessage {
	event := NewEvent(EventTypeInputQueueAck, map[string]any{
		"accepted":          true,
		"duplicate":         result.Duplicate,
		"action":            strings.TrimSpace(result.Action),
		"item_id":           strings.TrimSpace(result.ItemID),
		"client_request_id": clientRequestID,
		"client_message_id": clientMessageID,
		"ack_timeout_ms":    RequestAckTimeoutMS,
	})
	event.SessionKey = strings.TrimSpace(sessionKey)
	return event
}
