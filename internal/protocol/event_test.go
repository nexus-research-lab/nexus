package protocol

import (
	"errors"
	"fmt"
	"testing"
)

type testClientMessageError struct{}

func (testClientMessageError) Error() string {
	return "internal detail"
}

func (testClientMessageError) ClientMessage() string {
	return "可安全展示的错误"
}

func TestClientErrorMessageOnlyReadsDeclaredMessage(t *testing.T) {
	message, ok := ClientErrorMessage(fmt.Errorf("wrapped: %w", testClientMessageError{}))
	if !ok || message != "可安全展示的错误" {
		t.Fatalf("ClientErrorMessage() = %q, %v", message, ok)
	}
	if message, ok = ClientErrorMessage(errors.New("database secret")); ok || message != "" {
		t.Fatalf("内部错误不应穿透到客户端: %q, %v", message, ok)
	}
}

func TestChatPendingSnapshotEventMarksEmptySnapshotAsAuthoritative(t *testing.T) {
	event := NewChatPendingSnapshotEvent("room:group:conversation-1", "", nil)

	isSnapshot, ok := event.Data["pending_snapshot"].(bool)
	if !ok || !isSnapshot {
		t.Fatalf("pending_snapshot = %#v, want true", event.Data["pending_snapshot"])
	}
	pending, ok := event.Data["pending"].([]ChatAckPendingSlot)
	if !ok || len(pending) != 0 {
		t.Fatalf("pending = %#v, want authoritative empty slice", event.Data["pending"])
	}
}

func TestChatAckEventDoesNotReplaceUnrelatedPendingSlots(t *testing.T) {
	event := NewChatAckEvent("room:group:conversation-1", "request-1", "client-1", "round-1", "user-1", true, nil)

	isSnapshot, ok := event.Data["pending_snapshot"].(bool)
	if !ok || isSnapshot {
		t.Fatalf("pending_snapshot = %#v, want false", event.Data["pending_snapshot"])
	}
	if committed, ok := event.Data["user_message_committed"].(bool); !ok || !committed {
		t.Fatalf("user_message_committed = %#v, want true", event.Data["user_message_committed"])
	}
}

func TestInputQueueAckEventConfirmsDurableAcceptance(t *testing.T) {
	event := NewInputQueueAckEvent(
		"room:group:conversation-1",
		"request-queue-1",
		"client-queue-1",
		InputQueueMutationResult{
			Action:    " enqueue ",
			ItemID:    " queue-item-1 ",
			Duplicate: true,
		},
	)

	if event.EventType != EventTypeInputQueueAck || event.DeliveryMode != "ephemeral" {
		t.Fatalf("unexpected ack envelope: %+v", event)
	}
	if event.SessionKey != "room:group:conversation-1" {
		t.Fatalf("session_key = %q", event.SessionKey)
	}
	for key, want := range map[string]any{
		"accepted":          true,
		"duplicate":         true,
		"action":            "enqueue",
		"item_id":           "queue-item-1",
		"client_request_id": "request-queue-1",
		"client_message_id": "client-queue-1",
		"ack_timeout_ms":    RequestAckTimeoutMS,
	} {
		if got := event.Data[key]; got != want {
			t.Fatalf("data[%q] = %#v, want %#v", key, got, want)
		}
	}
	if ChatAckTimeoutMS != RequestAckTimeoutMS {
		t.Fatalf("chat ack alias drifted: chat=%d request=%d", ChatAckTimeoutMS, RequestAckTimeoutMS)
	}
}
