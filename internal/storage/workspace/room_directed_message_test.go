package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomDirectedMessageStoreRestoresGoalCollaborationBinding(t *testing.T) {
	root := t.TempDir()
	store := NewRoomDirectedMessageStore(root)
	store.paths.StateRoot = root
	message := protocol.RoomDirectedMessageRecord{
		MessageID:      "directed-goal-binding",
		RoomID:         "room-goal-binding",
		ConversationID: "conversation-goal-binding",
		SourceAgentID:  "agent-lead",
		Recipients:     []string{"agent-peer"},
		Content:        "complete the private check",
		WakePolicy:     protocol.RoomWakePolicyImmediate,
		GoalCollaborationBinding: &protocol.GoalCollaborationBinding{
			GoalID:            "goal-room",
			ObjectiveRevision: 5,
		},
	}
	if err := store.AppendMessage(testRoomOwnerUserID, message); err != nil {
		t.Fatal(err)
	}
	reloaded := NewRoomDirectedMessageStore(root)
	reloaded.paths.StateRoot = root
	rows, err := reloaded.ReadMessages(
		testRoomOwnerUserID,
		message.ConversationID,
	)
	if err != nil || len(rows) != 1 || rows[0].GoalCollaborationBinding == nil ||
		rows[0].GoalCollaborationBinding.GoalID != "goal-room" ||
		rows[0].GoalCollaborationBinding.ObjectiveRevision != 5 {
		t.Fatalf("Goal collaboration binding was not restored: rows=%+v err=%v", rows, err)
	}
	recoveryRows, err := reloaded.GoalCollaborationMessagesAll()
	if err != nil || len(recoveryRows) != 1 ||
		recoveryRows[0].OwnerUserID != testRoomOwnerUserID ||
		recoveryRows[0].Message.MessageID != message.MessageID {
		t.Fatalf("Goal directed-message recovery scan = %+v err=%v", recoveryRows, err)
	}
}

func TestRoomDirectedMessageStoreDeduplicatesStableMessageID(t *testing.T) {
	root := t.TempDir()
	store := NewRoomDirectedMessageStore(root)
	store.paths.StateRoot = root
	message := protocol.RoomDirectedMessageRecord{
		MessageID: "stable-directed-message", RoomID: "room-idempotent",
		ConversationID: "conversation-idempotent", SourceAgentID: "agent-source",
		Recipients: []string{"agent-target"}, Content: "deliver once",
		ReplyRoute: protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone},
		Timestamp:  100,
	}
	first, inserted, err := store.AppendMessageIfAbsent(testRoomOwnerUserID, message)
	if err != nil || !inserted || first.MessageID != message.MessageID {
		t.Fatalf("first append = %+v inserted=%t err=%v", first, inserted, err)
	}
	retry := message
	retry.Timestamp = 200
	replayed, inserted, err := store.AppendMessageIfAbsent(testRoomOwnerUserID, retry)
	if err != nil || inserted || replayed.Timestamp != 100 {
		t.Fatalf("retry append = %+v inserted=%t err=%v", replayed, inserted, err)
	}
	rows, err := store.ReadMessages(testRoomOwnerUserID, message.ConversationID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("deduplicated rows = %+v err=%v", rows, err)
	}
	conflict := message
	conflict.Content = "different payload"
	if _, _, err = store.AppendMessageIfAbsent(testRoomOwnerUserID, conflict); err == nil {
		t.Fatal("same message id with different intent must fail closed")
	}
}
