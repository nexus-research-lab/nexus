package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) broadcastRoomInputQueueSnapshot(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) error {
	items, err := s.roomInputQueueItems(ctx, contextValue)
	if err != nil {
		return err
	}
	s.broadcastInputQueueItems(ctx, sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, items)
	return nil
}

func (s *RealtimeService) broadcastInputQueueItems(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	items []protocol.InputQueueItem,
) {
	s.broadcastSharedEvent(ctx, sessionKey, roomID, newRoomInputQueueEvent(sessionKey, roomID, conversationID, items))
}

func newRoomInputQueueEvent(sessionKey string, roomID string, conversationID string, items []protocol.InputQueueItem) protocol.EventMessage {
	event := protocol.NewInputQueueEvent(sessionKey, items)
	event.Data["scope"] = string(protocol.InputQueueScopeRoom)
	event.RoomID = strings.TrimSpace(roomID)
	event.ConversationID = strings.TrimSpace(conversationID)
	return event
}
