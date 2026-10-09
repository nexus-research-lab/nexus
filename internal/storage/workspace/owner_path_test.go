package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

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
