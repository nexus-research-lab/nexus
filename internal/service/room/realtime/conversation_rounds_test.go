package realtime

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGetActiveRoundSnapshotKeepsPerSlotRootAcrossConcurrentRounds(
	t *testing.T,
) {
	const conversationID = "conversation-multi-root-snapshot"
	firstSlot := &activeRoomSlot{
		AgentID:      "agent-a",
		AgentRoundID: "agent-round-a",
		MsgID:        "slot-a",
		TimestampMS:  10,
	}
	firstSlot.setStatus("running")
	firstSlot.setDeliveryMetadata(
		protocol.RoomReplyRoute{},
		"source-message-a",
		"handoff-a",
	)
	secondSlot := &activeRoomSlot{
		AgentID:      "agent-b",
		AgentRoundID: "agent-round-b",
		MsgID:        "slot-b",
		TimestampMS:  20,
	}
	secondSlot.setStatus("pending")

	service := &Service{
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"runtime-round-a": {
				SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
				RoomID:         "room-multi-root-snapshot",
				ConversationID: conversationID,
				RoundID:        "runtime-round-a",
				RootRoundID:    "root-a",
				Slots:          map[string]*activeRoomSlot{firstSlot.MsgID: firstSlot},
			},
			"runtime-round-b": {
				SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
				RoomID:         "room-multi-root-snapshot",
				ConversationID: conversationID,
				RoundID:        "runtime-round-b",
				RootRoundID:    "root-b",
				Slots:          map[string]*activeRoomSlot{secondSlot.MsgID: secondSlot},
			},
		}),
	}

	snapshot := service.GetActiveRoundSnapshot(conversationID)
	if snapshot == nil {
		t.Fatal("GetActiveRoundSnapshot() = nil")
	}
	if snapshot.RoundID != "" {
		t.Fatalf("aggregate RoundID = %q, want empty for multiple roots", snapshot.RoundID)
	}
	rootsByMessageID := make(map[string]string, len(snapshot.Pending))
	handoffsByMessageID := make(map[string]string, len(snapshot.Pending))
	for _, slot := range snapshot.Pending {
		rootsByMessageID[slot.MsgID] = slot.RoundID
		handoffsByMessageID[slot.MsgID] = slot.HandoffID
	}
	if rootsByMessageID[firstSlot.MsgID] != "root-a" {
		t.Fatalf("slot-a root = %q, want root-a", rootsByMessageID[firstSlot.MsgID])
	}
	if rootsByMessageID[secondSlot.MsgID] != "root-b" {
		t.Fatalf("slot-b root = %q, want root-b", rootsByMessageID[secondSlot.MsgID])
	}
	if handoffsByMessageID[firstSlot.MsgID] != "handoff-a" {
		t.Fatalf(
			"slot-a handoff = %q, want handoff-a",
			handoffsByMessageID[firstSlot.MsgID],
		)
	}
	if handoffsByMessageID[secondSlot.MsgID] != "" {
		t.Fatalf(
			"ordinary slot handoff = %q, want empty",
			handoffsByMessageID[secondSlot.MsgID],
		)
	}
}
