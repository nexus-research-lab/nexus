package room

import (
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
