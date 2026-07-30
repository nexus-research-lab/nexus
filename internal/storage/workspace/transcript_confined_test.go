package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func TestOpenTranscriptPathRejectsIntermediateSymlink(t *testing.T) {
	workspacePath := t.TempDir()
	privateDir := filepath.Join(workspacePath, "private")
	if err := os.Mkdir(privateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(privateDir, "session.jsonl")
	if err := os.WriteFile(transcriptPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("private", filepath.Join(workspacePath, "transcripts")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	root, _, _, err := openTranscriptPath(
		workspacePath,
		filepath.Join(workspacePath, "transcripts", "session.jsonl"),
	)
	if root != nil {
		root.Close()
	}
	if !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("transcript 中间 symlink 应被拒绝: %v", err)
	}
}

func TestOwnerTranscriptReadRejectsCrossOwnerWorkspaceSymlink(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)

	ownerAWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-a"),
		"agent-a",
	)
	ownerBWorkspace := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, "user-b"),
		"agent-b",
	)
	if err := os.MkdirAll(filepath.Dir(ownerAWorkspace), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ownerBWorkspace, 0o700); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(ownerBWorkspace, "task.jsonl")
	if err := os.WriteFile(transcriptPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ownerBWorkspace, ownerAWorkspace); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	store := NewAgentHistoryStore(appfs.UsersRoot())
	messages, err := store.ReadTranscriptPathMessagesForOwner(
		"user-a",
		filepath.Join(ownerAWorkspace, "task.jsonl"),
		ownerAWorkspace,
		"agent:agent-a:ws:dm:test",
		"agent-a",
	)
	if len(messages) != 0 || !errors.Is(err, confinedfs.ErrSymlink) {
		t.Fatalf("跨 owner transcript symlink 应被拒绝: messages=%+v err=%v", messages, err)
	}
}
