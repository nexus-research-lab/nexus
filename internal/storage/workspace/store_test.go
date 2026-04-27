package workspace

import (
	"path/filepath"
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
