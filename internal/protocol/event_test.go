package protocol

import "testing"

func TestChatPendingSnapshotCarriesRoomReplayFence(t *testing.T) {
	event := NewChatPendingSnapshotEvent(
		"room:session",
		"round-root",
		42,
		[]ChatAckPendingSlot{},
		[]string{},
	)
	if event.Data["pending_snapshot"] != true {
		t.Fatalf("pending snapshot marker missing: %+v", event.Data)
	}
	if event.Data["snapshot_room_seq"] != int64(42) {
		t.Fatalf("snapshot_room_seq = %#v, want 42", event.Data["snapshot_room_seq"])
	}

	clamped := NewChatPendingSnapshotEvent("", "", -1, nil, nil)
	if clamped.Data["snapshot_room_seq"] != int64(0) {
		t.Fatalf("negative snapshot_room_seq must clamp to zero: %+v", clamped.Data)
	}
}
