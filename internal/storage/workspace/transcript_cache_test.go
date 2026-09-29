// INPUT: transcript 重写、替换、取消和分段投影。
// OUTPUT: 缓存命中不重解析，变化与不同投影身份不复用过期数据。
// POS: 共享解析缓存回归。
package workspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

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
	if _, err = h.buildAgentHistoryPageIndex(context.Background(), workspace, session); err != nil {
		t.Fatal(err)
	}
	if &cached[0] != &h.cache.entries[path].Entries[0] {
		t.Fatal("index rebuild cleared unchanged segment")
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
