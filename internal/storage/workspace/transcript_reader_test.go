// transcript_reader_test.go 覆盖 Transcript 读取、缓存、受限文件边界、并发读取与唤醒测试。
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestTranscriptParseCacheFreshnessAndIsolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	first := []byte("{\"uuid\":\"one\",\"message\":{\"content\":[{\"text\":\"old\"}]}}\n")
	second := []byte("{\"uuid\":\"two\",\"message\":{\"content\":[{\"text\":\"new\"}]}}\n")
	if err := os.WriteFile(path, first, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := confinedfs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	h := NewAgentHistoryStore(dir)
	ctx := context.Background()
	read := func() []transcriptEntry {
		t.Helper()
		rows, err := h.readTranscriptEntriesAtContext(ctx, root, "s.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	rows := read()
	cached := h.cache.entries[path].Entries
	rows[0].Index = 99
	rows[0].Data["message"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"] = "mutated"
	again := read()
	if again[0].Index != 0 || again[0].Data["message"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"] != "old" {
		t.Fatal("cache was mutated by caller")
	}
	if &cached[0] != &h.cache.entries[path].Entries[0] {
		t.Fatal("unchanged file was parsed again")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// 同长度并恢复 mtime 的原地重写，由文件指纹识别。
	if err = os.WriteFile(path, second, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if read()[0].Data["uuid"] != "two" {
		t.Fatal("same-size rewrite reused old data")
	}
	cached = h.cache.entries[path].Entries
	// 同内容、同 mtime 的新文件，由文件身份识别。
	replacement := filepath.Join(dir, "replacement")
	if err = os.WriteFile(replacement, second, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	read()
	if &cached[0] == &h.cache.entries[path].Entries[0] {
		t.Fatal("replacement reused old file identity")
	}
	if err = os.WriteFile(path, append(first, second...), 0o600); err != nil {
		t.Fatal(err)
	}
	if len(read()) != 2 {
		t.Fatal("append not observed")
	}
	if err = os.Truncate(path, 0); err != nil {
		t.Fatal(err)
	}
	if len(read()) != 0 {
		t.Fatal("truncate not observed")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = h.readTranscriptEntriesAtContext(cancelled, root, "s.jsonl"); err != context.Canceled {
		t.Fatalf("cancel ignored: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.readTranscriptEntriesAtContext(ctx, root, "s.jsonl")
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

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

func TestTranscriptCacheKeepsLatestCheckpointedEntryOverBudget(t *testing.T) {
	s := NewAgentHistoryStore(t.TempDir())
	source := historyPageSourceSnapshot{Exists: true, Size: maxTranscriptCacheSourceBytes/2 + 1}
	rows := []transcriptEntry{{Data: map[string]any{"uuid": "test"}}}
	s.writeTranscriptCache("other", source, rows, len(rows))
	lineage := s.writeTranscriptCache("active", source, rows, len(rows))
	// 带检查点后单个条目的权重就超过预算，仍要保留它，只淘汰其他会话。
	s.storeTranscriptProjectionCheckpoint("active", &transcriptProjectionCheckpoint{Lineage: lineage})
	if s.readTranscriptProjectionCheckpoint("active", lineage) == nil {
		t.Fatal("active checkpoint evicted by its own weight")
	}
	if _, ok := s.readTranscriptCache("other", source); ok {
		t.Fatal("older entry retained over budget")
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

func TestTranscriptPathLookupHonorsPreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	history := NewAgentHistoryStore(t.TempDir())
	if _, err := history.resolveTranscriptPathContext(ctx, t.TempDir(), "session"); !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveTranscriptPathContext() error = %v, want context.Canceled", err)
	}

	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "git-dir"))
	t.Setenv("GIT_WORK_TREE", "")
	commandStarted := false
	paths, err := listTranscriptWorktreePathsContextWithCommand(
		ctx,
		t.TempDir(),
		func(commandCtx context.Context, name string, args ...string) *exec.Cmd {
			commandStarted = true
			return exec.CommandContext(commandCtx, name, args...)
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("listTranscriptWorktreePathsContext() error = %v, want context.Canceled", err)
	}
	if len(paths) != 0 {
		t.Fatalf("listTranscriptWorktreePathsContext() = %v, want no paths", paths)
	}
	if commandStarted {
		t.Fatal("预取消 context 不得启动 Git worktree lookup")
	}
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

func TestTranscriptContinuationPromptIsSkippedFromBothChains(t *testing.T) {
	entry := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": "Output token limit hit. Resume directly — no apology, no recap of what you were doing. Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces.",
		},
	}

	if !shouldSkipTranscriptEntry(entry) {
		t.Fatalf("主 transcript chain 应跳过内部续跑提示")
	}
	if !shouldSkipExplicitTranscriptEntry(entry) {
		t.Fatalf("显式 transcript chain 应跳过内部续跑提示")
	}
}

func TestPrimaryTranscriptChainIncludesParallelSubagentStructuredOutput(t *testing.T) {
	entries := []transcriptEntry{
		{
			Index: 0,
			Data: map[string]any{
				"type": "user",
				"uuid": "user-1",
				"message": map[string]any{
					"role":    "user",
					"content": "拉一个子智能体",
				},
			},
		},
		{
			Index: 1,
			Data: map[string]any{
				"type":       "assistant",
				"uuid":       "assistant-agent",
				"parentUuid": "user-1",
				"message": map[string]any{
					"role": "assistant",
					"content": []any{
						map[string]any{
							"type": "tool_use",
							"id":   "call-agent",
							"name": "Agent",
						},
					},
				},
			},
		},
		{
			Index: 2,
			Data: map[string]any{
				"type":       "attachment",
				"uuid":       "agent-output",
				"parentUuid": "assistant-agent",
				"attachment": map[string]any{
					"type": "structured_output",
					"data": map[string]any{
						"agentId":   "agent-child-1",
						"toolUseId": "call-agent",
					},
				},
			},
		},
		{
			Index: 3,
			Data: map[string]any{
				"type":       "user",
				"uuid":       "tool-result",
				"parentUuid": "assistant-agent",
				"message": map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{
							"type":        "tool_result",
							"tool_use_id": "call-agent",
						},
					},
				},
			},
		},
		{
			Index: 4,
			Data: map[string]any{
				"type":       "assistant",
				"uuid":       "assistant-final",
				"parentUuid": "tool-result",
				"message": map[string]any{
					"role":    "assistant",
					"content": []any{map[string]any{"type": "text", "text": "完成"}},
				},
			},
		},
	}

	chain := buildPrimaryTranscriptChain(entries)
	found := false
	for _, entry := range chain {
		if entry.Data["uuid"] == "agent-output" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("主链遗漏与 Agent tool_use 关联的 structured output: %+v", chain)
	}

	rows := projectTranscriptChain("", "agent:host:ws:dm:test", "host", chain, nil)
	var taskNotification map[string]any
	for _, row := range rows {
		metadata, _ := row["metadata"].(map[string]any)
		if metadata["subtype"] == "task_notification" {
			taskNotification = metadata
			break
		}
	}
	if taskNotification == nil ||
		taskNotification["task_id"] != "agent-child-1" ||
		taskNotification["tool_use_id"] != "call-agent" ||
		taskNotification["task_type"] != "local_agent" {
		t.Fatalf("structured output 未投影成 subagent task notification: %+v", rows)
	}
}

const runtimeWakeTestSummary = `Background command "Wait for stage1 confirmation" completed (exit code 0)`

func runtimeWakeTestChain() []transcriptEntry {
	return []transcriptEntry{
		{Index: 0, Data: map[string]any{
			"type": "user", "uuid": "user-1", "timestamp": "2026-11-19T01:00:00Z",
			"message": map[string]any{"role": "user", "content": "生成 PPT"},
		}},
		{Index: 1, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-1", "parentUuid": "user-1", "timestamp": "2026-11-19T01:00:05Z",
			"message": map[string]any{"id": "msg-1", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "请打开 127.0.0.1 确认第一阶段"},
			}},
		}},
		{Index: 2, Data: map[string]any{
			"type": "user", "uuid": "wake-1", "parentUuid": "assistant-1", "timestamp": "2026-11-19T01:01:00Z",
			"origin": map[string]any{"kind": "task-notification"},
			"message": map[string]any{"role": "user", "content": "<task-notification>\n<task-id>b1</task-id>\n<status>completed</status>\n<summary>" +
				runtimeWakeTestSummary + "</summary>\n</task-notification>"},
		}},
		{Index: 3, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-2", "parentUuid": "wake-1", "timestamp": "2026-11-19T01:01:03Z",
			"message": map[string]any{"id": "msg-2", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "第二阶段开始"},
			}},
		}},
	}
}

func findRuntimeWakeRow(t *testing.T, rows []protocol.Message) protocol.Message {
	t.Helper()
	var found protocol.Message
	for _, row := range rows {
		if row["role"] != "user" {
			continue
		}
		if strings.Contains(stringFromAny(row["content"]), "<task-notification>") {
			t.Fatalf("原始 task-notification 标签被投影: %+v", row)
		}
		metadata, _ := row["metadata"].(map[string]any)
		if metadata["input_kind"] == "runtime_wake" {
			found = row
		}
	}
	if found == nil {
		t.Fatalf("没有投影出 runtime 唤醒触发行: %+v", rows)
	}
	if found["content"] != runtimeWakeTestSummary || found["is_synthetic"] != true {
		t.Fatalf("唤醒行 = %+v", found)
	}
	return found
}

// 没有 durable marker 的旧会话：唤醒条目仍成为独立轮次的触发行，而不是带原始标签的用户气泡。
func TestTranscriptRuntimeWakeProjectsTriggerRowWithoutMarker(t *testing.T) {
	rows := projectTranscriptChain("/workspace", "agent:a:ws:dm:c", "a", runtimeWakeTestChain(), nil)
	wake := findRuntimeWakeRow(t, rows)
	for _, row := range rows {
		if row["message_id"] == "msg-2" && row["round_id"] != wake["round_id"] {
			t.Fatalf("唤醒后的回复必须属于唤醒轮次: reply=%v wake=%v", row["round_id"], wake["round_id"])
		}
	}
}

// live 侧写入的唤醒 marker 必须与 transcript 唤醒条目对齐，刷新前后 round 身份一致；
// 普通用户输入不能抢走唤醒 marker。
func TestTranscriptRuntimeWakeAlignsWithLiveMarker(t *testing.T) {
	markers := []transcriptRoundMarker{
		{RoundID: "round-user", UserMessageID: "msg_user_round-user", Content: "生成 PPT", Timestamp: 1795050000000},
		{
			RoundID: "round-wake", UserMessageID: "msg_user_round-wake", Content: runtimeWakeTestSummary,
			Timestamp: 1795050060000, Synthetic: true, Purpose: "runtime_wake",
			Metadata: map[string]string{"input_kind": "runtime_wake", "trigger_kind": "background_task", "task_id": "b1", "task_status": "completed"},
		},
	}
	chain := runtimeWakeTestChain()
	aligned := alignTranscriptRoundMarkers(chain, markers)
	if len(aligned) != 2 || aligned[0].RoundID != "round-user" || aligned[1].RoundID != "round-wake" {
		t.Fatalf("aligned = %+v", aligned)
	}
	rows := projectTranscriptChain("/workspace", "agent:a:ws:dm:c", "a", chain, aligned)
	wake := findRuntimeWakeRow(t, rows)
	if wake["round_id"] != "round-wake" || wake["message_id"] != "msg_user_round-wake" {
		t.Fatalf("唤醒行身份 = %+v", wake)
	}

	materialized := materializeRoundMarkerRows("agent:a:ws:dm:c", "a", markers)
	findRuntimeWakeRow(t, materialized)
}

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
