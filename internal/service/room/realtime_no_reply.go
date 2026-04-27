package room

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomNoReplyMarker = "<nexus_room_no_reply/>"

func isRoomNoReplyAssistantMessage(message protocol.Message) bool {
	if protocol.MessageRole(message) != "assistant" {
		return false
	}
	return strings.TrimSpace(extractHistoryText(message)) == roomNoReplyMarker
}

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
	if isNoReplyCandidateStreamEvent(event) {
		return true
	}
	slot.NoReplyCandidate = false
	return false
}

func isNoReplyCandidateStreamEvent(event protocol.EventMessage) bool {
	eventType := strings.TrimSpace(anyString(event.Data["type"]))
	switch eventType {
	case "message_start", "message_delta", "message_stop":
		return true
	case "content_block_stop":
		return true
	case "content_block_start", "content_block_delta":
		block, _ := event.Data["content_block"].(map[string]any)
		if strings.TrimSpace(anyString(block["type"])) != "text" {
			return false
		}
		text := strings.TrimSpace(anyString(block["text"]))
		return text == "" || strings.HasPrefix(roomNoReplyMarker, text)
	default:
		return false
	}
}
