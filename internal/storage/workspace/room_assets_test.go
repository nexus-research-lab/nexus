package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func TestEnsureRoomConversationAssetDirRejectsWorkspaceSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	store := New(filepath.Join(stateRoot, "users"))
	workspaceRoot := appfs.UserWorkspaceRootAt(stateRoot, "user-a")
	agentRoot := filepath.Join(workspaceRoot, "agent-a")
	if err := os.MkdirAll(agentRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("agent-a", filepath.Join(workspaceRoot, ".rooms")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := store.EnsureRoomConversationAssetDir("user-a", "conversation-a")
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("Room 资产根 symlink 应被拒绝: %v", err)
	}
	redirected := filepath.Join(
		agentRoot,
		filepath.Base(store.RoomConversationAssetDir("user-a", "conversation-a")),
	)
	if _, statErr := os.Stat(redirected); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("不能借 .rooms symlink 写入 agent workspace: %v", statErr)
	}
}

func TestOpenRoomConversationAssetFileRejectsCrossOwnerSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	store := New(filepath.Join(stateRoot, "users"))
	ownerBAssetRoot := store.RoomConversationAssetDir("user-b", "conversation-a")
	if err := os.MkdirAll(filepath.Join(ownerBAssetRoot, "attachments"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(ownerBAssetRoot, "attachments", "secret.txt"),
		[]byte("secret"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	ownerAWorkspaceRoot := appfs.UserWorkspaceRootAt(stateRoot, "user-a")
	if err := os.MkdirAll(ownerAWorkspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(
		filepath.Join("..", "..", "user-b", "workspace", ".rooms"),
		filepath.Join(ownerAWorkspaceRoot, ".rooms"),
	); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, file, err := store.OpenRoomConversationAssetFile(
		"user-a",
		"conversation-a",
		"attachments/secret.txt",
	)
	if file != nil {
		_ = file.Close()
	}
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("Room 附件不能借 .rooms symlink 读取另一 owner 文件: %v", err)
	}
}
