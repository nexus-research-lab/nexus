package room

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
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
	return message.NewGuidedInputMessage(message.GuidedInputMessageInput{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		AgentID:        slot.AgentID,
		RoundID:        slot.AgentRoundID,
		SourceRoundID:  sourceRoundID,
		Content:        content,
		SessionID:      slot.SDKSessionID,
		Timestamp:      time.Now().UnixMilli(),
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
	event := wrapRoomMessageEvent(roomID, conversationID, message, strings.TrimSpace(sourceRoundID))
	event.DeliveryMode = "ephemeral"
	s.broadcastSharedEvent(ctx, sessionKey, roomID, event)
}
