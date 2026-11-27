// conversation_history_test.go 覆盖会话历史夹具、读取、归一化、游标、Overlay 与 Live 对齐测试。
package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type longSessionFixture struct {
	history       *AgentHistoryStore
	workspacePath string
	session       protocol.Session
	transcript    string
	lastUUID      string
	rounds        int
	toolBytes     int
}

func newLongSessionFixture(tb testing.TB, rounds int, toolBytes int) *longSessionFixture {
	tb.Helper()
	configRoot := tb.TempDir()
	tb.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		tb.Fatal(err)
	}
	sessionID := "7210e741-b09d-4714-a9c8-adc235129002"
	session := protocol.Session{
		SessionKey: "agent:amy:ws:dm:long-session",
		AgentID:    "amy",
		SessionID:  &sessionID,
		Options:    map[string]any{},
	}
	history := NewAgentHistoryStore(workspaceRoot)
	if _, err := history.files.UpsertSession(workspacePath, session); err != nil {
		tb.Fatal(err)
	}
	projectDir := filepath.Join(transcriptProjectsDirForWorkspace(workspacePath), TranscriptProjectDirectoryName(workspacePath))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	fixture := &longSessionFixture{
		history:       history,
		workspacePath: workspacePath,
		session:       session,
		transcript:    filepath.Join(projectDir, sessionID+".jsonl"),
		toolBytes:     toolBytes,
	}
	for range rounds {
		fixture.appendRound(tb)
	}
	return fixture
}

func (f *longSessionFixture) appendRound(tb testing.TB) {
	tb.Helper()
	f.appendTranscript(tb, f.startRound(tb)...)
}

// startRound 写入下一轮的 marker，并返回该轮尚未写入的 transcript 行，供测试分段追加。
func (f *longSessionFixture) startRound(tb testing.TB) []map[string]any {
	tb.Helper()
	round := f.rounds
	f.rounds++
	roundID := fmt.Sprintf("round-%05d", round)
	prompt := fmt.Sprintf("question %d", round)
	timestamp := int64(1_700_000_000_000 + round*10_000)
	if err := f.history.AppendRoundMarker(f.workspacePath, f.session.SessionKey, roundID, prompt, timestamp); err != nil {
		tb.Fatal(err)
	}
	sessionID := *f.session.SessionID
	ts := func(offset int) string {
		return time.UnixMilli(timestamp + int64(offset)*1000).UTC().Format(time.RFC3339Nano)
	}
	uuid := func(kind string) string { return fmt.Sprintf("%s-%05d", kind, round) }
	rows := []map[string]any{
		{"type": "user", "uuid": uuid("u"), "parentUuid": nilIfEmpty(f.lastUUID), "sessionId": sessionID, "timestamp": ts(0),
			"message": map[string]any{"role": "user", "content": prompt}},
		{"type": "assistant", "uuid": uuid("a1"), "parentUuid": uuid("u"), "sessionId": sessionID, "timestamp": ts(1),
			"message": map[string]any{"id": uuid("m1"), "role": "assistant", "stop_reason": "tool_use",
				"content": []map[string]any{{"type": "tool_use", "id": uuid("t"), "name": "Bash", "input": map[string]any{"command": "ls -la"}}}}},
		{"type": "user", "uuid": uuid("r"), "parentUuid": uuid("a1"), "sessionId": sessionID, "timestamp": ts(2),
			"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": uuid("t"),
				"content": strings.Repeat("x", f.toolBytes)}}}},
		{"type": "assistant", "uuid": uuid("a2"), "parentUuid": uuid("r"), "sessionId": sessionID, "timestamp": ts(3),
			"message": map[string]any{"id": uuid("m2"), "role": "assistant", "stop_reason": "end_turn",
				"content": []map[string]any{{"type": "text", "text": strings.Repeat("answer ", 128)}}}},
	}
	f.lastUUID = uuid("a2")
	return rows
}

func (f *longSessionFixture) appendTranscript(tb testing.TB, rows ...map[string]any) {
	tb.Helper()
	file, err := os.OpenFile(f.transcript, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		tb.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			tb.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		tb.Fatal(err)
	}
	// 夹具绕过宿主直接写 runtime transcript，没有经过实时落表：让下一次读取整会话回填，效果同进程重启后读取。
	f.history.invalidateDMConversationHistory(f.workspacePath, f.session.SessionKey)
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (f *longSessionFixture) readFirstPage(tb testing.TB) protocol.MessagePage {
	tb.Helper()
	page, err := f.history.ReadMessagesPageContext(context.Background(), f.workspacePath, f.session, nil, HistoryPageQuery{Limit: 20})
	if err != nil {
		tb.Fatal(err)
	}
	return page
}

// simulateHistoryProcessRestart 丢弃所有进程内状态：解析缓存、投影检查点、读模型连接与「已检查」记录，只剩落盘文件与表。
func simulateHistoryProcessRestart(tb testing.TB, fixture *longSessionFixture) {
	tb.Helper()
	previous := fixture.history.history
	previous.closeForTest()
	fixture.history = NewAgentHistoryStore(filepath.Dir(fixture.workspacePath))
	fixture.history.history = newConversationHistoryStoreForTest(previous.path)
	tb.Cleanup(fixture.history.history.closeForTest)
}

func (s *AgentHistoryStore) readSegmentedTranscriptMessages(
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionIDs []string,
	roundMarkers []transcriptRoundMarker,
) ([]protocol.Message, error) {
	return s.readSegmentedTranscriptMessagesContext(
		context.Background(),
		workspacePath,
		sessionKey,
		agentID,
		sessionIDs,
		roundMarkers,
	)
}

func (s *AgentHistoryStore) readTranscriptMessages(
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
	roundMarkers []transcriptRoundMarker,
	throughMessageID string,
) ([]protocol.Message, error) {
	return s.readTranscriptProjectionContext(
		context.Background(), workspacePath, sessionKey, agentID, sessionID, roundMarkers, throughMessageID,
	)
}

func (s *AgentHistoryStore) readTranscriptCache(path string, source historyPageSourceSnapshot) ([]transcriptEntry, bool) {
	entries, _, ok := s.readTranscriptCacheLineage(path, source)
	return cloneTranscriptEntries(entries), ok
}

func readTranscriptEntriesFileContext(ctx context.Context, file *os.File) ([]transcriptEntry, error) {
	entries, _, err := readTranscriptEntriesReaderContext(ctx, file, 0)
	return entries, err
}

func (s *SessionFileStore) readJSONL(path string) ([]map[string]any, error) {
	root, relative, err := s.openStorePath(path, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return readJSONLAtRoot(root, relative)
}

// newConversationHistoryStoreForTest 返回指向同一库文件的全新存储实例，模拟进程重启：进程内的单飞、
// 退避与「已检查」记录全部丢弃，只剩落盘的表。
func newConversationHistoryStoreForTest(path string) *conversationHistoryStore {
	return &conversationHistoryStore{
		path:     path,
		inflight: make(map[string]*conversationHistoryFlight),
		attempts: make(map[string]conversationHistoryAttempt),
	}
}

func (h *conversationHistoryStore) closeForTest() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db != nil {
		_ = h.db.Close()
		h.db = nil
	}
}

func conversationHistoryRowExists(t *testing.T, h *conversationHistoryStore, scope, messageID string) bool {
	t.Helper()
	db, err := h.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM conversation_history_rows WHERE scope = ? AND message_id = ?`, scope, messageID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count > 0
}

// assertRoomPagesMatchCanonical 从最新一页往前翻到底，每页都与 canonical oracle 逐字节一致。
func assertRoomPagesMatchCanonical(t *testing.T, h *RoomHistoryStore, owner, conversation string) {
	t.Helper()
	before, beforeTimestamp := "", int64(0)
	for page := 0; page < 30; page++ {
		got := assertRoomHistoryPageParity(t, h, owner, conversation, nil, 2, before, beforeTimestamp, "", 0)
		if !got.HasMore || got.NextBeforeRoundID == nil || got.NextBeforeRoundTimestamp == nil {
			return
		}
		before, beforeTimestamp = *got.NextBeforeRoundID, *got.NextBeforeRoundTimestamp
	}
	t.Fatal("分页没有结束")
}

func TestConversationHistoryRoomBackfillAndLiveWritesMatchCanonical(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	h := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "projection-room"
	scope := roomConversationHistoryScope(owner, conversation).key

	// 第一次读取之前写入的行只在 ledger 里；第一次读取同步回填出全部轮次。
	for index := 1; index <= 5; index++ {
		writeRoomCheckpointMember(t, h, root, conversation, index, 2, 1024)
	}
	assertRoomPagesMatchCanonical(t, h, owner, conversation)
	assertRoomHistoryPageParity(t, h, owner, conversation, nil, 0, "", 0, "room-03", 1)

	// 就绪后的实时写入在追加 ledger 时直接落表。
	if err := h.AppendInlineMessage(owner, conversation, protocol.Message{
		"role": "user", "message_id": "live-question", "round_id": "room-live",
		"content": "next", "timestamp": int64(100000),
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.AppendInlineMessage(owner, conversation, protocol.Message{
		"role": "assistant", "message_id": "live-answer", "round_id": "room-live:member-01", "agent_id": "member-01",
		"content":     []any{map[string]any{"type": "text", "text": "live answer"}},
		"stop_reason": "end_turn", "is_complete": true, "timestamp": int64(100001),
	}); err != nil {
		t.Fatal(err)
	}
	if !conversationHistoryRowExists(t, h.history, scope, "live-answer") {
		t.Fatal("实时写入没有直接落表")
	}
	assertRoomPagesMatchCanonical(t, h, owner, conversation)
	index, err := h.ReadRoundIndexPageContext(ctx, owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalRoomRoundIndexForTest(h, owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSessionRoundIndexJSONEqual(t, want, index)

	// 删除会话时一并删掉表里的数据。
	if _, err = h.files.DeleteRoomConversation(owner, conversation); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := h.history.state(ctx, scope); err != nil || exists {
		t.Fatalf("删除会话后状态仍在: exists=%t err=%v", exists, err)
	}
	if conversationHistoryRowExists(t, h.history, scope, "live-answer") {
		t.Fatal("删除会话后仍有残行")
	}
}

// 公区 ledger 被替换（例如从备份恢复）后，下一次读取发现文件身份变化，整会话重新回填。
func TestConversationHistoryRoomRebuildsAfterLedgerReplacement(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	h := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "projection-replaced"
	appendRoomHistoryIndexRound(t, h, owner, conversation, "kept", "amy", 1000)
	appendRoomHistoryIndexRound(t, h, owner, conversation, "dropped", "amy", 2000)
	assertRoomPagesMatchCanonical(t, h, owner, conversation)

	path := h.paths.RoomConversationOverlayPath(owner, conversation)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if !strings.Contains(scanner.Text(), "dropped") {
			lines = append(lines, scanner.Text())
		}
	}
	_ = file.Close()
	replacement := path + ".replacement"
	if err = os.WriteFile(replacement, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	page := assertRoomHistoryPageParity(t, h, owner, conversation, nil, 10, "", 0, "", 0)
	if historyPageHasMessage(page, "user-dropped") || !historyPageHasMessage(page, "user-kept") {
		t.Fatalf("替换后的 ledger 没有重新回填: %s", parityDump(page.Items))
	}
	if _, ok, err := h.history.state(ctx, roomConversationHistoryScope(owner, conversation).key); err != nil || !ok {
		t.Fatalf("重新回填后状态缺失: %v", err)
	}
}

func TestConversationHistorySplitsLargeToolResults(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	h := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "projection-details"
	// 空会话的第一次读取完成回填，之后的写入都直接落表。
	if _, err := h.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 10}); err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", historyMessageDetailThresholdBytes+1024)
	appendRow := func(row protocol.Message) {
		t.Helper()
		if err := h.AppendInlineMessage(owner, conversation, row); err != nil {
			t.Fatal(err)
		}
	}
	appendRow(protocol.Message{"role": "user", "message_id": "user-d", "round_id": "round-d", "content": "run it", "timestamp": int64(1000)})
	toolUse := map[string]any{"type": "tool_use", "id": "tool-1", "name": "Bash", "input": map[string]any{}}
	toolResult := map[string]any{"type": "tool_result", "tool_use_id": "tool-1", "content": large}
	snapshot := func(blocks ...any) protocol.Message {
		return protocol.Message{
			"role": "assistant", "message_id": "assistant-d", "round_id": "round-d:amy", "agent_id": "amy",
			"content": blocks, "timestamp": int64(1001),
		}
	}
	appendRow(snapshot(toolUse, toolResult))
	final := snapshot(toolUse, toolResult, map[string]any{"type": "text", "text": "done"})
	final["stop_reason"], final["is_complete"] = "end_turn", true
	appendRow(final)

	page, err := h.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var ref string
	for _, item := range page.Items {
		if item["message_id"] != "assistant-d" {
			continue
		}
		for _, block := range normalizeMessageContentBlocks(item["content"]) {
			if block["type"] == "tool_result" {
				ref = stringFromAny(block["detail_ref"])
				if preview := stringFromAny(block["content"]); len(preview) > historyMessageDetailPreviewBytes {
					t.Fatalf("页面里的预览过长: %d", len(preview))
				}
			}
		}
	}
	if ref == "" {
		t.Fatalf("大块工具输出没有拆成 detail: %s", parityDump(page.Items))
	}
	detail, err := h.ReadMessageDetailContext(ctx, owner, conversation, ref)
	if err != nil {
		t.Fatal(err)
	}
	var content string
	if err = json.Unmarshal(detail.Payload, &content); err != nil || content != large {
		t.Fatalf("detail 内容不一致: err=%v size=%d", err, len(content))
	}
	if _, err = h.ReadMessageDetailContext(ctx, owner, "other-conversation", ref); err != ErrHistoryMessageDetailUnavailable {
		t.Fatalf("跨会话的 detail 引用必须拒绝: %v", err)
	}

	// 成员上下文要原文，不能是预览。
	rows, err := h.ReadRuntimeHistoryContext(ctx, owner, conversation, "user-d", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	restored := false
	for _, row := range rows {
		for _, block := range normalizeMessageContentBlocks(row["content"]) {
			if block["type"] == "tool_result" && block["content"] == large {
				restored = true
			}
		}
	}
	if !restored {
		t.Fatalf("成员上下文没有还原工具输出: %s", parityDump(rows))
	}

	// 两份快照引用同一份内容，只存一份。
	db, err := h.history.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM conversation_history_details WHERE scope = ?`,
		roomConversationHistoryScope(owner, conversation).key).Scan(&count); err != nil || count != 1 {
		t.Fatalf("detail 行数 = %d, want 1 (err=%v)", count, err)
	}
}

func TestConversationHistoryDefersLargeImages(t *testing.T) {
	ctx := context.Background()
	h := newRoomHistoryTestStore(t, t.TempDir())
	owner, conversation := testRoomOwnerUserID, "projection-image"
	if _, err := h.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	imageBytes := bytes.Repeat([]byte{0x42}, historyMessageDetailThresholdBytes)
	if err := h.AppendInlineMessage(owner, conversation, protocol.Message{
		"role": "assistant", "message_id": "assistant-image", "round_id": "round-image", "timestamp": int64(1000),
		"content": []any{map[string]any{"type": "image", "mime_type": "image/png", "data": base64.StdEncoding.EncodeToString(imageBytes)}},
	}); err != nil {
		t.Fatal(err)
	}
	page, err := h.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 1})
	if err != nil || len(page.Items) == 0 {
		t.Fatalf("page: %+v %v", page, err)
	}
	blocks := normalizeMessageContentBlocks(page.Items[0]["content"])
	if len(blocks) != 1 || blocks[0]["detail_ref"] == nil || blocks[0]["data"] != nil {
		t.Fatalf("大图没有拆成 detail: %#v", blocks)
	}
	detail, err := h.ReadMessageDetailContext(ctx, owner, conversation, stringFromAny(blocks[0]["detail_ref"]))
	if err != nil || detail.Kind != historyMessageDetailKindImage || !bytes.Equal(detail.Payload, imageBytes) {
		t.Fatalf("图片 detail: kind=%s size=%d err=%v", detail.Kind, len(detail.Payload), err)
	}
	// 图片无法还原原始 data 形状，成员上下文按 canonical 读取原文。
	rows, err := h.ReadRuntimeHistoryContext(ctx, owner, conversation, "assistant-image", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := h.ReadMessages(owner, conversation, nil)
	if err != nil || parityDump(rows) != parityDump(canonical) {
		t.Fatalf("图片会话的成员上下文没有回退 canonical: %v", err)
	}
}

// 同一条 assistant 先写带大块输出 A 的快照，再写 A+B。表里的旧快照是骨架（A 已换成预览加 detail_ref），
// 与原文快照合并后两块都必须能完整展开；回填方向（表里已有的行比回填快照新）同样成立。
func TestConversationHistoryMergesSnapshotsOverDetailSkeletons(t *testing.T) {
	ctx := context.Background()
	h := newRoomHistoryTestStore(t, t.TempDir())
	store := h.history
	db, err := store.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{
		"tool-a": strings.Repeat("a", historyMessageDetailThresholdBytes+512),
		"tool-b": strings.Repeat("b", historyMessageDetailThresholdBytes+1024),
	}
	use := func(id string) map[string]any {
		return map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{}}
	}
	result := func(id string) map[string]any {
		return map[string]any{"type": "tool_result", "tool_use_id": id, "content": contents[id]}
	}
	snapshot := func(blocks ...any) protocol.Message {
		return protocol.Message{
			"role": "assistant", "message_id": "assistant-m", "round_id": "round-m:amy", "agent_id": "amy",
			"content": blocks, "timestamp": int64(2000),
		}
	}
	first := snapshot(use("tool-a"), result("tool-a"))
	second := snapshot(use("tool-a"), result("tool-a"), use("tool-b"), result("tool-b"))

	for _, tc := range []struct {
		conversation string
		apply        func(scope conversationHistoryScope) error
	}{
		{"merge-live", func(scope conversationHistoryScope) error {
			if err := store.applyRows(ctx, scope, []protocol.Message{first}, conversationHistoryLiveSeq, nil); err != nil {
				return err
			}
			return store.applyRows(ctx, scope, []protocol.Message{second}, conversationHistoryLiveSeq, nil)
		}},
		{"merge-backfill", func(scope conversationHistoryScope) error {
			if err := store.applyRows(ctx, scope, []protocol.Message{second}, conversationHistoryLiveSeq, nil); err != nil {
				return err
			}
			return store.applyRows(ctx, scope, []protocol.Message{first}, -1, nil)
		}},
	} {
		scope := roomConversationHistoryScope(testRoomOwnerUserID, tc.conversation)
		if err = writeConversationHistoryState(ctx, db, scope.key, conversationHistoryState{
			Status: conversationHistoryStatusReady, ProjectionVersion: message.ProjectionVersion,
		}); err != nil {
			t.Fatal(err)
		}
		if err = tc.apply(scope); err != nil {
			t.Fatalf("%s: %v", tc.conversation, err)
		}
		page, ok, err := store.readPage(ctx, scope, "", historyPageIndexRequest{Limit: 10, CollapseRoomRounds: true})
		if err != nil || !ok {
			t.Fatalf("%s: 读表失败 ok=%t err=%v", tc.conversation, ok, err)
		}
		expanded := map[string]bool{}
		for _, item := range page.Items {
			if item["message_id"] != "assistant-m" {
				continue
			}
			for _, block := range normalizeMessageContentBlocks(item["content"]) {
				if block["type"] != "tool_result" {
					continue
				}
				id := stringFromAny(block["tool_use_id"])
				detail, found, err := store.readDetail(ctx, scope, "", stringFromAny(block["detail_ref"]))
				if err != nil || !found {
					t.Fatalf("%s: %s 展开失败 found=%t err=%v", tc.conversation, id, found, err)
				}
				var content string
				if err = json.Unmarshal(detail.Payload, &content); err != nil || content != contents[id] {
					t.Fatalf("%s: %s 内容不一致 err=%v size=%d", tc.conversation, id, err, len(content))
				}
				expanded[id] = true
			}
		}
		if !expanded["tool-a"] || !expanded["tool-b"] {
			t.Fatalf("%s: 应展开 A 与 B，实际 %v\n%s", tc.conversation, expanded, parityDump(page.Items))
		}
	}
}

func newDMConversationHistoryFixture(t *testing.T) (*AgentHistoryStore, string) {
	t.Helper()
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))
	return NewAgentHistoryStore(workspaceRoot), workspacePath
}

func TestConversationHistoryDMLiveRowsMatchCanonical(t *testing.T) {
	ctx := context.Background()
	history, workspacePath := newDMConversationHistoryFixture(t)
	sessionKey := "agent:Amy:ws:dm:projection"
	sessionID := "sdk-session-parity"
	session := protocol.Session{SessionKey: sessionKey, AgentID: "Amy", SessionID: &sessionID}
	scope, _ := dmConversationHistoryScope(workspacePath, sessionKey)
	all := HistoryPageQuery{Limit: 100}

	// 空会话的第一次读取完成回填：之后的写入全部走实时落表。
	if _, err := history.ReadMessagesPageContext(ctx, workspacePath, session, nil, all); err != nil {
		t.Fatal(err)
	}
	var transcript []map[string]any
	var arrival int64
	for _, round := range liveParityRounds(sessionID) {
		round.options.InputSeq = protocol.NewInputSeq()
		if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, round.roundID, round.content, round.at, round.options); err != nil {
			t.Fatal(err)
		}
		mapper := message.NewEventMapper(message.EventMapperOptions{Context: message.MessageContext{
			SessionKey: sessionKey, AgentID: "Amy", WorkspacePath: workspacePath,
			RoundID: round.roundID, AgentRoundID: round.options.AgentRoundID, ParentID: round.options.UserMessageID,
		}, IncludeStreamLifecycle: true})
		for _, step := range round.steps {
			if step.transcript {
				transcript = append(transcript, step.raw)
			}
			if !step.live {
				continue
			}
			decoded, err := sdkprotocol.DecodeMessage(jsonRoundTrip(t, step.raw))
			if err != nil {
				t.Fatalf("解码实时消息失败: %v", err)
			}
			result, err := mapper.Map(decoded)
			if err != nil {
				t.Fatalf("实时映射失败: %v", err)
			}
			if at, parseErr := time.Parse(time.RFC3339, stringFromAny(step.raw["timestamp"])); parseErr == nil {
				arrival = at.UnixMilli() + parityHostLatencyMillis
			} else {
				arrival++
			}
			for _, durable := range result.DurableMessages {
				durable["timestamp"] = arrival
				// 与 DM 实时链路一致：transcript 原生回复只记录，其余写 overlay。
				if protocol.IsTranscriptNativeMessage(durable) {
					history.RecordTranscriptNativeMessage(workspacePath, sessionKey, durable)
					continue
				}
				if err := history.AppendOverlayMessage(workspacePath, sessionKey, durable); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, transcript)

	refreshed, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err := history.ReadMessagesPageContext(ctx, workspacePath, session, nil, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != len(refreshed) {
		t.Fatalf("行数不一致: table=%d refreshed=%d\ntable=%s\nrefreshed=%s", len(page.Items), len(refreshed), parityDump(page.Items), parityDump(refreshed))
	}
	for index := range refreshed {
		if want, got := parityComparable(t, refreshed[index]), parityComparable(t, page.Items[index]); want != got {
			t.Errorf("第 %d 行不一致:\n refreshed %s\n table     %s", index, want, got)
		}
	}

	// 按 canonical 重新回填后与刷新后的历史完全一致（含时间与 transcript 行号）。
	if err = history.backfillDMConversationHistory(ctx, workspacePath, session); err != nil {
		t.Fatal(err)
	}
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 100, "", 0, "", 0)
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 1, "", 0, "", 0)
	assertAgentHistoryPageParity(t, history, workspacePath, session, nil, 0, "", 0, "round-wake", 1)

	// 改写裁掉的轮次立即从表里消失。
	if _, err = history.RemoveOverlayRounds(workspacePath, sessionKey, []string{"round-3"}); err != nil {
		t.Fatal(err)
	}
	page, err = history.ReadMessagesPageContext(ctx, workspacePath, session, nil, all)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if item["round_id"] == "round-3" {
			t.Fatalf("改写裁掉的轮次仍在表里: %s", parityDump(page.Items))
		}
	}

	// 换了 transcript 会话：来源身份变化，下一次读取整会话回填并换上新身份。
	otherID := "sdk-session-other"
	switched := session
	switched.SessionID = &otherID
	if _, err = history.ReadMessagesPageContext(ctx, workspacePath, switched, nil, all); err != nil {
		t.Fatal(err)
	}
	state, ok, err := history.history.state(ctx, scope.key)
	wantIdentity, _ := dmConversationHistoryIdentity(switched)
	if err != nil || !ok || state.Identity != wantIdentity {
		t.Fatalf("换 transcript 后没有按新身份回填: state=%+v err=%v", state, err)
	}
}

// 进程在 runtime 写完 transcript、表还没提交时退出：表里缺这条回复。新进程第一次读取发现最近轮次未收口，
// 整会话回填一次补上；同一进程里不重复检查。
func TestConversationHistoryDMReverifiesUnfinishedRoundsOncePerProcess(t *testing.T) {
	ctx := context.Background()
	history, workspacePath := newDMConversationHistoryFixture(t)
	sessionKey := "agent:Amy:ws:dm:verify"
	sessionID := "sdk-session-verify"
	session := protocol.Session{SessionKey: sessionKey, AgentID: "Amy", SessionID: &sessionID}
	all := HistoryPageQuery{Limit: 100}
	if _, err := history.ReadMessagesPageContext(ctx, workspacePath, session, nil, all); err != nil {
		t.Fatal(err)
	}
	turn := func(index int) []map[string]any {
		second := fmt.Sprintf("2026-11-19T01:0%d:00Z", index)
		reply := fmt.Sprintf("2026-11-19T01:0%d:01Z", index)
		return []map[string]any{
			{"type": "user", "uuid": fmt.Sprint("u", index), "session_id": sessionID, "timestamp": second,
				"message": map[string]any{"role": "user", "content": fmt.Sprint("question ", index)}},
			{"type": "assistant", "uuid": fmt.Sprint("a", index), "parentUuid": fmt.Sprint("u", index), "session_id": sessionID, "timestamp": reply,
				"message": map[string]any{"id": fmt.Sprint("msg-", index), "role": "assistant", "model": "m", "stop_reason": "end_turn",
					"content": []any{map[string]any{"type": "text", "text": fmt.Sprint("answer ", index)}}}},
		}
	}
	var transcript []map[string]any
	crash := func(index int) {
		t.Helper()
		at := time.Date(2026, 11, 19, 1, index, 0, 0, time.UTC).UnixMilli() - 500
		if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, fmt.Sprint("round-", index), fmt.Sprint("question ", index), at,
			RoundMarkerOptions{UserMessageID: fmt.Sprint("msg_user_round-", index)}); err != nil {
			t.Fatal(err)
		}
		// runtime 已写完 transcript，宿主还没把回复落表就退出了。
		transcript = append(transcript, turn(index)...)
		writeAgentTranscriptFixture(t, workspacePath, sessionID, transcript)
	}
	hasReply := func(index int) bool {
		t.Helper()
		page, err := history.ReadMessagesPageContext(ctx, workspacePath, session, nil, all)
		if err != nil {
			t.Fatal(err)
		}
		return historyPageHasMessage(page, fmt.Sprint("msg-", index))
	}

	restart := func() {
		history.history.closeForTest()
		history.history = newConversationHistoryStoreForTest(history.history.path)
		t.Cleanup(history.history.closeForTest)
	}
	crash(1)
	restart()
	if !hasReply(1) {
		t.Fatal("新进程第一次读取没有补回未落表的回复")
	}
	crash(2)
	if hasReply(2) {
		t.Fatal("同一进程里不应重复整会话回填")
	}
	restart()
	if !hasReply(2) {
		t.Fatal("再次重启后没有补回未落表的回复")
	}

	// 未收口轮次与上次回填时的指纹相同（没有新的落表写入）：重启后不再整会话回填。
	scope, _ := dmConversationHistoryScope(workspacePath, sessionKey)
	stamp := func() int64 {
		t.Helper()
		db, err := history.history.database(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var updated int64
		if err = db.QueryRow(`SELECT updated_at_ms FROM conversation_history_state WHERE scope = ?`, scope.key).Scan(&updated); err != nil {
			t.Fatal(err)
		}
		return updated
	}
	before := stamp()
	time.Sleep(5 * time.Millisecond)
	restart()
	hasReply(2)
	if after := stamp(); after != before {
		t.Fatalf("指纹未变时重启后不应整会话回填: before=%d after=%d", before, after)
	}
}

// TestConversationHistoryFallsBackToCanonicalWhenBackfillFails 回填失败（这里用退避窗口内的上次错误模拟）时，
// 分页与导航退回 canonical，而不是让会话打不开。
func TestConversationHistoryFallsBackToCanonicalWhenBackfillFails(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	h := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "fallback-room"
	appendRoomHistoryIndexRound(t, h, owner, conversation, "root-1", "amy", 1000)
	appendRoomHistoryIndexRound(t, h, owner, conversation, "root-2", "amy", 3000)
	scope := roomConversationHistoryScope(owner, conversation).key
	h.history.jobsMu.Lock()
	h.history.attempts[scope] = conversationHistoryAttempt{at: time.Now(), failures: 1, err: errors.New("disk I/O error")}
	h.history.jobsMu.Unlock()

	assertRoomHistoryPageParity(t, h, owner, conversation, nil, 1, "", 0, "", 0)
	index, err := h.ReadRoundIndexPageContext(ctx, owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalRoomRoundIndexForTest(h, owner, conversation, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSessionRoundIndexJSONEqual(t, want, index)

	// 请求自身取消时照常返回错误，不退回 canonical。
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = h.ReadMessagesPageContext(canceled, owner, conversation, nil, HistoryPageQuery{Limit: 1}); err == nil {
		t.Fatal("请求已取消时不应退回 canonical")
	}
}

func TestConversationHistoryRemovesLegacyReadModel(t *testing.T) {
	root := t.TempDir()
	h := newRoomHistoryTestStore(t, root)
	legacy := filepath.Join(filepath.Dir(h.history.path), conversationHistoryLegacyFileName)
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.WriteFile(legacy+suffix, []byte("legacy"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fresh := newConversationHistoryStoreForTest(h.history.path)
	t.Cleanup(fresh.closeForTest)
	if _, err := fresh.database(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(legacy + suffix); !os.IsNotExist(err) {
			t.Fatalf("旧读模型文件 %s 没有删除: %v", legacy+suffix, err)
		}
	}
}

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

func TestAgentHistoryStoreHidesGoalContextTranscriptTurnWithoutMarker(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "goal_context",
			content: "<goal_context>\nContinue working toward the active thread goal.\n</goal_context>",
		},
		{
			name:    "codex_internal_context",
			content: "<codex_internal_context source=\"goal\">\nContinue working toward the active thread goal.\n</codex_internal_context>",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configRoot := t.TempDir()
			workspaceRoot := filepath.Join(configRoot, "workspace")
			workspacePath := filepath.Join(workspaceRoot, "Amy")
			if err := os.MkdirAll(workspacePath, 0o755); err != nil {
				t.Fatalf("创建 workspace 失败: %v", err)
			}
			t.Setenv("NEXUS_STATE_ROOT", "")
			t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))

			history := NewAgentHistoryStore(workspaceRoot)
			sessionKey := "agent:c5740009ac97:ws:dm:a731e54f7af5"
			sessionID := "hidden-" + test.name + "-no-marker-session"
			writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
				{
					"type":      "user",
					"uuid":      "transcript-user-hidden-context",
					"sessionId": sessionID,
					"timestamp": "2026-05-22T10:00:00.000Z",
					"message": map[string]any{
						"role":    "user",
						"content": test.content,
					},
				},
				{
					"type":       "assistant",
					"uuid":       "transcript-assistant-hidden-context",
					"sessionId":  sessionID,
					"parentUuid": "transcript-user-hidden-context",
					"message": map[string]any{
						"role":        "assistant",
						"stop_reason": "end_turn",
						"content": []map[string]any{
							{"type": "text", "text": "继续推进 Goal。"},
						},
					},
				},
			})

			rows, err := history.ReadMessages(workspacePath, protocol.Session{
				SessionKey: sessionKey,
				AgentID:    "Amy",
				SessionID:  &sessionID,
				Options:    map[string]any{},
			}, nil)
			if err != nil {
				t.Fatalf("读取历史失败: %v", err)
			}
			for _, row := range rows {
				if row["role"] == "user" {
					t.Fatalf("缺失 marker 时 goal context transcript 不应展示成用户消息: %+v", rows)
				}
			}
			if len(rows) != 1 || rows[0]["role"] != "assistant" {
				t.Fatalf("缺失 marker 时应保留 assistant 输出并隐藏 goal context 输入: %+v", rows)
			}
		})
	}
}

// userRowInputSeqs 按 message_id 收集用户行的受理游标；缺游标的用户行直接判失败。
func userRowInputSeqs(t *testing.T, rows []protocol.Message) map[string]int64 {
	t.Helper()
	cursors := map[string]int64{}
	for _, row := range rows {
		if row["role"] != "user" {
			continue
		}
		inputSeq := protocol.Int64FromAny(row[protocol.InputSeqField])
		if inputSeq <= 0 {
			t.Fatalf("用户行缺少受理游标: %+v", row)
		}
		cursors[stringFromAny(row["message_id"])] = inputSeq
	}
	return cursors
}

func assertInputSeqs(t *testing.T, stage string, got map[string]int64, live []protocol.Message) {
	t.Helper()
	if len(got) != len(live) {
		t.Fatalf("%s: 用户行数量 = %d，期望 %d: %v", stage, len(got), len(live), got)
	}
	var previous int64
	for _, row := range live {
		messageID := stringFromAny(row["message_id"])
		want := protocol.Int64FromAny(row[protocol.InputSeqField])
		if got[messageID] != want {
			t.Fatalf("%s: %s 的游标 = %d，实时行是 %d", stage, messageID, got[messageID], want)
		}
		if want <= previous {
			t.Fatalf("%s: %s 的游标 %d 没有大于上一条输入的 %d", stage, messageID, want, previous)
		}
		previous = want
	}
}

func TestDMUserRowsKeepInputSeqAcrossRefreshAndRestart(t *testing.T) {
	f := newLongSessionFixture(t, 0, 0)
	var live []protocol.Message
	// accept 复刻 DM 受理：铸造一次游标，写进 round marker，并用实时构造器生成广播给前端的用户行。
	accept := func() {
		index := len(live)
		roundID := fmt.Sprintf("cursor-round-%d", index)
		userMessageID := "msg_user_cursor_" + fmt.Sprint(index)
		prompt := fmt.Sprintf("question %d", index)
		at := int64(1_700_000_000_000 + index*10_000)
		inputSeq := protocol.NewInputSeq()
		if err := f.history.AppendRoundMarkerWithOptions(f.workspacePath, f.session.SessionKey, roundID, prompt, at, RoundMarkerOptions{
			UserMessageID: userMessageID,
			InputSeq:      inputSeq,
		}); err != nil {
			t.Fatal(err)
		}
		userUUID, assistantUUID := "cursor-u-"+fmt.Sprint(index), "cursor-a-"+fmt.Sprint(index)
		transcriptTime := func(offsetSeconds int64) string {
			return time.UnixMilli(at + offsetSeconds*1000).UTC().Format(time.RFC3339Nano)
		}
		f.appendTranscript(t,
			map[string]any{"type": "user", "uuid": userUUID, "parentUuid": nilIfEmpty(f.lastUUID), "sessionId": *f.session.SessionID,
				"timestamp": transcriptTime(0), "message": map[string]any{"role": "user", "content": prompt}},
			map[string]any{"type": "assistant", "uuid": assistantUUID, "parentUuid": userUUID, "sessionId": *f.session.SessionID,
				"timestamp": transcriptTime(1), "message": map[string]any{"id": "cursor-m-" + fmt.Sprint(index), "role": "assistant",
					"stop_reason": "end_turn", "content": []map[string]any{{"type": "text", "text": "answer"}}}},
		)
		f.lastUUID = assistantUUID
		live = append(live, dmdomain.BuildUserRoundMarker(f.session, roundID, "", userMessageID, inputSeq, prompt, "", nil))
	}
	readAll := func() []protocol.Message {
		rows, err := f.history.ReadMessages(f.workspacePath, f.session, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	accept()
	accept()
	assertInputSeqs(t, "首次分页读取", userRowInputSeqs(t, f.readFirstPage(t).Items), live)
	assertInputSeqs(t, "首次完整读取", userRowInputSeqs(t, readAll()), live)

	accept()
	assertInputSeqs(t, "追加一轮后刷新", userRowInputSeqs(t, f.readFirstPage(t).Items), live)

	simulateHistoryProcessRestart(t, f)
	assertInputSeqs(t, "重启后分页读取", userRowInputSeqs(t, f.readFirstPage(t).Items), live)
	assertInputSeqs(t, "重启后完整读取", userRowInputSeqs(t, readAll()), live)

	accept()
	assertInputSeqs(t, "重启后再受理一轮", userRowInputSeqs(t, f.readFirstPage(t).Items), live)
}

func TestRoomUserRowsKeepInputSeqAcrossRefreshAndRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	history := newRoomHistoryTestStore(t, root)
	owner, conversation := testRoomOwnerUserID, "room-input-cursor"
	var live []protocol.Message
	// accept 复刻 Room 受理：同一个带游标的用户行既写入公区账本，也是广播给前端的实时行。
	accept := func() {
		index := len(live)
		row := protocol.Message{
			"message_id":           "msg_user_room_cursor_" + fmt.Sprint(index),
			"conversation_id":      conversation,
			"round_id":             fmt.Sprintf("room-cursor-round-%d", index),
			"role":                 "user",
			"content":              fmt.Sprintf("room question %d", index),
			"timestamp":            int64(1_700_000_000_000 + index*10_000),
			protocol.InputSeqField: protocol.NewInputSeq(),
		}
		if err := history.AppendInlineMessage(owner, conversation, row); err != nil {
			t.Fatal(err)
		}
		live = append(live, row)
	}
	readPage := func(store *RoomHistoryStore) []protocol.Message {
		page, err := store.ReadMessagesPageContext(ctx, owner, conversation, nil, HistoryPageQuery{Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		return page.Items
	}
	readAll := func(store *RoomHistoryStore) []protocol.Message {
		rows, err := store.ReadMessages(owner, conversation, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	accept()
	accept()
	assertInputSeqs(t, "首次分页读取", userRowInputSeqs(t, readPage(history)), live)
	assertInputSeqs(t, "首次完整读取", userRowInputSeqs(t, readAll(history)), live)

	accept()
	assertInputSeqs(t, "追加一条后刷新", userRowInputSeqs(t, readPage(history)), live)

	// 新的 store 与读模型连接模拟进程重启：进程内缓存全部丢弃，只剩落盘的账本与读模型。
	reopened := NewRoomHistoryStore(filepath.Join(root, "users"))
	reopened.history = newConversationHistoryStoreForTest(history.history.path)
	t.Cleanup(reopened.history.closeForTest)
	assertInputSeqs(t, "重启后分页读取", userRowInputSeqs(t, readPage(reopened)), live)
	assertInputSeqs(t, "重启后完整读取", userRowInputSeqs(t, readAll(reopened)), live)
}

// 同一条用户消息的后一份快照（如 transcript 副本或 reparent 后的重写）不带游标时，合并结果沿用受理时的值。
func TestCompactKeepsInputSeqWhenLaterUserSnapshotOmitsIt(t *testing.T) {
	accepted := protocol.Message{"message_id": "msg_user_1", "round_id": "round-1", "role": "user", "content": "hi",
		"timestamp": int64(1000), protocol.InputSeqField: int64(1_700_000_000_000_001)}
	later := protocol.Message{"message_id": "msg_user_1", "round_id": "round-1", "role": "user", "content": "hi", "timestamp": int64(2000)}
	for name, rows := range map[string][]protocol.Message{
		"带游标的快照在前": {accepted, later},
		"带游标的快照在后": {later, accepted},
	} {
		compacted := compactMessages(rows)
		if len(compacted) != 1 || protocol.Int64FromAny(compacted[0][protocol.InputSeqField]) != 1_700_000_000_000_001 {
			t.Fatalf("%s: 合并后的用户行 = %+v", name, compacted)
		}
	}
}

type parityStep struct {
	raw        map[string]any
	live       bool
	transcript bool
}

type parityRound struct {
	roundID string
	at      int64
	content string
	options RoundMarkerOptions
	steps   []parityStep
}

// parityHostLatencyMillis 大于夹具里相邻 transcript 条目的 1 秒间隔。
const parityHostLatencyMillis = 1500

func both(raw map[string]any) parityStep     { return parityStep{raw: raw, live: true, transcript: true} }
func liveOnly(raw map[string]any) parityStep { return parityStep{raw: raw, live: true} }
func diskOnly(raw map[string]any) parityStep { return parityStep{raw: raw, transcript: true} }

// 同一批事件经实时链路（EventMapper）与刷新后的历史回放（transcript + overlay）必须得到逐条一致的行：
// 多段回复、压缩边界、runtime 自唤醒和子任务都不丢失、不换轮、不乱序。
func TestLiveAndRefreshedHistoryProjectTheSameRows(t *testing.T) {
	configRoot := t.TempDir()
	workspaceRoot := filepath.Join(configRoot, "workspace")
	workspacePath := filepath.Join(workspaceRoot, "Amy")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(configRoot, "home"))
	history := NewAgentHistoryStore(workspaceRoot)
	sessionKey := "agent:Amy:ws:dm:parity"
	sessionID := "sdk-session-parity"

	rounds := liveParityRounds(sessionID)

	var liveRows []protocol.Message
	var transcript []map[string]any
	var arrival int64
	for _, round := range rounds {
		// 受理输入时铸造一次游标：marker 落盘与实时用户行用的是同一个值。
		round.options.InputSeq = protocol.NewInputSeq()
		if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, round.roundID, round.content, round.at, round.options); err != nil {
			t.Fatal(err)
		}
		// 实时链路在受理输入时用同一条 marker 物化用户行。
		if userRow, ok := materializeRoundMarkerMessage(sessionKey, "Amy", transcriptRoundMarker{
			RoundID: round.roundID, UserMessageID: round.options.UserMessageID, AgentRoundID: round.options.AgentRoundID,
			Content: round.content, Timestamp: round.at, Synthetic: round.options.Synthetic,
			Purpose: round.options.Purpose, Metadata: round.options.Metadata, InputSeq: round.options.InputSeq,
		}); ok {
			liveRows = append(liveRows, userRow)
		}
		mapper := message.NewEventMapper(message.EventMapperOptions{Context: message.MessageContext{
			SessionKey: sessionKey, AgentID: "Amy", WorkspacePath: workspacePath,
			RoundID: round.roundID, AgentRoundID: round.options.AgentRoundID, ParentID: round.options.UserMessageID,
		}, IncludeStreamLifecycle: true})
		for _, step := range round.steps {
			if step.transcript {
				transcript = append(transcript, step.raw)
			}
			if !step.live {
				continue
			}
			decoded, err := sdkprotocol.DecodeMessage(jsonRoundTrip(t, step.raw))
			if err != nil {
				t.Fatalf("解码实时消息失败: %v", err)
			}
			result, err := mapper.Map(decoded)
			if err != nil {
				t.Fatalf("实时映射失败: %v", err)
			}
			// 宿主总是晚于 runtime 写入 transcript 才收到消息；延迟超过相邻条目的间隔时，
			// 宿主接收时间与 transcript 时间交错，刷新后的顺序不能因此改变。
			if at, parseErr := time.Parse(time.RFC3339, stringFromAny(step.raw["timestamp"])); parseErr == nil {
				arrival = at.UnixMilli() + parityHostLatencyMillis
			} else {
				arrival++
			}
			for _, durable := range result.DurableMessages {
				durable["timestamp"] = arrival
				liveRows = append(liveRows, durable)
				// 与 DM 实时链路一致：transcript 原生消息不写 overlay，其余 durable 行写入。
				if protocol.IsTranscriptNativeMessage(durable) {
					continue
				}
				if err := history.AppendOverlayMessage(workspacePath, sessionKey, durable); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	writeAgentTranscriptFixture(t, workspacePath, sessionID, transcript)

	live := normalizeHistoryRows(compactMessages(liveRows), nil)
	refreshed, err := history.ReadMessages(workspacePath, protocol.Session{SessionKey: sessionKey, AgentID: "Amy", SessionID: &sessionID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != len(refreshed) {
		t.Fatalf("行数不一致: live=%d refreshed=%d\nlive=%s\nrefreshed=%s", len(live), len(refreshed), parityDump(live), parityDump(refreshed))
	}
	for index := range live {
		want, got := parityComparable(t, live[index]), parityComparable(t, refreshed[index])
		if want != got {
			t.Errorf("第 %d 行不一致:\n live      %s\n refreshed %s", index, want, got)
		}
	}
	for index, row := range refreshed {
		if row["role"] == "user" && protocol.Int64FromAny(row[protocol.InputSeqField]) != protocol.Int64FromAny(live[index][protocol.InputSeqField]) {
			t.Errorf("用户行 %v 刷新后的受理游标与实时不一致", row["message_id"])
		}
	}
	for _, id := range []string{"msg-1", "system_compact_boundary_round-1", "msg-2", "msg_user_round-wake", "msg-3", "msg-4", "msg-5"} {
		if !slices.ContainsFunc(refreshed, func(row protocol.Message) bool { return row["message_id"] == id }) {
			t.Errorf("刷新后的历史缺少 %s", id)
		}
	}
}

// liveParityRounds 是实时链路与历史回放比对共用的三轮夹具：多段回复与工具调用、压缩边界、
// runtime 自唤醒和子任务。DM 与 Room 的一致性测试都用它。
func liveParityRounds(sessionID string) []parityRound {
	stream := func(event map[string]any) parityStep {
		return liveOnly(map[string]any{"type": "stream_event", "session_id": sessionID, "event": event})
	}
	assistant := func(uuid, parent, ts, messageID, stop string, blocks ...any) parityStep {
		msg := map[string]any{"id": messageID, "role": "assistant", "model": "m", "content": blocks}
		if stop != "" {
			msg["stop_reason"] = stop
		}
		return both(map[string]any{
			"type": "assistant", "uuid": uuid, "parentUuid": parent, "session_id": sessionID, "timestamp": ts, "message": msg,
		})
	}
	rounds := []parityRound{
		{roundID: "round-1", at: 1795049999500, content: "写个报告", options: RoundMarkerOptions{UserMessageID: "msg_user_round-1", AgentRoundID: "agent-round-1"}, steps: []parityStep{
			diskOnly(map[string]any{"type": "user", "uuid": "u1", "session_id": sessionID, "timestamp": "2026-11-19T01:00:00Z",
				"message": map[string]any{"role": "user", "content": "写个报告"}}),
			stream(map[string]any{"type": "message_start", "message": map[string]any{"id": "msg-1", "model": "m"}}),
			stream(map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}}),
			stream(map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "先查资料"}}),
			stream(map[string]any{"type": "content_block_stop", "index": 0}),
			assistant("a1", "u1", "2026-11-19T01:00:01Z", "msg-1", "", map[string]any{"type": "text", "text": "先查资料"}),
			stream(map[string]any{"type": "content_block_start", "index": 1, "content_block": map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Bash", "input": map[string]any{}}}),
			stream(map[string]any{"type": "content_block_delta", "index": 1, "delta": map[string]any{"type": "input_json_delta", "partial_json": `{"command":"ls"}`}}),
			stream(map[string]any{"type": "content_block_stop", "index": 1}),
			assistant("a2", "a1", "2026-11-19T01:00:02Z", "msg-1", "tool_use", map[string]any{"type": "tool_use", "id": "toolu_1", "name": "Bash", "input": map[string]any{"command": "ls"}}),
			stream(map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "tool_use"}}),
			stream(map[string]any{"type": "message_stop"}),
			both(map[string]any{"type": "user", "uuid": "tr1", "parentUuid": "a2", "session_id": sessionID, "timestamp": "2026-11-19T01:00:03Z",
				"message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_1", "content": "a.txt"}}}}),
			both(map[string]any{"type": "system", "subtype": "compact_boundary", "uuid": "cb1", "parentUuid": nil, "logicalParentUuid": "tr1", "session_id": sessionID, "timestamp": "2026-11-19T01:00:04Z",
				"compactMetadata": map[string]any{"trigger": "auto", "preTokens": 1000}}),
			diskOnly(map[string]any{"type": "user", "uuid": "cs1", "parentUuid": "cb1", "isCompactSummary": true, "session_id": sessionID, "timestamp": "2026-11-19T01:00:04Z",
				"message": map[string]any{"role": "user", "content": "summary of earlier conversation"}}),
			stream(map[string]any{"type": "message_start", "message": map[string]any{"id": "msg-2", "model": "m"}}),
			stream(map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}}),
			stream(map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": "报告完成"}}),
			stream(map[string]any{"type": "content_block_stop", "index": 0}),
			assistant("a3", "cs1", "2026-11-19T01:00:05Z", "msg-2", "end_turn", map[string]any{"type": "text", "text": "报告完成"}),
			stream(map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}}),
			stream(map[string]any{"type": "message_stop"}),
			liveOnly(map[string]any{"type": "result", "subtype": "success", "uuid": "r1", "session_id": sessionID, "result": "报告完成", "duration_ms": 10, "num_turns": 2}),
		}},
		{roundID: "round-wake", at: 1795050059500, content: runtimeWakeTestSummary, options: RoundMarkerOptions{
			UserMessageID: "msg_user_round-wake", AgentRoundID: "agent-round-wake", Synthetic: true, Purpose: "runtime_wake",
			Metadata: map[string]string{"input_kind": "runtime_wake", "trigger_kind": "background_task", "task_id": "b1", "task_status": "completed"},
		}, steps: []parityStep{
			diskOnly(map[string]any{"type": "user", "uuid": "wake-1", "parentUuid": "a3", "session_id": sessionID, "timestamp": "2026-11-19T01:01:00Z",
				"origin":  map[string]any{"kind": "task-notification"},
				"message": map[string]any{"role": "user", "content": "<task-notification>\n<task-id>b1</task-id>\n<status>completed</status>\n<summary>" + runtimeWakeTestSummary + "</summary>\n</task-notification>"}}),
			assistant("a4", "wake-1", "2026-11-19T01:01:03Z", "msg-3", "end_turn", map[string]any{"type": "text", "text": "第二阶段开始"}),
			liveOnly(map[string]any{"type": "result", "subtype": "success", "uuid": "r2", "session_id": sessionID, "result": "第二阶段开始", "duration_ms": 10, "num_turns": 1}),
		}},
		{roundID: "round-3", at: 1795050119500, content: "跑子任务", options: RoundMarkerOptions{UserMessageID: "msg_user_round-3", AgentRoundID: "agent-round-3"}, steps: []parityStep{
			diskOnly(map[string]any{"type": "user", "uuid": "u3", "parentUuid": "a4", "session_id": sessionID, "timestamp": "2026-11-19T01:02:00Z",
				"message": map[string]any{"role": "user", "content": "跑子任务"}}),
			assistant("a5", "u3", "2026-11-19T01:02:01Z", "msg-4", "tool_use", map[string]any{"type": "tool_use", "id": "toolu_agent", "name": "Agent", "input": map[string]any{"description": "调研", "prompt": "调研一下"}}),
			both(map[string]any{"type": "attachment", "uuid": "at1", "parentUuid": "a5", "session_id": sessionID, "timestamp": "2026-11-19T01:02:02Z",
				"attachment": map[string]any{"type": "structured_output", "agentId": "agent-child-1", "agentType": "Explore", "description": "调研", "status": "completed", "toolUseId": "toolu_agent"}}),
			both(map[string]any{"type": "user", "uuid": "tr3", "parentUuid": "a5", "session_id": sessionID, "timestamp": "2026-11-19T01:02:03Z",
				"message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "toolu_agent", "content": "调研结论"}}}}),
			assistant("a6", "tr3", "2026-11-19T01:02:04Z", "msg-5", "end_turn", map[string]any{"type": "text", "text": "子任务完成"}),
			liveOnly(map[string]any{"type": "result", "subtype": "success", "uuid": "r3", "session_id": sessionID, "result": "子任务完成", "duration_ms": 10, "num_turns": 2}),
		}},
	}
	return rounds
}

func jsonRoundTrip(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded := map[string]any{}
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// parityComparable 去掉两条链路天然不同的时钟与来源位置：实时行用宿主接收时间，
// 历史行用 transcript 时间和行号；其余字段必须完全一致。
func parityComparable(t *testing.T, row protocol.Message) string {
	t.Helper()
	comparable := protocol.Clone(row)
	delete(comparable, "timestamp")
	delete(comparable, message.TranscriptLineField)
	if comparable["role"] == "user" {
		// 受理输入时 runtime 还没有登记 SDK session，只有回放能补上它。
		delete(comparable, "session_id")
	}
	if summary, ok := comparable["result_summary"].(map[string]any); ok {
		summary = maps.Clone(summary)
		delete(summary, "timestamp")
		comparable["result_summary"] = summary
	}
	encoded, err := json.Marshal(comparable)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func parityDump(rows []protocol.Message) string {
	encoded, _ := json.Marshal(rows)
	return string(encoded)
}

// 带子任务进度的回复不是 transcript 原生消息，宿主会另写一份 overlay 副本。
// 宿主收到它的时间晚于 runtime 记录压缩边界的时间时，刷新后压缩提示仍须排在这条回复之后。
func TestHostCopiesKeepTranscriptOrderAcrossCompaction(t *testing.T) {
	reply := func(timestamp int64, positioned bool) protocol.Message {
		row := protocol.Message{
			"message_id": "msg-progress", "role": "assistant", "round_id": "round-1", "timestamp": timestamp,
			"content": []any{map[string]any{"type": "task_progress", "task_id": "task-1"}},
		}
		if positioned {
			row[message.TranscriptLineField] = 3
		}
		return row
	}
	notice := func(timestamp int64, positioned bool) protocol.Message {
		row := protocol.Message{
			"message_id": "system_compact_boundary_round-1", "role": "system", "round_id": "round-1", "timestamp": timestamp,
		}
		if positioned {
			row[message.TranscriptLineField] = 5
		}
		return row
	}
	transcript := []protocol.Message{reply(1000, true), notice(2000, true)}
	overlay := []protocol.Message{reply(1000+parityHostLatencyMillis, false), notice(2000+parityHostLatencyMillis, false)}

	for name, rows := range map[string][]protocol.Message{
		"transcript 副本在前": slices.Concat(transcript, overlay),
		"overlay 副本在前":    slices.Concat(overlay, transcript),
	} {
		compacted := compactMessages(rows)
		if len(compacted) != 2 || compacted[0]["message_id"] != "msg-progress" ||
			compacted[1]["message_id"] != "system_compact_boundary_round-1" {
			t.Errorf("%s: 顺序 = %s", name, parityDump(compacted))
			continue
		}
		for index, want := range []int64{1000, 2000} {
			if got := messageTimestamp(compacted[index]); got != want {
				t.Errorf("%s: 第 %d 行时间 = %d, want %d", name, index, got, want)
			}
		}
	}
}

func TestNormalizeHistoryRowsKeepsResultUsageWhenSuppressingDuplicateResultText(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "你好",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"parent_id":   "round-1",
			"role":        "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "你好，世界",
				},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   2000,
		},
		{
			"message_id":      "result-1",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "round-1",
			"role":            "result",
			"subtype":         "success",
			"result":          "你好，世界",
			"timestamp":       3000,
			"duration_ms":     1200,
			"duration_api_ms": 1100,
			"num_turns":       1,
			"usage": map[string]any{
				"input_tokens":  3,
				"output_tokens": 5,
			},
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("历史消息数量不正确: got=%d want=2", len(normalized))
	}

	assistant := normalized[1]
	resultSummary, ok := assistant["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("assistant 应挂载 result_summary: %+v", assistant)
	}
	if _, exists := resultSummary["result"]; exists {
		t.Fatalf("重复 result 文本应被移除: %+v", resultSummary)
	}
	usage, ok := resultSummary["usage"].(map[string]any)
	if !ok {
		t.Fatalf("result usage 不应丢失: %+v", resultSummary)
	}
	if intFromAny(usage["output_tokens"]) != 5 {
		t.Fatalf("result usage 不正确: %+v", usage)
	}
}

func TestNormalizeHistoryRowsFiltersInternalTransportRows(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "system-retry",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "system",
			"content":     "API 正在重试",
			"timestamp":   900,
			"metadata": map[string]any{
				"subtype": "api_retry",
			},
		},
		{
			"message_id":  "user-internal",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "[Request interrupted by user]",
			"timestamp":   950,
		},
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-2",
			"role":        "user",
			"content":     "继续",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-2",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "好的，继续。"},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   1100,
		},
		{
			"message_id":      "result-1",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "round-2",
			"role":            "result",
			"subtype":         "success",
			"timestamp":       1200,
			"duration_ms":     10,
			"duration_api_ms": 8,
			"num_turns":       1,
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 2,
			},
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("内部 transport 行应被过滤: got=%d want=2 rows=%+v", len(normalized), normalized)
	}
	if normalized[0]["role"] != "user" || stringFromAny(normalized[0]["content"]) != "继续" {
		t.Fatalf("内部 user prompt 未被过滤: %+v", normalized)
	}
	if normalized[1]["role"] != "assistant" {
		t.Fatalf("结果应继续收口到 assistant: %+v", normalized)
	}
}

func TestNormalizeHistoryRowsSuppressesTranscriptGuidanceWhenDurableUserExists(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":      "source-round",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "root-round",
			"role":            "system",
			"content":         "然后给点建议",
			"timestamp":       1000,
			"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
			"metadata": map[string]any{
				"subtype":         message.SystemMessageSubtypeGuidedInput,
				"source_round_id": "source-round",
			},
		},
		{
			"message_id":      "user-source-round",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "root-round",
			"source_round_id": "source-round",
			"role":            "user",
			"content":         "然后给点建议",
			"timestamp":       1001,
			"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	userCount := 0
	for _, row := range normalized {
		if row["role"] == "system" {
			t.Fatalf("同一引导不应保留 transcript system 投影: %+v", normalized)
		}
		if row["role"] == "user" && row["source_round_id"] == "source-round" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("同一引导应只保留一条 durable user 投影: %+v", normalized)
	}
}

func TestSortHistoryRowsUsesDisplayOrderForSameTimestamp(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "assistant-b", "role": "assistant", "round_id": "root", "timestamp": int64(10), "display_order": int64(10001)},
		{"message_id": "assistant-a", "role": "assistant", "round_id": "root", "timestamp": int64(10), "display_order": int64(10000)},
	}
	sortHistoryRows(rows)
	if rows[0]["message_id"] != "assistant-a" || rows[1]["message_id"] != "assistant-b" {
		t.Fatalf("同一时间应按 display_order 稳定排序: %#v", rows)
	}
}

func TestCompactMessagesMergesGoalCompletionReceiptIntoOriginalAssistant(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"round_id":   "round-1",
			"content":    []map[string]any{{"type": "text", "text": "最终交付"}},
			"timestamp":  int64(1000),
		},
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"round_id":   "round-1",
			protocol.GoalCompletionReceiptField: protocol.GoalCompletionReceipt{
				GoalID: "goal-1", RoundID: "round-1", ActualTokens: int64TestPointer(42),
			},
			"timestamp": int64(1000),
		},
	}

	compacted := compactMessages(rows)
	if len(compacted) != 1 {
		t.Fatalf("compacted = %+v, want one assistant", compacted)
	}
	if compacted[0][protocol.GoalCompletionReceiptField] == nil {
		t.Fatalf("completion receipt was lost: %+v", compacted[0])
	}
	blocks := normalizeMessageContentBlocks(compacted[0]["content"])
	if len(blocks) != 1 || stringFromAny(blocks[0]["text"]) != "最终交付" {
		t.Fatalf("original assistant content was lost: %+v", compacted[0])
	}
}

func TestSortHistoryRowsPlacesGoalControlBeforeContinuationAtSameMillisecond(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "msg_a", "round_id": "goal-continuation", "role": "assistant", "timestamp": int64(1000)},
		{
			"message_id":   "msg_z",
			"round_id":     "goal-command",
			"role":         "user",
			"timestamp":    int64(1000),
			"control_only": true,
			"metadata":     map[string]string{"subtype": "goal_set"},
		},
	}

	sortHistoryRows(rows)

	if rows[0]["message_id"] != "msg_z" || rows[1]["message_id"] != "msg_a" {
		t.Fatalf("same-millisecond Goal order = %#v, want control before continuation", rows)
	}
}

func int64TestPointer(value int64) *int64 { return &value }

func TestBuildRoomTranscriptReferenceRejectsResultRole(t *testing.T) {
	row := buildRoomTranscriptReference(protocol.Message{
		"message_id": "result-1",
		"session_id": "sdk-session-1",
		"agent_id":   "nexus",
		"round_id":   "round-1",
		"role":       "result",
		"timestamp":  1000,
	}, "/tmp/workspace", "agent:nexus:ws:dm:test")
	if row != nil {
		t.Fatalf("result 行不应生成 transcript_ref: %+v", row)
	}
}
