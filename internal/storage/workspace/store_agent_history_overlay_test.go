package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAgentHistoryStoreRoomPublicCursorIsControlRow(t *testing.T) {
	root := t.TempDir()
	workspacePath := t.TempDir()
	sessionKey := "agent:devin:ws:group:conversation-1"
	store := NewAgentHistoryStore(root)

	if err := store.AppendOverlayMessage(workspacePath, sessionKey, protocol.Message{
		"message_id": "visible",
		"role":       "system",
		"content":    "普通 overlay",
		"timestamp":  int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRoomPublicCursor(workspacePath, sessionKey, RoomPublicCursor{
		RoomID:              "room-1",
		ConversationID:      "conversation-1",
		AgentID:             "devin",
		RoundID:             "round-1",
		LastPublicMessageID: "m4",
		LastPublicTimestamp: 4,
		Timestamp:           5,
	}); err != nil {
		t.Fatal(err)
	}

	cursor, ok, err := store.ReadRoomPublicCursor(workspacePath, sessionKey, "conversation-1", "devin")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cursor.LastPublicMessageID != "m4" || cursor.LastPublicTimestamp != 4 {
		t.Fatalf("cursor 读取不正确: ok=%v cursor=%+v", ok, cursor)
	}

	rows, err := store.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "devin",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row["nexus_overlay_kind"] == overlayKindRoomPublicCursor {
			t.Fatalf("公区 cursor 控制行不应进入普通 history: %+v", rows)
		}
	}
}
