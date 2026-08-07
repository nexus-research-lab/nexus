package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func TestDeleteTranscriptSessionRemovesOwnedArtifactGraph(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	ownerUserID := "user-transcript-delete"
	workspacePath := filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, ownerUserID),
		"agent-a",
	)
	if err := os.MkdirAll(workspacePath, 0o700); err != nil {
		t.Fatal(err)
	}

	const (
		sessionID       = "550e8400-e29b-41d4-a716-446655440000"
		otherSessionID  = "650e8400-e29b-41d4-a716-446655440000"
		sharedChild     = "agent-11111111111111111111111111111111"
		ownedChild      = "agent-22222222222222222222222222222222"
		ownedGrandchild = "agent-33333333333333333333333333333333"
	)
	projectsRoot := filepath.Join(
		appfs.UserRuntimeRootAt(stateRoot, ownerUserID),
		"projects",
	)
	projectDir := filepath.Join(projectsRoot, TranscriptProjectDirectoryName(workspacePath))

	writeTranscriptArtifactRows(t, filepath.Join(projectDir, sessionID+".jsonl"), []map[string]any{
		structuredSubagentReference(sharedChild),
		structuredSubagentReference(ownedChild),
	})
	writeTranscriptArtifactRows(t, filepath.Join(projectDir, otherSessionID+".jsonl"), []map[string]any{
		structuredSubagentReference(sharedChild),
	})
	writeTranscriptArtifactRows(t, filepath.Join(projectDir, sharedChild+".jsonl"), []map[string]any{
		{"type": "assistant", "uuid": "shared-child"},
	})
	writeTranscriptArtifactRows(t, filepath.Join(projectDir, ownedChild+".jsonl"), []map[string]any{
		structuredSubagentReference(ownedGrandchild),
	})
	writeTranscriptArtifactRows(t, filepath.Join(projectDir, ownedGrandchild+".jsonl"), []map[string]any{
		{"type": "assistant", "uuid": "owned-grandchild"},
	})

	sessionDir := filepath.Join(projectDir, sessionID)
	if err := os.MkdirAll(filepath.Join(sessionDir, "session-memory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(sessionDir, "session-memory", "summary.md"),
		[]byte("summary\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sessionDir, "subagents"), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(map[string]any{
		"task": map[string]any{"child_session_id": ownedChild},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(
		filepath.Join(sessionDir, "subagents", "agent-owned.meta.json"),
		metadata,
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	memoryTranscriptName := "session-memory-" + sessionID + ".jsonl"
	writeTranscriptArtifactRows(t, filepath.Join(projectDir, memoryTranscriptName), []map[string]any{
		{"type": "assistant", "uuid": "session-memory"},
	})
	memoryWorkspacePath := filepath.Join(projectDir, sessionID, "session-memory")
	memoryProjectDir := filepath.Join(
		projectsRoot,
		TranscriptProjectDirectoryName(memoryWorkspacePath),
	)
	writeTranscriptArtifactRows(t, filepath.Join(memoryProjectDir, memoryTranscriptName), []map[string]any{
		{"type": "assistant", "uuid": "nested-session-memory"},
	})

	legacyProjectDir := filepath.Join(projectsRoot, "legacy-project-layout")
	if err = os.MkdirAll(filepath.Join(legacyProjectDir, sessionID), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTranscriptArtifactRows(t, filepath.Join(legacyProjectDir, memoryTranscriptName), []map[string]any{
		{"type": "assistant", "uuid": "legacy-session-memory"},
	})
	unrelatedPath := filepath.Join(projectDir, "unrelated.txt")
	if err = os.WriteFile(unrelatedPath, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewAgentHistoryStore(appfs.UsersRoot()).ForOwner(ownerUserID)
	deleted, err := store.DeleteTranscriptSession(workspacePath, sessionID)
	if err != nil {
		t.Fatalf("删除 transcript session 失败: %v", err)
	}
	if !deleted {
		t.Fatal("存在会话产物时应返回 deleted=true")
	}

	for _, removedPath := range []string{
		filepath.Join(projectDir, sessionID+".jsonl"),
		sessionDir,
		filepath.Join(projectDir, memoryTranscriptName),
		filepath.Join(projectDir, ownedChild+".jsonl"),
		filepath.Join(projectDir, ownedGrandchild+".jsonl"),
		memoryProjectDir,
		legacyProjectDir,
	} {
		if _, statErr := os.Lstat(removedPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("会话产物仍残留 %s: %v", removedPath, statErr)
		}
	}
	for _, retainedPath := range []string{
		filepath.Join(projectDir, otherSessionID+".jsonl"),
		filepath.Join(projectDir, sharedChild+".jsonl"),
		unrelatedPath,
	} {
		if _, statErr := os.Lstat(retainedPath); statErr != nil {
			t.Errorf("共享或无关产物不应被删除 %s: %v", retainedPath, statErr)
		}
	}

	deleted, err = store.DeleteTranscriptSession(workspacePath, sessionID)
	if err != nil || deleted {
		t.Fatalf("重复删除应幂等: deleted=%v err=%v", deleted, err)
	}
}

func structuredSubagentReference(agentID string) map[string]any {
	return map[string]any{
		"type": "attachment",
		"uuid": "reference-" + agentID,
		"attachment": map[string]any{
			"type": "structured_output",
			"data": map[string]any{"agentId": agentID},
		},
	}
}

func writeTranscriptArtifactRows(t *testing.T, path string, rows []map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err = encoder.Encode(row); err != nil {
			t.Fatal(err)
		}
	}
}
