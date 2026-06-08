package room

import (
	"context"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func roomSlotReplyRoute(slot *activeRoomSlot) protocol.RoomReplyRoute {
	if slot == nil || slot.ReplyRoute.Mode == "" {
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePublic}
	}
	return slot.ReplyRoute
}

func roomSlotPublishesPublicOutput(slot *activeRoomSlot) bool {
	return roomSlotReplyRoute(slot).Mode == protocol.RoomReplyRoutePublic
}

func roomSlotShouldDropPublicOutputEvent(slot *activeRoomSlot, event protocol.EventMessage) bool {
	if roomSlotPublishesPublicOutput(slot) {
		return false
	}
	return event.EventType == protocol.EventTypeStream || event.EventType == protocol.EventTypeMessage
}

func (s *RealtimeService) recordRoomDirectedMessageReply(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	assistantMessage protocol.Message,
) error {
	if s.directedMessages == nil || roundValue == nil || slot == nil || strings.TrimSpace(slot.ReplySourceMessage) == "" {
		return nil
	}
	replyRoute := roomSlotReplyRoute(slot)
	if replyRoute.Mode != protocol.RoomReplyRoutePrivate {
		return nil
	}
	recipients := normalizeRoomDirectedMessageRecipients(replyRoute.Recipients)
	if len(recipients) == 0 {
		return nil
	}
	content := strings.TrimSpace(roomdomain.ExtractAssistantResultText(assistantMessage))
	if content == "" {
		return nil
	}

	message := protocol.RoomDirectedMessageRecord{
		MessageID:      newRealtimeID(),
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
		SourceAgentID:  strings.TrimSpace(slot.AgentID),
		Recipients:     recipients,
		Content:        content,
		WakePolicy:     protocol.RoomWakePolicyNone,
		ReplyRoute:     roomReplyRouteAfterPrivateHandback(replyRoute),
		Timestamp:      time.Now().UnixMilli(),
	}
	if err := s.directedMessages.AppendMessage(message); err != nil {
		return err
	}
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, newRoomDirectedMessageEvent(message))
	if replyRoute.WakePolicy == protocol.RoomWakePolicyImmediate {
		wakeMessage := message
		wakeMessage.WakePolicy = protocol.RoomWakePolicyImmediate
		return s.runRoomDirectedMessageWake(ctx, roundValue.Context, wakeMessage)
	}
	return nil
}

func roomReplyRouteAfterPrivateHandback(route protocol.RoomReplyRoute) protocol.RoomReplyRoute {
	if route.NextReplyRoute == nil {
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone}
	}
	return cloneRoomReplyRoute(*route.NextReplyRoute)
}

func cloneRoomReplyRoute(route protocol.RoomReplyRoute) protocol.RoomReplyRoute {
	cloned := protocol.RoomReplyRoute{
		Mode:       route.Mode,
		Recipients: append([]string(nil), route.Recipients...),
		WakePolicy: route.WakePolicy,
	}
	if route.NextReplyRoute != nil {
		next := cloneRoomReplyRoute(*route.NextReplyRoute)
		cloned.NextReplyRoute = &next
	}
	return cloned
}
