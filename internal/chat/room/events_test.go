package room

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

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
	if event.Data["failure_code"] != protocol.ConversationFailureRoundFailed {
		t.Fatalf("error round status failure_code = %#v", event.Data["failure_code"])
	}
	if event.DeliveryMode != "durable" || event.RoomID != "room-1" || event.ConversationID != "conversation-1" {
		t.Fatalf("error round status identity = %+v", event)
	}
}

func TestNewErrorEventClassifiesRoomAndRequestFailures(t *testing.T) {
	roomFailure := NewErrorEvent("session-1", "room-1", "conversation-1", "room_error", "failed", "round-1")
	if roomFailure.Data["failure_code"] != protocol.ConversationFailureRoundFailed {
		t.Fatalf("room failure_code = %#v", roomFailure.Data["failure_code"])
	}
	requestFailure := NewErrorEvent("session-1", "room-1", "conversation-1", "input_queue_error", "failed", "queue-1")
	if requestFailure.Data["failure_code"] != protocol.ConversationFailureRequestRejected {
		t.Fatalf("request failure_code = %#v", requestFailure.Data["failure_code"])
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

func TestSlotMessageMapperAddsRoomContextWithoutDMStreamLifecycle(t *testing.T) {
	mapper := NewSlotMessageMapper(
		"chat:conversation:shared:test",
		"room-1",
		"conversation-1",
		"agent-1",
		"slot-1",
		"round-1",
		"agent-round-1",
	)
	events, messages, status, err := mapper.Map(sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    "assistant-1",
				"model": "test-model",
			},
		}},
	})
	if err != nil {
		t.Fatalf("映射 Room 流事件失败: %v", err)
	}
	if len(messages) != 0 || status != "" {
		t.Fatalf("流开始不应产生持久消息或终态: messages=%+v status=%q", messages, status)
	}
	if len(events) != 1 || events[0].EventType != protocol.EventTypeStream {
		t.Fatalf("Room 不应补充 DM stream_start: %+v", events)
	}
	event := events[0]
	if event.SessionKey != "chat:conversation:shared:test" ||
		event.RoomID != "room-1" ||
		event.ConversationID != "conversation-1" ||
		event.AgentID != "agent-1" ||
		event.RoundID != "round-1" ||
		event.AgentRoundID != "agent-round-1" ||
		event.MessageID != "assistant-1" {
		t.Fatalf("Room 事件身份不完整: %+v", event)
	}
}
