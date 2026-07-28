package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
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

func TestOwnerHistoryOverlayCannotCrossWorkspaceOwner(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	ownerA := "user-history-a"
	ownerB := "user-history-b"
	workspaceA := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, ownerA), "agent-a")
	workspaceB := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, ownerB), "agent-b")
	if err := os.MkdirAll(workspaceA, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspaceB, 0o700); err != nil {
		t.Fatal(err)
	}

	history := NewAgentHistoryStore(appfs.UsersRoot()).ForOwner(ownerA)
	sessionKey := "agent:agent-a:ws:dm:owner-bound-overlay"
	message := protocol.Message{
		"message_id": "owner-a-message",
		"role":       "assistant",
		"round_id":   "round-a",
		"content":    "owner-a",
	}
	if err := history.AppendOverlayMessage(workspaceA, sessionKey, message); err != nil {
		t.Fatalf("owner A overlay 写入失败: %v", err)
	}

	if err := history.AppendOverlayMessage(workspaceB, sessionKey, message); err == nil {
		t.Fatal("owner-bound history 不应写入 owner B workspace")
	}
	if _, err := history.ReadMessages(workspaceB, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "agent-b",
	}, nil); err == nil {
		t.Fatal("owner-bound history 不应读取 owner B workspace")
	}
	if _, err := history.RemoveOverlayRounds(workspaceB, sessionKey, []string{"round-a"}); err == nil {
		t.Fatal("owner-bound history 不应替换 owner B overlay")
	}

	overlayPath := filepath.Join(
		workspaceB,
		".agents",
		"sessions",
		encodeSessionDirName(sessionKey),
		"overlay.jsonl",
	)
	if _, statErr := os.Stat(overlayPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("owner B overlay 被越权创建: path=%s err=%v", overlayPath, statErr)
	}
}
