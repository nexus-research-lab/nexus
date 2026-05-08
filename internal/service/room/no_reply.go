package room

import (
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func roomEventsReadyForEmission(slot *activeRoomSlot, event protocol.EventMessage) []protocol.EventMessage {
	if slot == nil {
		return []protocol.EventMessage{event}
	}
	if slot.SuppressOutput {
		slot.PendingStream = nil
		return nil
	}
	if shouldHoldNoReplyCandidateStream(slot, event) {
		slot.PendingStream = append(slot.PendingStream, event)
		return nil
	}
	if len(slot.PendingStream) == 0 {
		return []protocol.EventMessage{event}
	}
	events := append([]protocol.EventMessage(nil), slot.PendingStream...)
	slot.PendingStream = nil
	events = append(events, event)
	return events
}

func shouldHoldNoReplyCandidateStream(slot *activeRoomSlot, event protocol.EventMessage) bool {
	if slot == nil || !slot.NoReplyCandidate {
		return false
	}
	if event.EventType != protocol.EventTypeStream {
		slot.NoReplyCandidate = false
		return false
	}
	if roomdomain.IsNoReplyCandidateStreamEvent(event) {
		return true
	}
	slot.NoReplyCandidate = false
	return false
}
