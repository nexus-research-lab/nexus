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
	TargetAgentIDs  []string           `json:"target_agent_ids,omitempty"`
	Source          InputQueueSource   `json:"source"`
	Content         string             `json:"content"`
	DeliveryPolicy  ChatDeliveryPolicy `json:"delivery_policy"`
	OwnerUserID     string             `json:"owner_user_id,omitempty"`
	RootRoundID     string             `json:"root_round_id,omitempty"`
	HopIndex        int                `json:"hop_index,omitempty"`
	QueueOrder      int64              `json:"queue_order,omitempty"`
	CreatedAt       int64              `json:"created_at"`
	UpdatedAt       int64              `json:"updated_at"`
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
	switch InputQueueSource(strings.ToLower(strings.TrimSpace(value))) {
	case InputQueueSourceAgentPublicMention:
		return InputQueueSourceAgentPublicMention
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
