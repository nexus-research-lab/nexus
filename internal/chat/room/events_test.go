package room

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestWrapRoundStatusErrorEventCarriesRoomIdentity(t *testing.T) {
	event := WrapRoundStatusErrorEvent(
		"room:group:conversation-1",
		"room-1",
		"conversation-1",
		"round-1",
		"provider unavailable",
	)

	if event.Data["status"] != "error" || event.Data["message"] != "provider unavailable" {
		t.Fatalf("error round status data = %#v", event.Data)
	}
	if event.DeliveryMode != "durable" || event.RoomID != "room-1" || event.ConversationID != "conversation-1" {
		t.Fatalf("error round status identity = %+v", event)
	}
}

func TestServerPendingSlotsEventIsDurableWhileClientAckIsEphemeral(
	t *testing.T,
) {
	pending := []protocol.ChatAckPendingSlot{{
		AgentID:      "agent-1",
		AgentRoundID: "agent-round-1",
		MsgID:        "slot-1",
		RoundID:      "root-1",
		Status:       "pending",
		Timestamp:    1,
	}}
	serverEvent := WrapServerPendingSlotsEvent(
		"room:group:conversation-1",
		"room-1",
		"conversation-1",
		"root-1",
		pending,
	)
	if serverEvent.DeliveryMode != "durable" {
		t.Fatalf("server pending delivery = %q, want durable", serverEvent.DeliveryMode)
	}

	clientEvent := WrapChatAckEvent(
		"room:group:conversation-1",
		"room-1",
		"conversation-1",
		"request-1",
		"client-message-1",
		"root-1",
		"user-message-1",
		true,
		pending,
	)
	if clientEvent.DeliveryMode != "ephemeral" {
		t.Fatalf("client ACK delivery = %q, want ephemeral", clientEvent.DeliveryMode)
	}
}
