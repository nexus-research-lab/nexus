package room

import (
	"context"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func roomSlotReplyTarget(slot *activeRoomSlot) protocol.RoomReplyTarget {
	if slot == nil || slot.ReplyTarget == "" {
		return protocol.RoomReplyTargetPublicFeed
	}
	return slot.ReplyTarget
}

func roomSlotPublishesPublicOutput(slot *activeRoomSlot) bool {
	return roomSlotReplyTarget(slot) == protocol.RoomReplyTargetPublicFeed
}

func roomSlotShouldDropPublicOutputEvent(slot *activeRoomSlot, event protocol.EventMessage) bool {
	if roomSlotPublishesPublicOutput(slot) {
		return false
	}
	return event.EventType == protocol.EventTypeStream || event.EventType == protocol.EventTypeMessage
}

func (s *RealtimeService) recordRoomActionReply(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	assistantMessage protocol.Message,
) error {
	if s.actions == nil || roundValue == nil || slot == nil || strings.TrimSpace(slot.ReplySourceAction) == "" {
		return nil
	}
	replyTarget := roomSlotReplyTarget(slot)
	if replyTarget == protocol.RoomReplyTargetPublicFeed ||
		replyTarget == protocol.RoomReplyTargetTargetPrivate ||
		replyTarget == protocol.RoomReplyTargetNone {
		return nil
	}
	content := strings.TrimSpace(roomdomain.ExtractAssistantResultText(assistantMessage))
	if content == "" {
		return nil
	}

	action := protocol.RoomActionRecord{
		ActionID:       newRealtimeID(),
		RoomID:         roundValue.RoomID,
		ConversationID: roundValue.ConversationID,
		RequestID:      strings.TrimSpace(slot.ReplyRequestID),
		SourceAgentID:  strings.TrimSpace(slot.AgentID),
		Content:        content,
		Visibility:     protocol.RoomActionVisibilityPrivate,
		Timestamp:      time.Now().UnixMilli(),
	}
	switch replyTarget {
	case protocol.RoomReplyTargetSenderPrivate:
		targetAgentID := strings.TrimSpace(slot.ReplySourceAgent)
		if targetAgentID == "" {
			return nil
		}
		action.ActionType = protocol.RoomActionTypePrivateMessage
		action.TargetAgentID = targetAgentID
		action.ReplyTarget = protocol.RoomReplyTargetTargetPrivate
	case protocol.RoomReplyTargetAudience:
		audienceAgentIDs := normalizeRoomActionAudience(slot.ReplyAudience)
		if len(audienceAgentIDs) == 0 {
			return nil
		}
		action.ActionType = protocol.RoomActionTypeMarker
		action.AudienceAgentIDs = audienceAgentIDs
		action.ReplyTarget = protocol.RoomReplyTargetAudience
	default:
		return nil
	}
	if err := s.actions.AppendAction(action); err != nil {
		return err
	}
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, newRoomActionEvent(action))
	if roomActionReplyVisibleToAgent(action, slot.AgentID) {
		slot.ActionCursorID = action.ActionID
		slot.ActionCursorTS = action.Timestamp
	}
	return nil
}

func roomActionReplyVisibleToAgent(action protocol.RoomActionRecord, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return false
	}
	if action.ActionType == protocol.RoomActionTypePrivateMessage {
		return strings.TrimSpace(action.TargetAgentID) == agentID
	}
	if action.ReplyTarget == protocol.RoomReplyTargetAudience {
		for _, audienceAgentID := range action.AudienceAgentIDs {
			if strings.TrimSpace(audienceAgentID) == agentID {
				return true
			}
		}
	}
	return false
}
