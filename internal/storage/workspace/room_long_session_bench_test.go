package workspace

import (
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

// roomLongSessionFixture 是只有一个成员的长 Room：每轮一条公区用户消息、成员 transcript 里的
// 一个私有轮次（提问、工具调用、工具结果、回复），以及指向回复的 transcript_ref。
type roomLongSessionFixture struct {
	history       *RoomHistoryStore
	stateRoot     string
	owner         string
	conversation  string
	workspacePath string
	sessionKey    string
	sessionID     string
	transcript    string
	lastUUID      string
	rounds        int
	toolBytes     int
}

func newRoomLongSessionFixture(tb testing.TB, rounds int, toolBytes int) *roomLongSessionFixture {
	tb.Helper()
	stateRoot := tb.TempDir()
	tb.Setenv("NEXUS_STATE_ROOT", stateRoot)
	tb.Setenv("NEXUS_CONFIG_DIR", "")
	fixture := &roomLongSessionFixture{
		history:      NewRoomHistoryStore(filepath.Join(stateRoot, "users")),
		stateRoot:    stateRoot,
		owner:        testRoomOwnerUserID,
		conversation: "long-room",
		sessionID:    "5d928f9e-31bd-48a0-9cd5-21a804772001",
		toolBytes:    toolBytes,
	}
	fixture.workspacePath = filepath.Join(appfs.UserWorkspaceRootAt(stateRoot, fixture.owner), "Amy")
	fixture.sessionKey = "agent:amy:ws:group:" + fixture.conversation
	if err := os.MkdirAll(fixture.workspacePath, 0o755); err != nil {
		tb.Fatal(err)
	}
	if _, err := fixture.private().files.UpsertSession(fixture.workspacePath, protocol.Session{
		SessionKey: fixture.sessionKey,
		AgentID:    "amy",
		SessionID:  &fixture.sessionID,
	}); err != nil {
		tb.Fatal(err)
	}
	projectDir := filepath.Join(transcriptProjectsDirForWorkspace(fixture.workspacePath), TranscriptProjectDirectoryName(fixture.workspacePath))
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	fixture.transcript = filepath.Join(projectDir, fixture.sessionID+".jsonl")
	for range rounds {
		fixture.appendRound(tb)
	}
	return fixture
}

func (f *roomLongSessionFixture) private() *AgentHistoryStore {
	return f.history.agentHistory.ForOwner(f.owner)
}

func (f *roomLongSessionFixture) roundTimestamp(round int) int64 {
	return int64(1_700_000_000_000 + round*10_000)
}

// replyID 是第 round 轮成员最终回复的 message_id，也是公区引用的消息。
func (f *roomLongSessionFixture) replyID(round int) string {
	return fmt.Sprintf("m2-%05d", round)
}

// appendRound 追加完整的一轮：公区提问、成员私有轮次与公区引用。
func (f *roomLongSessionFixture) appendRound(tb testing.TB) {
	tb.Helper()
	rows := f.startRound(tb)
	f.appendTranscript(tb, rows...)
	f.publishReply(tb, f.rounds-1)
}

// startRound 写入下一轮的公区提问和成员 marker，并返回该轮尚未写入的 transcript 行，供测试分段追加。
func (f *roomLongSessionFixture) startRound(tb testing.TB) []map[string]any {
	tb.Helper()
	round := f.rounds
	f.rounds++
	timestamp := f.roundTimestamp(round)
	rootRound := fmt.Sprintf("root-%05d", round)
	prompt := "question " + rootRound
	if err := f.history.AppendInlineMessage(f.owner, f.conversation, protocol.Message{
		"message_id": "user-" + rootRound, "round_id": rootRound, "role": "user", "content": prompt, "timestamp": timestamp,
	}); err != nil {
		tb.Fatal(err)
	}
	if err := f.private().AppendRoundMarker(f.workspacePath, f.sessionKey, fmt.Sprintf("private-%05d", round), prompt, timestamp); err != nil {
		tb.Fatal(err)
	}
	ts := func(offset int) string {
		return time.UnixMilli(timestamp + int64(offset)*1000).UTC().Format(time.RFC3339Nano)
	}
	uuid := func(kind string) string { return fmt.Sprintf("%s-%05d", kind, round) }
	rows := []map[string]any{
		{"type": "user", "uuid": uuid("u"), "parentUuid": nilIfEmpty(f.lastUUID), "sessionId": f.sessionID, "timestamp": ts(0),
			"message": map[string]any{"role": "user", "content": prompt}},
		{"type": "assistant", "uuid": uuid("a1"), "parentUuid": uuid("u"), "sessionId": f.sessionID, "timestamp": ts(1),
			"message": map[string]any{"id": uuid("m1"), "role": "assistant", "stop_reason": "tool_use",
				"content": []map[string]any{{"type": "tool_use", "id": uuid("t"), "name": "Bash", "input": map[string]any{"command": "ls -la"}}}}},
		{"type": "user", "uuid": uuid("r"), "parentUuid": uuid("a1"), "sessionId": f.sessionID, "timestamp": ts(2),
			"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": uuid("t"),
				"content": strings.Repeat("x", f.toolBytes)}}}},
		{"type": "assistant", "uuid": uuid("a2"), "parentUuid": uuid("r"), "sessionId": f.sessionID, "timestamp": ts(3),
			"message": map[string]any{"id": f.replyID(round), "role": "assistant", "stop_reason": "end_turn",
				"content": []map[string]any{{"type": "text", "text": strings.Repeat("answer ", 128)}}}},
	}
	f.lastUUID = uuid("a2")
	return rows
}

func (f *roomLongSessionFixture) appendTranscript(tb testing.TB, rows ...map[string]any) {
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
}

// publishReply 把第 round 轮的成员回复以 transcript_ref 发布到公区。
func (f *roomLongSessionFixture) publishReply(tb testing.TB, round int) {
	tb.Helper()
	if err := f.history.AppendTranscriptReference(f.owner, f.conversation, f.workspacePath, f.sessionKey, protocol.Message{
		"message_id": f.replyID(round), "session_id": f.sessionID, "conversation_id": f.conversation, "agent_id": "amy",
		"round_id": fmt.Sprintf("root-%05d:amy", round), "agent_round_id": fmt.Sprintf("private-%05d", round),
		"role": "assistant", "is_complete": true, "timestamp": f.roundTimestamp(round) + 3000,
	}); err != nil {
		tb.Fatal(err)
	}
}

func (f *roomLongSessionFixture) readFirstPage(tb testing.TB) protocol.MessagePage {
	tb.Helper()
	scope := f.history.historyPageAccess(f.owner, f.conversation).Scope
	for {
		page, err := f.history.ReadMessagesPageContext(context.Background(), f.owner, f.conversation, nil, HistoryPageQuery{Limit: 20})
		if err != nil {
			tb.Fatal(err)
		}
		waitHistoryPageIndexFutureTB(tb, scope)
		if !page.Indexing {
			return page
		}
	}
}

// restart 丢弃所有进程内状态：解析缓存、投影检查点、读模型连接与落盘备忘。
func (f *roomLongSessionFixture) restart(tb testing.TB) {
	tb.Helper()
	root := filepath.Join(f.stateRoot, "users")
	closeHistoryReadModelForRestart(root)
	f.history = NewRoomHistoryStore(root)
}

func (f *roomLongSessionFixture) requireLatestReply(tb testing.TB, page protocol.MessagePage) {
	tb.Helper()
	if want := f.replyID(f.rounds - 1); !historyPageHasMessage(page, want) {
		tb.Fatalf("page missing %s", want)
	}
}

// BenchmarkRoomLongSessionHistoryAfterAppend 模拟长 Room 每轮成员回复后重新读取首页。
func BenchmarkRoomLongSessionHistoryAfterAppend(b *testing.B) {
	fixture := newRoomLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	fixture.readFirstPage(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		fixture.appendRound(b)
		b.StartTimer()
		fixture.requireLatestReply(b, fixture.readFirstPage(b))
	}
}

// BenchmarkRoomLongSessionHistoryCold 用新的 store 读取首页，覆盖读模型的完整重建。
func BenchmarkRoomLongSessionHistoryCold(b *testing.B) {
	fixture := newRoomLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		fixture.restart(b)
		if err := os.RemoveAll(filepath.Dir(historyReadModelPath(filepath.Join(fixture.stateRoot, "users")))); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		fixture.requireLatestReply(b, fixture.readFirstPage(b))
	}
}

// BenchmarkRoomLongSessionHistoryRestartAppended 重启后成员又回复了一轮：读模型里的成员检查点仍可续接。
func BenchmarkRoomLongSessionHistoryRestartAppended(b *testing.B) {
	fixture := newRoomLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	fixture.readFirstPage(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		fixture.restart(b)
		fixture.appendRound(b)
		b.StartTimer()
		fixture.requireLatestReply(b, fixture.readFirstPage(b))
	}
}
