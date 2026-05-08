package room

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func roomEventsReadyForEmission(slot *activeRoomSlot, event protocol.EventMessage) []protocol.EventMessage {
	if slot == nil {
		return []protocol.EventMessage{event}
	}
	return slot.eventsReadyForEmission(event)
}

func shouldHoldNoReplyCandidateStream(slot *activeRoomSlot, event protocol.EventMessage) bool {
	if slot == nil {
		return false
	}
	ready := slot.eventsReadyForEmission(event)
	return len(ready) == 0
}
