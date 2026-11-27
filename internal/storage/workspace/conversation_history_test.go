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
	"strings"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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
