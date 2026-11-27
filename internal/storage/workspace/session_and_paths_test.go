// session_and_paths_test.go 覆盖会话文件、Owner 路径与工作区路径解析测试。
package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestSessionConfigurationVersionCASSerializesConcurrentWriters(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	sessionKey := "agent:test:ws:dm:configuration-cas"
	now := time.Now().UTC()
	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "initial",
		CreatedAt: now, LastActivity: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ConfigurationVersion != 1 {
		t.Fatalf("created configuration_version=%d, want 1", created.ConfigurationVersion)
	}

	results := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for _, title := range []string{"left", "right"} {
		title := title
		go func() {
			start.Wait()
			next := *created
			next.Title = title
			_, updateErr := store.UpsertSessionAtVersion(
				workspacePath,
				next,
				created.ConfigurationVersion,
			)
			results <- updateErr
		}()
	}
	start.Done()
	successes := 0
	conflicts := 0
	for range 2 {
		switch updateErr := <-results; {
		case updateErr == nil:
			successes++
		case errors.Is(updateErr, ErrSessionConfigurationVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", updateErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent CAS results: successes=%d conflicts=%d", successes, conflicts)
	}
	current, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ConfigurationVersion != 2 {
		t.Fatalf("current Session after CAS=%+v", current)
	}

	// Runtime writers do not carry a configuration token. They merge only
	// runtime-owned fields and must not advance the stable configuration version.
	runtimeProjection := *created
	runtimeProjection.Title = "stale title that must not win"
	runtimeProjection.MessageCount = 1
	runtimeUpdated, err := store.PatchSessionRuntime(workspacePath, runtimeProjection)
	if err != nil {
		t.Fatalf("unversioned runtime writer should serialize, got %v", err)
	}
	if runtimeUpdated.ConfigurationVersion != 2 {
		t.Fatalf("runtime writer configuration_version=%d, want 2", runtimeUpdated.ConfigurationVersion)
	}
	if runtimeUpdated.Title != current.Title {
		t.Fatalf(
			"stale runtime writer rolled title back: got=%q want=%q",
			runtimeUpdated.Title,
			current.Title,
		)
	}

	if deleted, deleteErr := store.DeleteSessionAtVersion(
		workspacePath,
		sessionKey,
		created.ConfigurationVersion,
	); deleted || !errors.Is(deleteErr, ErrSessionConfigurationVersionConflict) {
		t.Fatalf("stale delete deleted=%t err=%v", deleted, deleteErr)
	}
	if deleted, deleteErr := store.DeleteSessionAtVersion(
		workspacePath,
		sessionKey,
		runtimeUpdated.ConfigurationVersion,
	); deleteErr != nil || !deleted {
		t.Fatalf("current delete deleted=%t err=%v", deleted, deleteErr)
	}
}

func TestPatchSessionRuntimeAtVersionRejectsNewerConfiguration(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	sessionKey := "agent:test:ws:dm:runtime-version-fence"
	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "test",
		Options:    map[string]any{"session_connector_ids": []string{"github"}},
	})
	if err != nil || created == nil {
		t.Fatalf("create Session: item=%+v err=%v", created, err)
	}
	staleRuntime := *created
	forkedSessionID := "forked-sdk-session"
	staleRuntime.SessionID = &forkedSessionID
	updatedConfiguration := *created
	updatedConfiguration.Options = map[string]any{
		"session_connector_ids": []string{"feishu-docx"},
	}
	current, err := store.UpsertSessionAtVersion(
		workspacePath,
		updatedConfiguration,
		created.ConfigurationVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.PatchSessionRuntimeAtVersion(
		workspacePath,
		staleRuntime,
		created.ConfigurationVersion,
	); !errors.Is(err, ErrSessionConfigurationVersionConflict) {
		t.Fatalf("stale runtime patch error=%v", err)
	}
	reloaded, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded == nil || reloaded.ConfigurationVersion != current.ConfigurationVersion ||
		reloaded.SessionID != nil {
		t.Fatalf("stale runtime patch changed current Session: %+v", reloaded)
	}
}

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

func TestListSessionsRejectsMetaStoredUnderWrongPhysicalDirectory(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	sessionKey := "agent:test:ws:dm:correct-directory"
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "wrong directory",
		CreatedAt: now, LastActivity: now,
	}); err != nil {
		t.Fatal(err)
	}
	paths := New(storeRoot)
	wrongKey := "agent:test:ws:dm:different-directory"
	if err := os.Rename(
		paths.SessionDir(workspacePath, sessionKey),
		paths.SessionDir(workspacePath, wrongKey),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListSessions(workspacePath); !errors.Is(
		err,
		ErrSessionStorageIdentityMismatch,
	) {
		t.Fatalf("ListSessions should reject wrong physical identity: %v", err)
	}
}

func TestOwnerSessionDeletionLifecycleUsesOwnerStateAndBlocksCollisionAlias(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	ownerUserID := "owner-session-delete"
	workspacePath := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, ownerUserID),
		"agent-delete",
	)
	store := NewSessionFileStore("").ForOwner(ownerUserID)
	leftKey := "agent:agent-delete:ws:dm:user:one"
	rightKey := "agent:agent-delete:ws:dm:user_3aone"
	now := time.Now().UTC()
	created, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: leftKey, AgentID: "agent-delete", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "delete collision",
		CreatedAt: now, LastActivity: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := store.BeginSessionDeletion(
		workspacePath,
		leftKey,
		created.ConfigurationVersion,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRoot := filepath.Join(
		appfs.UserStateRootAt(stateRoot, ownerUserID),
		"session-lifecycle",
	)
	entries, err := os.ReadDir(lifecycleRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("owner lifecycle entries=%d err=%v", len(entries), err)
	}
	lifecycleInfo, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	wantLifecycleMode := storageFileMode(0o600)
	if lifecycleInfo.Mode().Perm() != wantLifecycleMode {
		t.Fatalf("owner lifecycle mode=%#o, want %#o", lifecycleInfo.Mode().Perm(), wantLifecycleMode)
	}
	if _, err = os.Stat(filepath.Join(workspacePath, ".agents", "session_lifecycle")); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		t.Fatalf("workspace must not contain authoritative lifecycle state: %v", err)
	}
	if deleted, commitErr := store.CommitSessionDeletion(
		lease,
		created.ConfigurationVersion,
	); commitErr != nil || !deleted {
		t.Fatalf("commit deletion deleted=%t err=%v", deleted, commitErr)
	}
	if err = store.CompleteSessionDeletionCleanup(lease); err != nil {
		t.Fatal(err)
	}
	if _, err = store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: rightKey, AgentID: "agent-delete", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "collision takeover",
		CreatedAt: now, LastActivity: now,
	}); !errors.Is(err, ErrSessionStorageIdentityMismatch) {
		t.Fatalf("collision alias should not take over deleted directory: %v", err)
	}
}

func TestSessionArtifactDeletionPersistsFenceWithoutMeta(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	ownerUserID := "owner-artifact-delete"
	workspacePath := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, ownerUserID),
		"agent-artifact",
	)
	store := NewSessionFileStore("").ForOwner(ownerUserID)
	sessionKey := "agent:agent-artifact:automation:dm:scheduled-task:job:run"
	lease, version, err := store.BeginSessionArtifactDeletion(
		workspacePath,
		sessionKey,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if version != 0 {
		t.Fatalf("missing artifact configuration_version=%d, want 0", version)
	}
	if deleted, commitErr := store.CommitSessionDeletion(lease, version); commitErr != nil || !deleted {
		t.Fatalf("commit missing artifact deleted=%t err=%v", deleted, commitErr)
	}
	if err = store.CompleteSessionDeletionCleanup(lease); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err = store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: "agent-artifact", ChannelType: "automation",
		ChatType: "dm", Status: "closed", Title: "late artifact",
		CreatedAt: now, LastActivity: now,
	}); !errors.Is(err, ErrSessionDeleted) {
		t.Fatalf("missing artifact tombstone did not block late writer: %v", err)
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

func TestListSessionsReusesUnchangedMetaAndRereadsReplacedMeta(t *testing.T) {
	storeRoot := t.TempDir()
	workspacePath := filepath.Join(storeRoot, "owner", "workspace", "agent")
	store := NewSessionFileStore(storeRoot)
	sessionKey := "agent:test:ws:dm:meta-cache"
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "first",
		CreatedAt: now, LastActivity: now,
	}); err != nil {
		t.Fatal(err)
	}
	listTitle := func() string {
		t.Helper()
		items, err := store.ListSessions(workspacePath)
		if err != nil || len(items) != 1 {
			t.Fatalf("ListSessions items=%+v err=%v", items, err)
		}
		return items[0].Title
	}
	if title := listTitle(); title != "first" {
		t.Fatalf("title=%q", title)
	}

	// 命中缓存时每次重新解码：调用方改写 Options 不得污染下一次列表。
	items, err := store.ListSessions(workspacePath)
	if err != nil {
		t.Fatal(err)
	}
	items[0].Options["leak"] = true
	again, err := store.ListSessions(workspacePath)
	if err != nil || again[0].Options["leak"] != nil {
		t.Fatalf("cached Options leaked between calls: %+v err=%v", again, err)
	}

	if len(loadSessionMetaSnapshots(workspacePath)) != 1 && runtime.GOOS != "windows" {
		t.Fatal("unchanged meta should be cached after listing")
	}

	// 原子写入换新 inode。
	updated, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: "test", ChannelType: "websocket",
		ChatType: "dm", Status: "closed", Title: "second",
		CreatedAt: now, LastActivity: now,
	})
	if err != nil || updated.Title != "second" {
		t.Fatalf("upsert=%+v err=%v", updated, err)
	}
	if title := listTitle(); title != "second" {
		t.Fatalf("atomic replacement not observed: title=%q", title)
	}

	// 原地改写改变大小。
	metaPath := filepath.Join(New(storeRoot).SessionDir(workspacePath, sessionKey), "meta.json")
	payload, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := []byte(strings.Replace(string(payload), "\"second\"", "\"third-in-place\"", 1))
	if err := os.WriteFile(metaPath, rewritten, 0o600); err != nil {
		t.Fatal(err)
	}
	if title := listTitle(); title != "third-in-place" {
		t.Fatalf("in-place rewrite not observed: title=%q", title)
	}

	// meta 被换成符号链接时不得继续返回缓存内容，而是走 no-symlink 读取并失败。
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, rewritten, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(metaPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, metaPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := store.ListSessions(workspacePath); err == nil {
		t.Fatal("symlinked meta.json must not be served from cache")
	}
}

func TestWorkspacePathIsConfinedForOwnerRejectsPhysicalSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	store := New(filepath.Join(stateRoot, "users"))
	store.StateRoot = stateRoot

	ownerBWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-b"),
		"agent-b",
	)
	if err := os.MkdirAll(ownerBWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	ownerAWorkspaceRoot := appfs.UserWorkspaceRootAt(stateRoot, "user-a")
	if err := os.MkdirAll(ownerAWorkspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	ownerAWorkspace := filepath.Join(ownerAWorkspaceRoot, "agent-a")
	if err := os.Symlink(
		filepath.Join("..", "..", "user-b", "workspace", "agent-b"),
		ownerAWorkspace,
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if store.workspacePathIsConfinedForOwner("user-a", ownerAWorkspace) {
		t.Fatal("owner lexical 路径不能借 symlink 指向另一用户")
	}
	if err := os.WriteFile(filepath.Join(ownerBWorkspace, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, file, err := store.OpenOwnerWorkspaceFile(
		"user-a",
		ownerAWorkspace,
		"secret.txt",
	)
	if file != nil {
		_ = file.Close()
	}
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("owner workspace 文件不能借 symlink 跨用户读取: %v", err)
	}
}

func TestRemoveOwnerWorkspacePathDoesNotFollowSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	store := New(filepath.Join(stateRoot, "users"))
	store.StateRoot = stateRoot
	ownerBWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-b"),
		"agent-b",
	)
	if err := os.MkdirAll(ownerBWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	foreignFile := filepath.Join(ownerBWorkspace, "keep.txt")
	if err := os.WriteFile(foreignFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	ownerAWorkspaceRoot := appfs.UserWorkspaceRootAt(stateRoot, "user-a")
	if err := os.MkdirAll(ownerAWorkspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	ownerAWorkspace := filepath.Join(ownerAWorkspaceRoot, "agent-a")
	if err := os.Symlink(ownerBWorkspace, ownerAWorkspace); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if err := store.RemoveOwnerWorkspacePath("user-a", ownerAWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(ownerAWorkspace); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owner A 的链接目录项应被删除: %v", err)
	}
	if payload, err := os.ReadFile(foreignFile); err != nil || string(payload) != "keep" {
		t.Fatalf("owner B workspace 不能被递归删除: payload=%q err=%v", payload, err)
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

func TestStoreRoomConversationAssetDirUsesOwnerWorkspace(t *testing.T) {
	stateRoot := t.TempDir()
	store := New("")
	store.StateRoot = stateRoot
	conversationID := "conversation/assets"
	got := store.RoomConversationAssetDir(testRoomOwnerUserID, conversationID)
	want := filepath.Join(
		appfs.UserRoomAssetsRootAt(stateRoot, testRoomOwnerUserID),
		encodeConversationDirName(conversationID),
	)
	if got != want {
		t.Fatalf("RoomConversationAssetDir() = %q, want %q", got, want)
	}
}

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
