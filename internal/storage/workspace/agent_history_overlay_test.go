package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestOwnerHistoryOverlayCannotCrossWorkspaceOwner(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")

	ownerA := "user-history-a"
	ownerB := "user-history-b"
	workspaceA := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, ownerA), "agent-a")
	workspaceB := filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, ownerB), "agent-b")
	if err := os.MkdirAll(workspaceA, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workspaceB, 0o700); err != nil {
		t.Fatal(err)
	}

	history := NewAgentHistoryStore(appfs.UsersRoot()).ForOwner(ownerA)
	sessionKey := "agent:agent-a:ws:dm:owner-bound-overlay"
	message := protocol.Message{
		"message_id": "owner-a-message",
		"role":       "assistant",
		"round_id":   "round-a",
		"content":    "owner-a",
	}
	if err := history.AppendOverlayMessage(workspaceA, sessionKey, message); err != nil {
		t.Fatalf("owner A overlay 写入失败: %v", err)
	}

	if err := history.AppendOverlayMessage(workspaceB, sessionKey, message); err == nil {
		t.Fatal("owner-bound history 不应写入 owner B workspace")
	}
	if _, err := history.ReadMessages(workspaceB, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    "agent-b",
	}, nil); err == nil {
		t.Fatal("owner-bound history 不应读取 owner B workspace")
	}
	if _, err := history.RemoveOverlayRounds(workspaceB, sessionKey, []string{"round-a"}); err == nil {
		t.Fatal("owner-bound history 不应替换 owner B overlay")
	}

	overlayPath := filepath.Join(
		workspaceB,
		".agents",
		"sessions",
		encodeSessionDirName(sessionKey),
		"overlay.jsonl",
	)
	if _, statErr := os.Stat(overlayPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("owner B overlay 被越权创建: path=%s err=%v", overlayPath, statErr)
	}
}
