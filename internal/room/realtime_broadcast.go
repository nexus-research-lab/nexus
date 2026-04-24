package room

import (
	"context"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"strings"
)

func (s *RealtimeService) withBroadcastTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, roomBroadcastTimeout)
}

func (s *RealtimeService) broadcastSharedEventWithTimeout(
	ctx context.Context,
	sessionKey string,
	roomID string,
	event protocol.EventMessage,
) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	s.broadcastSharedEvent(broadcastCtx, sessionKey, roomID, event)
}

func (s *RealtimeService) broadcastSessionStatus(ctx context.Context, sessionKey string) {
	broadcastCtx, cancel := s.withBroadcastTimeout(ctx)
	defer cancel()
	if errs := s.permission.BroadcastSessionStatus(
		broadcastCtx,
		sessionKey,
		s.runtime.GetRunningRoundIDs(sessionKey),
	); len(errs) > 0 {
		s.loggerFor(broadcastCtx).Warn("广播 room session 状态失败", "session_key", sessionKey, "error_count", len(errs))
	}
}

func (s *RealtimeService) broadcastSharedEvent(ctx context.Context, sessionKey string, roomID string, event protocol.EventMessage) {
	if s.broadcaster != nil && strings.TrimSpace(roomID) != "" {
		s.broadcaster.Broadcast(ctx, roomID, event)
		return
	}
	s.permission.BroadcastEvent(ctx, sessionKey, event)
}

func wrapRoomMessageEvent(roomID string, conversationID string, message protocol.Message, causedBy string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeMessage, message)
	event.DeliveryMode = "durable"
	event.SessionKey = anyString(message["session_key"])
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.AgentID = anyString(message["agent_id"])
	event.MessageID = anyString(message["message_id"])
	event.CausedBy = causedBy
	return event
}

func wrapRoomRoundStatusEvent(sessionKey string, roomID string, conversationID string, roundID string, status string, resultSubtype string) protocol.EventMessage {
	event := protocol.NewRoundStatusEvent(sessionKey, roundID, status, resultSubtype)
	event.DeliveryMode = "durable"
	event.RoomID = roomID
	event.ConversationID = conversationID
	return event
}

func wrapRoomChatAckEvent(sessionKey string, roomID string, conversationID string, reqID string, roundID string, pending []map[string]any) protocol.EventMessage {
	event := protocol.NewChatAckEvent(sessionKey, reqID, roundID, pending)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.CausedBy = roundID
	return event
}

func wrapRoomLifecycleEvent(eventType protocol.EventType, sessionKey string, roomID string, conversationID string, agentID string, msgID string, roundID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, map[string]any{
		"msg_id":   msgID,
		"agent_id": agentID,
		"round_id": roundID,
	})
	event.SessionKey = sessionKey
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.AgentID = agentID
	event.MessageID = msgID
	event.CausedBy = roundID
	return event
}

func (s *RealtimeService) newRoomErrorEvent(sessionKey string, roomID string, conversationID string, errorType string, message string, causedBy string) protocol.EventMessage {
	event := protocol.NewEvent(protocol.EventTypeError, map[string]any{
		"error_type": errorType,
		"message":    message,
	})
	event.SessionKey = sessionKey
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.CausedBy = causedBy
	return event
}
