// INPUT: 多成员旧版 JSONL、断点后再次进入的 Room 和同一轮并行工具输出。
// OUTPUT: 冷建/增量/重启分页与 canonical 一致，检查点只保存公开引用，详情可无损展开。
// POS: 大 Room 读取回归与可独立运行的桌面规模基准。
package workspace

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func writeRoomCheckpointMember(t testing.TB, h *RoomHistoryStore, root, conversation string, index, turns, outputBytes int) string {
	t.Helper()
	owner := testRoomOwnerUserID
	agent := fmt.Sprintf("member-%02d", index)
	work := filepath.Join(appfs.UserWorkspaceRootAt(root, owner), agent)
	sessionID := fmt.Sprintf("5d928f9e-31bd-48a0-9cd5-%012d", index)
	key := "agent:" + agent + ":ws:group:" + conversation
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	private := h.agentHistory.ForOwner(owner)
	if _, err := private.files.UpsertSession(work, protocol.Session{SessionKey: key, AgentID: agent, SessionID: &sessionID}); err != nil {
		t.Fatal(err)
	}
	if err := private.AppendRoundMarker(work, key, "private-round", "question", 1000); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(transcriptProjectsDirForWorkspace(work), sanitizeTranscriptPath(canonicalizeTranscriptPath(work)))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, sessionID+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriter(f)
	output := strings.Repeat("x", outputBytes)
	parent := ""
	entry := func(kind, uuid string, msg map[string]any) {
		row := map[string]any{"type": kind, "uuid": uuid, "parentUuid": parent, "sessionId": sessionID, "message": msg}
		b, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(b); err != nil {
			t.Fatal(err)
		}
		if _, err = w.WriteString("\r\n"); err != nil {
			t.Fatal(err)
		}
		parent = uuid
	}
	for n := 0; n < turns; n++ {
		prefix := fmt.Sprintf("private-%d-", n)
		entry("user", prefix+"u", map[string]any{"role": "user", "content": "question"})
		entry("assistant", prefix+"a", map[string]any{"id": prefix + "a", "role": "assistant", "stop_reason": "tool_use", "content": []any{map[string]any{"type": "tool_use", "name": "Read", "id": prefix + "tool", "input": map[string]any{}}}})
		entry("user", prefix+"r", map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": prefix + "tool", "content": output}}})
	}
	id := "visible-" + agent
	entry("assistant", id, map[string]any{"id": id, "role": "assistant", "stop_reason": "end_turn", "content": []any{map[string]any{"type": "text", "text": "reply from " + agent}}})
	if err = w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	round := fmt.Sprintf("room-%02d", index)
	clock := int64(1000 * (index + 1))
	if err = h.AppendInlineMessage(owner, conversation, protocol.Message{"role": "user", "message_id": round + "-question", "round_id": round, "content": "question", "timestamp": clock}); err != nil {
		t.Fatal(err)
	}
	if err = h.AppendTranscriptReference(owner, conversation, work, key, protocol.Message{"role": "assistant", "message_id": id, "round_id": round, "agent_id": agent, "session_id": sessionID, "timestamp": clock + 1}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRoomHistoryReusesPublicMemberCheckpoints(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	h := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "room-ref-checkpoints"
	writeRoomCheckpointMember(t, h, root, conversation, 1, 2, 4096)
	path := writeRoomCheckpointMember(t, h, root, conversation, 2, 2, 4096)
	access := h.historyPageAccess(owner, conversation)
	ledger, err := h.snapshotRoomHistoryLedger(ctx, owner, conversation)
	if err != nil {
		t.Fatal(err)
	}
	build := func(store *RoomHistoryStore) historyPageIndexBuild {
		t.Helper()
		built, err := store.buildRoomHistoryPageIndexWithSourceLimit(ctx, owner, conversation, ledger.Size+1)
		if err != nil || built.Disabled || !built.Cacheable {
			t.Fatalf("build: disabled=%t cacheable=%t err=%v", built.Disabled, built.Cacheable, err)
		}
		return built
	}
	want, err := access.BuildCanonical(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first := build(h)
	assertHistoryPageJSONEqual(t, paginateHistoryPageIndexedGroups(want.Groups, historyPageIndexRequest{Limit: 3}), paginateHistoryPageIndexedGroups(first.Groups, historyPageIndexRequest{Limit: 3}))
	db, err := h.readModel.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM history_room_refs WHERE scope=?", access.Scope).Scan(&count); err != nil || count != 2 {
		t.Fatalf("expected exactly two public refs, got %d: %v", count, err)
	}
	// 模拟进程在第一位成员已提交、第二位尚未提交时退出。
	if _, err = db.ExecContext(ctx, `DELETE FROM history_room_refs WHERE message_id='visible-member-02'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE history_room_refs SET updated_at_ms=1 WHERE message_id='visible-member-01'`); err != nil {
		t.Fatal(err)
	}
	reopened := NewRoomHistoryStore(filepath.Join(root, "users"))
	reopened.readModel = &historyReadModel{path: h.readModel.path}
	t.Cleanup(func() {
		if reopened.readModel.db != nil {
			reopened.readModel.db.Close()
		}
	})
	second := build(reopened)
	assertHistoryPageJSONEqual(t, paginateHistoryPageIndexedGroups(first.Groups, historyPageIndexRequest{Limit: 3}), paginateHistoryPageIndexedGroups(second.Groups, historyPageIndexRequest{Limit: 3}))
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM history_room_refs WHERE scope=? AND (updated_at_ms=1 OR message_id='visible-member-02')`, access.Scope).Scan(&count); err != nil || count != 2 {
		t.Fatalf("checkpoint replayed or member missing: %d %v", count, err)
	}
	// 同 ID 正文被重写，缓存必须随源指纹失效。
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(strings.ReplaceAll(string(raw), "reply from member-02", "edited public reply")), 0600); err != nil {
		t.Fatal(err)
	}
	changed := build(reopened)
	want, err = access.BuildCanonical(ctx)
	if err != nil {
		t.Fatal(err)
	}
	page := paginateHistoryPageIndexedGroups(changed.Groups, historyPageIndexRequest{Limit: 3})
	assertHistoryPageJSONEqual(t, paginateHistoryPageIndexedGroups(want.Groups, historyPageIndexRequest{Limit: 3}), page)
	wire, _ := json.Marshal(page)
	if !strings.Contains(string(wire), "edited public reply") {
		t.Fatal("missing edited reply")
	}
}

func TestRoomHistoryToolResultsSharePreviewBudget(t *testing.T) {
	h := newRoomHistoryTestStore(t, t.TempDir())
	owner, conversation := testRoomOwnerUserID, "medium-results"
	content := strings.Repeat("output", 4*1024)
	var blocks []any
	for i := 0; i < 57; i++ {
		blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": fmt.Sprint(i), "content": content})
	}
	if err := h.AppendInlineMessage(owner, conversation, protocol.Message{"role": "assistant", "message_id": "message", "round_id": "round", "timestamp": int64(100), "content": blocks, "stop_reason": "end_turn"}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, phase := range []string{"cold", "warm"} {
		page, err := h.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 3})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("%s page: rows=%d err=%v", phase, len(page.Items), err)
		}
		var inline, refs int
		got := normalizeMessageContentBlocks(page.Items[0]["content"])
		if len(got) != len(blocks) {
			t.Fatalf("%s lost steps: %d", phase, len(got))
		}
		for _, block := range got {
			text, _ := block["content"].(string)
			inline += len(text)
			ref, _ := block["detail_ref"].(string)
			if ref == "" {
				continue
			}
			refs++
			detail, err := h.ReadMessageDetailContext(ctx, owner, conversation, ref)
			var full string
			if err != nil || json.Unmarshal(detail.Payload, &full) != nil || full != content {
				t.Fatalf("tool %v: %v", block["tool_use_id"], err)
			}
		}
		if inline > historyMessageDetailThresholdBytes || refs == 0 {
			t.Fatalf("%s: inline=%d refs=%d", phase, inline, refs)
		}
	}
}

func BenchmarkRoomHistoryLargeMembers(b *testing.B) {
	for _, fixture := range []struct {
		name        string
		resultBytes int
	}{{"57MB", 100 << 10}, {"588MB", 1 << 20}} {
		b.Run(fixture.name, func(b *testing.B) { benchmarkRoomHistoryMembers(b, fixture.resultBytes) })
	}
}

func benchmarkRoomHistoryMembers(b *testing.B, resultBytes int) {
	const conversation = "large-room-benchmark"
	root := b.TempDir()
	b.Setenv("NEXUS_STATE_ROOT", root)
	b.Setenv("NEXUS_CONFIG_DIR", "")
	owner := testRoomOwnerUserID
	h := NewRoomHistoryStore(filepath.Join(root, "users"))
	for i := 0; i < 16; i++ {
		writeRoomCheckpointMember(b, h, root, conversation, i, 35, resultBytes)
	}
	ctx := context.Background()
	query := HistoryPageQuery{Limit: 3, DeferIndex: true}
	read := func(wantID string) {
		for {
			page, err := h.ReadMessagesPageContext(ctx, owner, conversation, nil, query)
			if err != nil {
				b.Fatal(err)
			}
			if !page.Indexing {
				if !historyPageHasMessage(page, wantID) {
					b.Fatalf("page missing %s", wantID)
				}
				return
			}
			time.Sleep(time.Duration(page.RetryAfterMS) * time.Millisecond)
		}
	}
	started := time.Now()
	read("visible-member-15")
	cold := time.Since(started)
	if err := h.AppendInlineMessage(owner, conversation, protocol.Message{"role": "user", "message_id": "new", "round_id": "next", "content": "next", "timestamp": int64(100000)}); err != nil {
		b.Fatal(err)
	}
	started = time.Now()
	read("new")
	appended := time.Since(started)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		read("new")
	}
	b.StopTimer()
	b.ReportMetric(cold.Seconds(), "s/cold-open")
	b.ReportMetric(float64(appended.Microseconds())/1000, "ms/append-open")
}

func TestForegroundRoomLedgerPublishesExactPage(t *testing.T) {
	ctx := context.Background()
	h := newRoomHistoryTestStore(t, t.TempDir())
	owner, conversation := testRoomOwnerUserID, "foreground-ledger"
	appendRoomHistoryIndexRound(t, h, owner, conversation, "old", "amy", 1000)
	appendRoomHistoryIndexRound(t, h, owner, conversation, "latest", "amy", 2000)
	access := h.historyPageAccess(owner, conversation)
	access.Build = func(ctx context.Context) (historyPageIndexBuild, error) {
		return h.buildRoomHistoryPageIndexWithSourceLimit(ctx, owner, conversation, 1)
	}
	// 目录遇到超预算标记时只读公区摘要；消息首屏承担一次完整读取并发布。
	canonical := access.BuildCanonical
	calls := 0
	access.BuildCanonical = func(ctx context.Context) (historyPageIndexBuild, error) { calls++; return canonical(ctx) }
	index, err := readSessionRoundIndexWithModel(ctx, access, nil, true, false)
	if err != nil || len(index.Items) != 2 || calls != 0 {
		t.Fatalf("navigator replayed full history: %d %v", calls, err)
	}
	query := historyPageIndexRequest{Limit: 1, CollapseRoomRounds: true}
	first, err := readHistoryPageWithIndex(ctx, access, query)
	if err != nil || !first.HasMore || first.NextBeforeRoundID == nil || first.NextBeforeRoundTimestamp == nil || !historyPageHasMessage(first, "user-latest") || calls != 1 {
		t.Fatalf("foreground read: calls=%d err=%v", calls, err)
	}
	query.BeforeRoundID = *first.NextBeforeRoundID
	query.BeforeRoundTimestamp = *first.NextBeforeRoundTimestamp
	old, err := readHistoryPageWithIndex(ctx, access, query)
	if err != nil || !historyPageHasMessage(old, "user-old") || historyPageHasMessage(old, "user-latest") || calls != 1 {
		t.Fatalf("older page replayed: calls=%d err=%v", calls, err)
	}
}

func TestRoomTranscriptSeekMatchesBranchSelection(t *testing.T) {
	ctx := context.Background()
	file, err := os.CreateTemp(t.TempDir(), "fork*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	lines := []string{
		`{"type":"user","uuid":"root","message":{"role":"user","content":"question"}}`,
		`{"type":"assistant","uuid":"dead-end","parentUuid":"root","message":{"role":"assistant","content":[{"type":"text","text":"old branch"}]}}`,
		`{"type":"assistant","uuid":"call","parentUuid":"root","message":{"role":"assistant","content":[{"type":"tool_use","id":"parallel","name":"Read","input":{}}]}}`,
		`{"type":"user","uuid":"parallel-result","parentUuid":"call","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"parallel","content":"result"}]}}`,
		`{"type":"attachment","uuid":"subagent","parentUuid":"call","attachment":{"type":"structured_output","data":{"agent_id":"child","tool_use_id":"parallel"}}}`,
		`bad JSON`, "",
		`{"type":"assistant","uuid":"final","parentUuid":"call","message":{"role":"assistant","content":[{"type":"text","text":"latest branch"}],"stop_reason":"end_turn"}}`,
		`{"type":"assistant","uuid":"final","parentUuid":"final","message":{"role":"assistant","content":[{"type":"text","text":"self copy"}]}}`,
	}
	if _, err = file.WriteString(strings.Join(lines, "\r\n")); err != nil {
		t.Fatal(err)
	}
	if _, err = file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	raw, err := readTranscriptEntriesFileContext(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	chain := buildPrimaryTranscriptChain(raw)
	if _, err = file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	r, err := newRoomTranscriptReader(ctx, file, true, historyReadModelMaxPageBytes)
	if err != nil {
		t.Fatal(err)
	}
	markers := []transcriptRoundMarker{{RoundID: "host-round", Content: "question", UserMessageID: "host-user"}}
	want := projectTranscriptChain("work", "key", "agent", chain, markers)
	var got []protocol.Message
	visitProjectedTranscript("work", "key", "agent", r.entries(ctx), r.alignedMarkers(markers), shouldSkipTranscriptEntry, func(rows ...protocol.Message) { got = append(got, rows...) })
	if r.err != nil || len(want) == 0 {
		t.Fatalf("seek: %v, want %d", r.err, len(want))
	}
	expected, _ := json.Marshal(want)
	actual, _ := json.Marshal(got)
	if string(expected) != string(actual) {
		t.Fatalf("projection changed\nwant %s\ngot %s", expected, actual)
	}
}

func TestRoomRefBatchFailureKeepsCommittedMessages(t *testing.T) {
	h := newRoomHistoryTestStore(t, t.TempDir())
	ctx := context.Background()
	owner, conversation := testRoomOwnerUserID, "interrupted-batch"
	appendRoomHistoryIndexRound(t, h, owner, conversation, "start", "amy", 1000)
	point := roomRefCheckpoint{owner: owner, conversation: conversation, member: "member", stamp: "version"}
	db, err := h.readModel.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `CREATE TRIGGER stop_second_batch BEFORE INSERT ON history_room_refs WHEN new.message_id='id-256' BEGIN SELECT RAISE(ABORT,'interrupt'); END`); err != nil {
		t.Fatal(err)
	}
	items := map[string]protocol.Message{}
	wanted := map[string]bool{}
	for i := 0; i < 300; i++ {
		id := fmt.Sprintf("id-%03d", i)
		wanted[id] = true
		items[id] = protocol.Message{"role": "assistant", "message_id": id}
	}
	write := func(todo map[string]bool) error {
		return h.writeRoomRefCheckpoints(ctx, db, point, todo, items)
	}
	if err = write(wanted); err == nil {
		t.Fatal("expected failed second batch")
	}
	budget := roomRefBudget(historyReadModelMaxPageBytes)
	saved, missing, err := point.read(ctx, db, wanted, &budget)
	if err != nil || len(saved) != 256 || len(missing) != 44 {
		t.Fatalf("unexpected checkpoint: %d saved, %d missing; %v", len(saved), len(missing), err)
	}
	if _, err = db.ExecContext(ctx, "DROP TRIGGER stop_second_batch"); err != nil {
		t.Fatal(err)
	}
	if err = write(missing); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE history_room_refs SET payload='null' WHERE message_id='id-123'`); err != nil {
		t.Fatal(err)
	}
	_, missing, err = point.read(ctx, db, wanted, &budget)
	if err != nil || len(missing) != 1 || !missing["id-123"] {
		t.Fatalf("corrupt row reused: %v %v", missing, err)
	}
	if _, err = h.files.DeleteRoomConversation(owner, conversation); err != nil {
		t.Fatal(err)
	}
	_, missing, err = point.read(ctx, db, wanted, &budget)
	if err != nil || len(missing) != len(wanted) {
		t.Fatal("deleted room retained checkpoints")
	}
}
