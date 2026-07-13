package protocol

import "testing"

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
