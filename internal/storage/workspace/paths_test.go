package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const testRoomOwnerUserID = "user-room-test"

func TestStoreSessionDirUsesRoomConversationIDName(t *testing.T) {
	store := New(t.TempDir())
	workspacePath := filepath.Join(t.TempDir(), "workspace", "agent-c5740009ac97")
	sessionKey := protocol.BuildRoomAgentSessionKey(
		"743295d46e5841dea378d604d7e45431",
		"c5740009ac97",
		"group",
	)

	name := filepath.Base(store.SessionDir(workspacePath, sessionKey))
	if name != "room-743295d46e5841dea378d604d7e45431" {
		t.Fatalf("room 私有 session 目录不正确: %s", name)
	}
}

func TestStoreSessionDirUsesDMChannelAndRefName(t *testing.T) {
	store := New(t.TempDir())
	workspacePath := filepath.Join(t.TempDir(), "workspace", "agent-c5740009ac97")
	sessionKey := protocol.BuildAgentSessionKey(
		"c5740009ac97",
		"ws",
		"dm",
		"launcher-app-c5740009ac97",
		"",
	)

	name := filepath.Base(store.SessionDir(workspacePath, sessionKey))
	if name != "dm-ws-launcher-app-c5740009ac97" {
		t.Fatalf("dm session 目录不正确: %s", name)
	}
}

func TestStoreRoomConversationDirUsesConversationIDName(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	store := New(root)
	conversationID := "743295d46e5841dea378d604d7e45431"

	name := filepath.Base(store.RoomConversationDir(testRoomOwnerUserID, conversationID))
	if name != "room-743295d46e5841dea378d604d7e45431" {
		t.Fatalf("room 共享目录不正确: %s", name)
	}
}

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

func TestStoreUsesAppHostRoot(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	store := New("")
	if store.HomeRoot != filepath.Join(stateRoot, "app") {
		t.Fatalf("宿主存储根不正确: got=%q", store.HomeRoot)
	}
	if store.WorkspaceRoot != filepath.Join(stateRoot, "users") {
		t.Fatalf("workspace 默认根不正确: got=%q", store.WorkspaceRoot)
	}
	if got := store.RoomConversationRoot(testRoomOwnerUserID); got != filepath.Join(
		stateRoot,
		"users",
		testRoomOwnerUserID,
		"state",
		"rooms",
	) {
		t.Fatalf("Room 状态根不正确: got=%q", got)
	}

	customWorkspace := filepath.Join(t.TempDir(), "custom-workspace")
	customStore := New(customWorkspace)
	if customStore.HomeRoot != filepath.Join(stateRoot, "app") {
		t.Fatalf("自定义 workspace 不应改变宿主存储根: got=%q", customStore.HomeRoot)
	}
	if customStore.WorkspaceRoot != customWorkspace {
		t.Fatalf("自定义 workspace 根不正确: got=%q", customStore.WorkspaceRoot)
	}
}

func TestTranscriptProjectsDirUsesWorkspaceOwner(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	workspacePath := filepath.Join(stateRoot, "users", "user_demo", "workspace", "agent-a")

	got := transcriptProjectsDirForWorkspace(workspacePath)
	want := filepath.Join(stateRoot, "users", "user_demo", "runtime", "projects")
	if got != want {
		t.Fatalf("用户 transcript 根不正确: got=%q want=%q", got, want)
	}
}

func TestTranscriptProjectsDirFallsBackToSystemRuntimeAfterMigration(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)

	systemProjectsRoot := filepath.Join(
		stateRoot,
		"users",
		"__system__",
		"runtime",
		"projects",
	)
	if err := os.MkdirAll(systemProjectsRoot, 0o700); err != nil {
		t.Fatalf("创建 system transcript 根失败: %v", err)
	}

	got := transcriptProjectsDir()
	if got != systemProjectsRoot {
		t.Fatalf("迁移后的全局 transcript 回退根不正确: got=%q want=%q", got, systemProjectsRoot)
	}
}

func TestSanitizeTranscriptPathMatchesClaudeCodeProjectDirectory(t *testing.T) {
	if got := sanitizeTranscriptPath("/Users/foo/my_project-测试"); got != "-Users-foo-my-project---" {
		t.Fatalf("sanitizeTranscriptPath() = %q, want Claude Code ASCII replacement", got)
	}

	longPath := strings.Repeat("a", maxTranscriptSanitizedLength+1)
	expected := strings.Repeat("a", maxTranscriptSanitizedLength) + "-2lljc4d1ph1qx"
	if got := sanitizeTranscriptPath(longPath); got != expected {
		t.Fatalf("sanitizeTranscriptPath() = %q, want %q", got, expected)
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
