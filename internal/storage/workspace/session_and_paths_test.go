// session_and_paths_test.go 覆盖会话文件、Owner 路径与工作区路径解析测试。
package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSessionLegacyPathCollisionFailsClosedAndSerializes(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	leftKey := "agent:test:ws:dm:user:one"
	rightKey := "agent:test:ws:dm:user_3aone"
	if encodeSessionDirName(leftKey) != encodeSessionDirName(rightKey) {
		t.Fatalf(
			"test keys no longer exercise the legacy collision: left=%q right=%q",
			encodeSessionDirName(leftKey),
			encodeSessionDirName(rightKey),
		)
	}
	now := time.Now().UTC()
	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: leftKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "left",
		CreatedAt: now, LastActivity: now,
	})
	if err != nil || created == nil {
		t.Fatalf("create colliding left session: item=%+v err=%v", created, err)
	}
	if _, _, err = store.FindSession([]string{workspacePath}, rightKey); !errors.Is(
		err,
		ErrSessionStorageIdentityMismatch,
	) {
		t.Fatalf("colliding alias read should fail closed: %v", err)
	}
	if _, err = store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: rightKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "right",
		CreatedAt: now, LastActivity: now,
	}); !errors.Is(err, ErrSessionStorageIdentityMismatch) {
		t.Fatalf("colliding alias write should fail closed: %v", err)
	}
	if deleted, deleteErr := store.DeleteSessionAtVersion(
		workspacePath,
		rightKey,
		created.ConfigurationVersion,
	); deleted || !errors.Is(deleteErr, ErrSessionStorageIdentityMismatch) {
		t.Fatalf("colliding alias delete deleted=%t err=%v", deleted, deleteErr)
	}
	current, _, err := store.FindSession([]string{workspacePath}, leftKey)
	if err != nil || current == nil || current.Title != "left" {
		t.Fatalf("real session changed through collision alias: item=%+v err=%v", current, err)
	}
}

func TestSessionDeletionLifecycleAbortRestoresWriterAdmission(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	sessionKey := "agent:test:ws:dm:delete-abort"
	now := time.Now().UTC()
	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "keep me",
		CreatedAt: now, LastActivity: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := store.BeginSessionDeletion(
		workspacePath,
		sessionKey,
		created.ConfigurationVersion,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AbortSessionDeletion(lease); err != nil {
		t.Fatal(err)
	}
	next := *created
	next.MessageCount++
	updated, err := store.UpsertSession(workspacePath, next)
	if err != nil || updated == nil || updated.MessageCount != 1 {
		t.Fatalf("abort did not restore writer admission: item=%+v err=%v", updated, err)
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

func TestWorkspacePathIsConfinedForOwnerAllowsDeletedWorkspace(t *testing.T) {
	stateRoot := t.TempDir()
	store := New(filepath.Join(stateRoot, "users"))
	store.StateRoot = stateRoot
	workspacePath := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-a"),
		"deleted-agent",
	)
	if !store.workspacePathIsConfinedForOwner("user-a", workspacePath) {
		t.Fatal("已删除 workspace 的历史 transcript 仍应按 owner 路径解析")
	}
}

const testRoomOwnerUserID = "user-room-test"

func TestTranscriptProjectHashSuffixMatchesBunHashFixtures(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: "27k1wwwhf13t"},
		{name: "ascii", input: "abc", expected: "1g45uqqks6lu"},
		{name: "unicode", input: "/Users/foo/my_project-测试", expected: "2a16ot6asyzsy"},
		{name: "emoji", input: strings.Repeat("😀", 101), expected: "1wlro20j1vo13"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := transcriptProjectHashSuffix(test.input); got != test.expected {
				t.Fatalf("transcriptProjectHashSuffix() = %q, want %q", got, test.expected)
			}
		})
	}
}
