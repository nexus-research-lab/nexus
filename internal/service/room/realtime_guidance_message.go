package room

import (
	"context"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func buildRoomGuidanceMessage(
	sessionKey string,
	roomID string,
	conversationID string,
	slot *activeRoomSlot,
	sourceRoundID string,
	content string,
) protocol.Message {
	if slot == nil {
		return protocol.Message{}
	}
	return roomdomain.BuildGuidanceMessage(roomdomain.GuidanceMessageInput{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		AgentID:        slot.AgentID,
		AgentRoundID:   slot.AgentRoundID,
		SourceRoundID:  sourceRoundID,
		Content:        content,
		SDKSessionID:   slot.SDKSessionID,
	})
}

func (s *RealtimeService) broadcastSlotGuidanceMessage(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	sourceRoundID string,
	message protocol.Message,
) {
	if len(message) == 0 {
		return
	}
	event := roomdomain.WrapMessageEvent(roomID, conversationID, message, strings.TrimSpace(sourceRoundID))
	event.DeliveryMode = "ephemeral"
	s.broadcastSharedEvent(ctx, sessionKey, roomID, event)
}
