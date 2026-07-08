package room

import "github.com/nexus-research-lab/nexus/internal/protocol"

// WrapMessageEvent 构建 Room 公区消息事件。roundID 必须是 root round。
func WrapMessageEvent(roomID string, conversationID string, message protocol.Message, roundID string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeMessage, message)
	event.DeliveryMode = "durable"
	event.SessionKey = normalizeAnyString(message["session_key"])
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.AgentID = normalizeAnyString(message["agent_id"])
	event.MessageID = normalizeAnyString(message["message_id"])
	event.RoundID = roundID
	event.AgentRoundID = normalizeAnyString(message["agent_round_id"])
	return event
}

// WrapRoundStatusEvent 构建 Room root round 状态事件。
func WrapRoundStatusEvent(sessionKey string, roomID string, conversationID string, roundID string, status string, resultSubtype string) protocol.EventMessage {
	event := protocol.NewRoundStatusEvent(sessionKey, roundID, status, resultSubtype)
	event.DeliveryMode = "durable"
	event.RoomID = roomID
	event.ConversationID = conversationID
	return event
}

// WrapAgentRoundStatusEvent 构建 Room slot 状态事件。
func WrapAgentRoundStatusEvent(sessionKey string, roomID string, conversationID string, roundID string, agentRoundID string, agentID string, status string) protocol.EventMessage {
	event := protocol.NewAgentRoundStatusEvent(sessionKey, roundID, agentRoundID, agentID, status)
	event.DeliveryMode = "durable"
	event.RoomID = roomID
	event.ConversationID = conversationID
	return event
}

// WrapChatAckEvent 构建 Room chat ack 事件。
func WrapChatAckEvent(
	sessionKey string,
	roomID string,
	conversationID string,
	clientRequestID string,
	clientMessageID string,
	roundID string,
	userMessageID string,
	pending []protocol.ChatAckPendingSlot,
) protocol.EventMessage {
	event := protocol.NewChatAckEvent(sessionKey, clientRequestID, clientMessageID, roundID, userMessageID, pending)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.RoundID = roundID
	return event
}

// WrapLifecycleEvent 构建 Room slot 生命周期事件。
func WrapLifecycleEvent(eventType protocol.EventType, sessionKey string, roomID string, conversationID string, agentID string, msgID string, roundID string, agentRoundID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, map[string]any{
		"msg_id":         msgID,
		"agent_id":       agentID,
		"round_id":       roundID,
		"agent_round_id": agentRoundID,
	})
	event.SessionKey = sessionKey
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.AgentID = agentID
	event.MessageID = msgID
	event.RoundID = roundID
	event.AgentRoundID = agentRoundID
	return event
}

// NewErrorEvent 构建 Room 错误事件。roundID 可为空。
func NewErrorEvent(sessionKey string, roomID string, conversationID string, errorType string, message string, roundID string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeError, map[string]any{
		"error_type": errorType,
		"message":    message,
	})
	event.SessionKey = sessionKey
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.RoundID = roundID
	return event
}
