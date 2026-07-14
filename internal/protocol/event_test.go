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
	event := NewChatAckEvent("room:group:conversation-1", "request-1", "client-1", "round-1", "user-1", nil)

	isSnapshot, ok := event.Data["pending_snapshot"].(bool)
	if !ok || isSnapshot {
		t.Fatalf("pending_snapshot = %#v, want false", event.Data["pending_snapshot"])
	}
}
