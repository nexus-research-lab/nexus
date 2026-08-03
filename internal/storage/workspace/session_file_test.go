package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestUpsertSessionCreatesMissingWorkspace(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	now := time.Now().UTC()

	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   "agent:test:websocket:dm:user",
		AgentID:      "test",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		ContextUsage: &protocol.ContextUsageData{
			TotalTokens: 37_500,
			MaxTokens:   131_100,
			Percentage:  28.6,
			Model:       "glm-4.5-air",
		},
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("UpsertSession() error = %v", err)
	}
	if created == nil || created.SessionKey != "agent:test:websocket:dm:user" {
		t.Fatalf("UpsertSession() created = %+v", created)
	}
	if created.ContextUsage == nil ||
		created.ContextUsage.TotalTokens != 37_500 ||
		created.ContextUsage.MaxTokens != 131_100 ||
		created.ContextUsage.Percentage != 28.6 ||
		created.ContextUsage.Model != "glm-4.5-air" {
		t.Fatalf("UpsertSession() context_usage = %+v", created.ContextUsage)
	}
}

func TestUpsertSessionRejectsSymlinkedWorkspaceParent(t *testing.T) {
	storeRoot := t.TempDir()
	outsideRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(storeRoot, "owner"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(storeRoot, "owner", "workspace")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	store := NewSessionFileStore(storeRoot)
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")

	_, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: "agent:test:websocket:dm:user",
	})
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("UpsertSession() error = %v, want ErrSymlink", err)
	}
	if _, statErr := os.Stat(filepath.Join(outsideRoot, "agent")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("session 写入逃逸到 workspace 外: %v", statErr)
	}
}

func TestOwnerSessionStoreRejectsForeignWorkspace(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	foreignWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-b"),
		"agent-b",
	)
	if err := os.MkdirAll(foreignWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}

	store := NewSessionFileStore(appfs.UsersRoot()).ForOwner("user-a")
	_, err := store.UpsertSession(foreignWorkspace, protocol.Session{
		SessionKey: "agent:agent-b:websocket:dm:user",
	})
	if err == nil {
		t.Fatal("owner-bound session store 不应写入其他用户 workspace")
	}
	if _, statErr := os.Stat(filepath.Join(foreignWorkspace, ".agents")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("foreign workspace 被写入: %v", statErr)
	}
}

func TestDeleteRoomConversationRemovesLedgerAndAssets(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	ownerUserID := "user-room-delete"
	conversationID := "conversation-delete"

	history := NewRoomHistoryStore("")
	if err := history.AppendInlineMessage(ownerUserID, conversationID, protocol.Message{
		"message_id":      "message-delete",
		"conversation_id": conversationID,
		"content":         "delete",
	}); err != nil {
		t.Fatal(err)
	}
	paths := New("")
	assetRoot, err := paths.EnsureRoomConversationAssetDir(ownerUserID, conversationID)
	if err != nil {
		t.Fatal(err)
	}
	assetPath := filepath.Join(assetRoot, "attachments", "delete.txt")
	if err = os.MkdirAll(filepath.Dir(assetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(assetPath, []byte("delete"), 0o600); err != nil {
		t.Fatal(err)
	}

	deleted, err := NewSessionFileStore("").DeleteRoomConversation(ownerUserID, conversationID)
	if err != nil || !deleted {
		t.Fatalf("DeleteRoomConversation() deleted=%v err=%v", deleted, err)
	}
	for _, path := range []string{
		paths.RoomConversationDir(ownerUserID, conversationID),
		paths.RoomConversationAssetDir(ownerUserID, conversationID),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("Room 文件未删除: path=%s err=%v", path, statErr)
		}
	}
}
