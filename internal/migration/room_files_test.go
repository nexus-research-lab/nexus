package migration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

type fakeConversationOwnerLookup map[string]string

func (f fakeConversationOwnerLookup) LookupConversationOwnerUserID(
	_ context.Context,
	conversationID string,
) (string, error) {
	return f[conversationID], nil
}

func TestMigrateLegacyRoomFilesSplitsStateByOwner(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	legacyRoot := filepath.Join(stateRoot, "app", "rooms")
	if err := os.MkdirAll(filepath.Join(legacyRoot, "room-conversation-a"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacyRoot, "room-conversation-a", "overlay.jsonl"),
		[]byte("{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	legacyAttachment := filepath.Join(
		legacyRoot,
		"room-conversation-a",
		"attachments",
		"batch-a",
		"brief.txt",
	)
	if err := os.MkdirAll(filepath.Dir(legacyAttachment), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyAttachment, []byte("room attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyDirectory := filepath.Join(legacyRoot, "Y29udmVyc2F0aW9uLWxlZ2FjeQ")
	if err := os.MkdirAll(legacyDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacyDirectory, "messages.jsonl"),
		[]byte("{\"conversation_id\":\"conversation-legacy\",\"message_id\":\"legacy-message\",\"owner_user_id\":\"stale-owner\"}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	targetOverlay := filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "user-a"),
		"room-conversation-a",
		"overlay.jsonl",
	)
	if err := os.MkdirAll(filepath.Dir(targetOverlay), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		targetOverlay,
		[]byte(
			"{\"conversation_id\":\"conversation-a\",\"message_id\":\"existing\"}\n"+
				"{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n",
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	wakes := strings.Join([]string{
		`{"action":"schedule","wake":{"wake_id":"wake-a","owner_user_id":"stale-owner","message":{"conversation_id":"conversation-a"}}}`,
		`{"action":"complete","wake_id":"wake-a"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(legacyRoot, "directed_message_wakes.jsonl"), []byte(wakes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyRoot, "orphan.bin"), []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := migrateLegacyRoomFiles(
		context.Background(),
		stateRoot,
		legacyRoot,
		entries,
		fakeConversationOwnerLookup{
			"conversation-a":      "user-a",
			"conversation-legacy": "__system__",
		},
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.conversations != 2 || result.wakeEvents != 2 || result.quarantined != 1 {
		t.Fatalf("迁移统计不正确: %+v", result)
	}
	if _, err = os.Stat(legacyRoot); !os.IsNotExist(err) {
		t.Fatalf("旧 Room 根未移除: %v", err)
	}
	overlay, err := os.ReadFile(targetOverlay)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(overlay), `"message_id":"message-a"`) != 1 {
		t.Fatalf("重试合并产生重复消息: %s", overlay)
	}
	if !strings.Contains(string(overlay), `"message_id":"existing"`) {
		t.Fatalf("已有目标消息丢失: %s", overlay)
	}
	assetTarget := filepath.Join(
		appfs.UserRoomAssetsRootAt(stateRoot, "user-a"),
		"room-conversation-a",
		"attachments",
		"batch-a",
		"brief.txt",
	)
	if content, readErr := os.ReadFile(assetTarget); readErr != nil || string(content) != "room attachment" {
		t.Fatalf("Room 附件未迁入 owner workspace: content=%q err=%v", content, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "user-a"),
		"room-conversation-a",
		"attachments",
	)); !os.IsNotExist(statErr) {
		t.Fatalf("Room 附件不应留在宿主 ledger 根: %v", statErr)
	}
	legacyOverlay := filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "__system__"),
		"room-conversation-legacy",
		"overlay.jsonl",
	)
	if content, readErr := os.ReadFile(legacyOverlay); readErr != nil ||
		!strings.Contains(string(content), `"message_id":"legacy-message"`) {
		t.Fatalf("旧 messages.jsonl 未归并为 overlay: content=%q err=%v", content, readErr)
	} else if strings.Contains(string(content), "stale-owner") ||
		!strings.Contains(string(content), `"owner_user_id":"__system__"`) {
		t.Fatalf("Room ledger owner 未按数据库归属重写: %s", content)
	}
	wakeTarget := filepath.Join(appfs.UserRoomRootAt(stateRoot, "user-a"), "directed_message_wakes.jsonl")
	wakeContent, err := os.ReadFile(wakeTarget)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.TrimSpace(string(wakeContent)), "\n") != 1 {
		t.Fatalf("唤醒事件未按 owner 完整迁移: %s", wakeContent)
	}
	if strings.Contains(string(wakeContent), "stale-owner") ||
		!strings.Contains(string(wakeContent), `"owner_user_id":"user-a"`) {
		t.Fatalf("唤醒事件 owner 未按数据库归属重写: %s", wakeContent)
	}
	if _, statErr := os.Stat(filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "stale-owner"),
		"directed_message_wakes.jsonl",
	)); !os.IsNotExist(statErr) {
		t.Fatalf("旧 wake 中的 owner 不应覆盖数据库归属: %v", statErr)
	}
	quarantined := filepath.Join(stateRoot, "app", ".migration-quarantine", "room-state-v1", "orphan.bin")
	if content, readErr := os.ReadFile(quarantined); readErr != nil || string(content) != "orphan" {
		t.Fatalf("未知宿主文件未隔离保存: content=%q err=%v", content, readErr)
	}
}

func TestResolveLegacyRoomDirectoryOwnerSupportsBase64Name(t *testing.T) {
	root := t.TempDir()
	directoryName := "Y29udmVyc2F0aW9uLWxlZ2FjeQ"
	sourcePath := filepath.Join(root, directoryName)
	if err := os.MkdirAll(sourcePath, 0o700); err != nil {
		t.Fatal(err)
	}
	conversationID, ownerUserID, err := resolveLegacyRoomDirectoryOwner(
		context.Background(),
		sourcePath,
		directoryName,
		fakeConversationOwnerLookup{"conversation-legacy": "__system__"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if conversationID != "conversation-legacy" || ownerUserID != "__system__" {
		t.Fatalf("旧目录名解析失败: conversation=%q owner=%q", conversationID, ownerUserID)
	}
}

func TestMigrateLegacyRoomFilesQuarantinesMixedConversationDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	legacyRoot := filepath.Join(stateRoot, "app", "rooms")
	sourceRoot := filepath.Join(legacyRoot, "room-conversation-a")
	if err := os.MkdirAll(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`{"conversation_id":"conversation-a","message_id":"message-a"}`,
		`{"conversation_id":"conversation-b","message_id":"message-b"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(sourceRoot, "overlay.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := migrateLegacyRoomFiles(
		context.Background(),
		stateRoot,
		legacyRoot,
		entries,
		fakeConversationOwnerLookup{
			"conversation-a": "user-a",
			"conversation-b": "user-b",
		},
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.conversations != 0 || result.quarantined != 1 {
		t.Fatalf("混合归属目录应完整隔离: %+v", result)
	}
	quarantined := filepath.Join(
		stateRoot,
		"app",
		".migration-quarantine",
		"room-state-v1",
		"room-conversation-a",
		"overlay.jsonl",
	)
	if quarantinedContent, readErr := os.ReadFile(quarantined); readErr != nil ||
		string(quarantinedContent) != content {
		t.Fatalf("隔离目录内容不完整: content=%q err=%v", quarantinedContent, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "user-a"),
		"room-conversation-a",
	)); !os.IsNotExist(statErr) {
		t.Fatalf("混合归属目录不得产生部分目标状态: %v", statErr)
	}
}

func TestMigrateLegacyRoomFilesQuarantinesHardLinkedState(t *testing.T) {
	stateRoot := t.TempDir()
	legacyRoot := filepath.Join(stateRoot, "app", "rooms")
	sourceRoot := filepath.Join(legacyRoot, "room-conversation-a")
	if err := os.MkdirAll(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(sourceRoot, "overlay.jsonl")
	if err := os.WriteFile(
		overlayPath,
		[]byte("{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(overlayPath, filepath.Join(sourceRoot, "overlay-copy.jsonl")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := migrateLegacyRoomFiles(
		context.Background(),
		stateRoot,
		legacyRoot,
		entries,
		fakeConversationOwnerLookup{"conversation-a": "user-a"},
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.conversations != 0 || result.quarantined != 1 {
		t.Fatalf("硬链接状态应完整隔离: %+v", result)
	}
	quarantined := filepath.Join(
		stateRoot,
		"app",
		".migration-quarantine",
		"room-state-v1",
		"room-conversation-a",
	)
	firstInfo, err := os.Stat(filepath.Join(quarantined, "overlay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(filepath.Join(quarantined, "overlay-copy.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatal("隔离过程不应跟随或拆解硬链接")
	}
}
