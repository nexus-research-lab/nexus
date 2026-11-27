// transcript_reader_test.go 覆盖 Transcript 读取、缓存、受限文件边界、并发读取与唤醒测试。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestTranscriptCacheReprojectsMarkersAndSegments(t *testing.T) {
	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace", "agent")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(dir, "home"))
	h := NewAgentHistoryStore(filepath.Dir(workspace))
	ids := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"}
	for _, id := range ids {
		writeAgentTranscriptFixture(t, workspace, id, []map[string]any{{"type": "user", "uuid": id, "message": map[string]any{"role": "user", "content": "hello"}}})
	}
	marker := []transcriptRoundMarker{{RoundID: "before", Content: "hello", Timestamp: 1000}}
	first, err := h.readTranscriptMessages(workspace, "session-a", "agent-a", ids[0], marker, "")
	if err != nil {
		t.Fatal(err)
	}
	path, err := h.resolveTranscriptPath(workspace, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	cached := h.cache.entries[path].Entries
	marker[0].RoundID = "after"
	next, err := h.readTranscriptMessages(workspace, "session-b", "agent-b", ids[0], marker, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(first, next) || next[0]["round_id"] != "after" || next[0]["agent_id"] != "agent-b" {
		t.Fatalf("projection reused old identity/marker: %+v", next)
	}
	if &cached[0] != &h.cache.entries[path].Entries[0] {
		t.Fatal("marker change re-parsed file")
	}
	if _, err = h.readTranscriptMessages(workspace, "s", "a", ids[0], marker, "missing"); err == nil {
		t.Fatal("fork boundary validation skipped")
	}
	rows, err := h.readSegmentedTranscriptMessages(workspace, "session-a", "agent-a", ids, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("segments missing: %+v", rows)
	}
	if &cached[0] != &h.cache.entries[path].Entries[0] {
		t.Fatal("segmented reader did not reuse single-file cache")
	}
	otherPath, err := h.resolveTranscriptPath(workspace, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if h.cache.entries[otherPath].Entries[0].Index != 0 {
		t.Fatal("segment numbering mutated cached entry")
	}
	again, err := h.readSegmentedTranscriptMessages(workspace, "session-a", "agent-a", ids, nil)
	if err != nil || !reflect.DeepEqual(rows, again) {
		t.Fatalf("repeated segment projection changed: %v", err)
	}

	session := protocol.Session{SessionKey: "agent:agent-a:ws:dm:cache", AgentID: "agent-a", SessionID: &ids[1], TranscriptSessionIDs: ids, Options: map[string]any{protocol.OptionRuntimeSegmentedTranscript: true}}
	if _, err = h.readHistoryRowsContext(context.Background(), workspace, session); err != nil {
		t.Fatal(err)
	}
	if &cached[0] != &h.cache.entries[path].Entries[0] {
		t.Fatal("full history read cleared unchanged segment")
	}
	activeCached := h.cache.entries[otherPath].Entries
	writeAgentTranscriptFixture(t, workspace, ids[1], []map[string]any{{"type": "user", "uuid": ids[1], "message": map[string]any{"role": "user", "content": "updated"}}})
	if _, err = h.readSegmentedTranscriptMessages(workspace, "session-a", "agent-a", ids, nil); err != nil {
		t.Fatal(err)
	}
	if &cached[0] != &h.cache.entries[path].Entries[0] || &activeCached[0] == &h.cache.entries[otherPath].Entries[0] {
		t.Fatal("only the changed segment should be parsed again")
	}
	h.invalidateTranscriptCachePrefix(filepath.Dir(path))
	if len(h.cache.entries) != 0 {
		t.Fatal("deleted project retained cache")
	}
}

func TestTranscriptCacheCapsRetainedSourceBytes(t *testing.T) {
	s := NewAgentHistoryStore(t.TempDir())
	source := historyPageSourceSnapshot{Exists: true, Size: maxTranscriptCacheSourceBytes/2 + 1}
	rows := []transcriptEntry{{Data: map[string]any{"uuid": "test"}}}
	s.writeTranscriptCache("first", source, rows, len(rows))
	s.writeTranscriptCache("second", source, rows, len(rows))
	if _, ok := s.readTranscriptCache("first", source); ok {
		t.Fatal("older weighted entry retained")
	}
	if _, ok := s.readTranscriptCache("second", source); !ok {
		t.Fatal("latest entry missing")
	}
	large := source
	large.Size = maxTranscriptCacheSourceBytes + 1
	s.writeTranscriptCache("second", large, rows, len(rows))
	if _, ok := s.readTranscriptCache("second", large); ok {
		t.Fatal("oversized file retained")
	}
}

func TestTranscriptParallelReadMatchesSequential(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	pieces := func() []string {
		lines := []string{}
		for i := 0; i < 200; i++ {
			switch random.Intn(9) {
			case 0:
				lines = append(lines, "")
			case 1:
				lines = append(lines, "   \t")
			case 2:
				lines = append(lines, `{"uuid":`)
			case 3:
				lines = append(lines, fmt.Sprintf(`{"type":"user","message":{"content":"no uuid %d"}}`, i))
			case 4:
				lines = append(lines, fmt.Sprintf(`{"uuid":"crlf-%d","type":"user"}`+"\r", i))
			case 5:
				lines = append(lines, fmt.Sprintf(`{"uuid":"long-%d","type":"assistant","message":{"content":[{"type":"text","text":%q}]}}`, i, strings.Repeat("x", random.Intn(5000))))
			default:
				lines = append(lines, fmt.Sprintf(`  {"uuid":"u-%d","sessionId":"s","type":"user","message":{"content":"q%d"}}  `, i, i))
			}
		}
		return lines
	}
	for _, trailing := range []string{"\n", "", "\n\n"} {
		content := strings.Join(pieces(), "\n") + trailing
		path := filepath.Join(t.TempDir(), "transcript.jsonl")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		want, wantNext, err := readTranscriptEntriesReaderContext(context.Background(), file, 0)
		if err != nil {
			t.Fatal(err)
		}
		size := int64(len(content))
		for _, workers := range []int{2, 3, 7, 16, 64, 1000} {
			got, gotNext, err := readTranscriptEntriesChunksContext(context.Background(), file, size, workers)
			if err != nil {
				t.Fatal(err)
			}
			if gotNext != wantNext || !reflect.DeepEqual(got, want) {
				t.Fatalf("trailing=%q workers=%d: next %d want %d, entries %d want %d", trailing, workers, gotNext, wantNext, len(got), len(want))
			}
		}
		// 快照长度之后追加的字节由最后一块读到，与顺序读取 EOF 的行为一致。
		appended := content + `{"uuid":"late","type":"user"}` + "\n"
		if err = os.WriteFile(path, []byte(appended), 0o600); err != nil {
			t.Fatal(err)
		}
		lateWant, lateNext, _ := readTranscriptEntriesReaderContext(context.Background(), mustSeekStart(t, file), 0)
		lateGot, lateGotNext, err := readTranscriptEntriesChunksContext(context.Background(), file, size, 5)
		if err != nil || lateGotNext != lateNext || !reflect.DeepEqual(lateGot, lateWant) {
			t.Fatalf("trailing=%q: late append diverged: err=%v", trailing, err)
		}
		_ = file.Close()
	}
}

func TestTranscriptParallelReadHonorsContext(t *testing.T) {
	content := strings.Repeat(`{"uuid":"u","type":"user"}`+"\n", 1000)
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = readTranscriptEntriesChunksContext(ctx, file, int64(len(content)), 4); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func mustSeekStart(t *testing.T, file *os.File) *os.File {
	t.Helper()
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestTranscriptWorktreeLookupRequired(t *testing.T) {
	t.Setenv("GIT_DIR", "")
	t.Setenv("GIT_WORK_TREE", "")

	t.Run("non repository", func(t *testing.T) {
		if transcriptWorktreeLookupRequired(t.TempDir()) {
			t.Fatal("非 Git workspace 不应启动 worktree 查询")
		}
	})

	t.Run("single worktree repository", func(t *testing.T) {
		root := t.TempDir()
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		nested := filepath.Join(root, "nested", "workspace")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		if transcriptWorktreeLookupRequired(nested) {
			t.Fatal("单 worktree 仓库不应启动 Git 子进程")
		}
	})

	t.Run("repository with linked worktrees", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(
			filepath.Join(root, ".git", "worktrees", "linked"),
			0o755,
		); err != nil {
			t.Fatal(err)
		}
		if !transcriptWorktreeLookupRequired(root) {
			t.Fatal("存在 linked worktree 时必须回退 Git 查询")
		}
	})

	t.Run("linked worktree metadata file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(root, ".git"),
			[]byte("gitdir: /tmp/example/.git/worktrees/linked\n"),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
		if !transcriptWorktreeLookupRequired(root) {
			t.Fatal("linked worktree 的 .git 文件必须触发 Git 查询")
		}
	})
}

func TestTranscriptWorktreeLookupRequiredHonorsGitEnvironment(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "git-dir"))
	t.Setenv("GIT_WORK_TREE", "")
	if !transcriptWorktreeLookupRequired(t.TempDir()) {
		t.Fatal("显式 GIT_DIR 必须保留 Git 查询")
	}
}

const runtimeWakeTestSummary = `Background command "Wait for stage1 confirmation" completed (exit code 0)`

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
	internalMemoryTranscriptName := "session-memory-" + memoryTranscriptName
	writeTranscriptArtifactRows(t, filepath.Join(projectDir, internalMemoryTranscriptName), []map[string]any{
		{"type": "assistant", "uuid": "internal-session-memory"},
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
		filepath.Join(projectDir, internalMemoryTranscriptName),
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
