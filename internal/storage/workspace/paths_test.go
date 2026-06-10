package workspace

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	store := New(root)
	conversationID := "743295d46e5841dea378d604d7e45431"

	name := filepath.Base(store.RoomConversationDir(conversationID))
	if name != "room-743295d46e5841dea378d604d7e45431" {
		t.Fatalf("room 共享目录不正确: %s", name)
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
