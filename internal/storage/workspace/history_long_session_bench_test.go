package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

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
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	waitHistoryPageIndexFutureTB(tb, access.Scope)
	return page
}

func waitHistoryPageIndexFutureTB(tb testing.TB, scope string) {
	tb.Helper()
	for {
		historyPageRebuilds.mu.Lock()
		future := historyPageRebuilds.futures[scope]
		historyPageRebuilds.mu.Unlock()
		if future == nil {
			return
		}
		<-future.complete
	}
}

func benchmarkLongSessionRounds() int {
	if value, err := strconv.Atoi(os.Getenv("NEXUS_BENCH_ROUNDS")); err == nil && value > 0 {
		return value
	}
	return 400
}

// BenchmarkLongSessionHistoryAfterAppend 模拟长会话每轮结束后重新读取首页。
func BenchmarkLongSessionHistoryAfterAppend(b *testing.B) {
	fixture := newLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	fixture.readFirstPage(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		fixture.appendRound(b)
		b.StartTimer()
		page := fixture.readFirstPage(b)
		if len(page.Items) == 0 {
			b.Fatal("empty page")
		}
	}
}

// BenchmarkLongSessionHistoryWarm 读取未变化会话的首页。
func BenchmarkLongSessionHistoryWarm(b *testing.B) {
	fixture := newLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	fixture.readFirstPage(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		fixture.readFirstPage(b)
	}
}

// BenchmarkLongSessionRoundIndexWarm 从读模型读取未变化会话的完整 round 导航。
func BenchmarkLongSessionRoundIndexWarm(b *testing.B) {
	fixture := newLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	fixture.readFirstPage(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		index, err := fixture.history.ReadRoundIndexPageContext(context.Background(), fixture.workspacePath, fixture.session, nil, false)
		if err != nil {
			b.Fatal(err)
		}
		if len(index.Items) != fixture.rounds {
			b.Fatalf("index=%d want %d", len(index.Items), fixture.rounds)
		}
	}
}

// BenchmarkLongSessionHistoryCold 用新的 store 读取首页，覆盖重启后的首次打开。
func BenchmarkLongSessionHistoryCold(b *testing.B) {
	fixture := newLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		resetHistoryReadModelsForBenchmark(b, fixture)
		b.StartTimer()
		fixture.readFirstPage(b)
	}
}

func resetHistoryReadModelsForBenchmark(tb testing.TB, fixture *longSessionFixture) {
	tb.Helper()
	fixture.history = NewAgentHistoryStore(filepath.Dir(fixture.workspacePath))
	fixture.appendRound(tb)
}

// simulateHistoryProcessRestart 丢弃所有进程内状态：解析缓存、投影检查点、读模型连接与落盘备忘。
func simulateHistoryProcessRestart(tb testing.TB, fixture *longSessionFixture) {
	tb.Helper()
	path := historyReadModelPath(filepath.Dir(fixture.workspacePath))
	if value, ok := historyReadModels.LoadAndDelete(path); ok {
		model := value.(*historyReadModel)
		model.mu.Lock()
		if model.db != nil {
			_ = model.db.Close()
		}
		model.mu.Unlock()
	}
	fixture.history = NewAgentHistoryStore(filepath.Dir(fixture.workspacePath))
}

// BenchmarkLongSessionHistoryRestartUnchanged 重启后打开未变化的长会话，应直接命中落盘读模型。
func BenchmarkLongSessionHistoryRestartUnchanged(b *testing.B) {
	fixture := newLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	fixture.readFirstPage(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		simulateHistoryProcessRestart(b, fixture)
		b.StartTimer()
		fixture.readFirstPage(b)
	}
}

// BenchmarkLongSessionHistoryRestartAppended 重启前 transcript 已追加但读模型未更新（崩溃或关闭前未落盘）。
func BenchmarkLongSessionHistoryRestartAppended(b *testing.B) {
	fixture := newLongSessionFixture(b, benchmarkLongSessionRounds(), 8*1024)
	fixture.readFirstPage(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		simulateHistoryProcessRestart(b, fixture)
		fixture.appendRound(b)
		b.StartTimer()
		fixture.readFirstPage(b)
	}
}

// BenchmarkLongSessionOverlayAfterAppend 追加一轮 marker 后读取 overlay 状态与 round 导航，
// 单独衡量每轮刷新里 overlay 两条读取链路的成本。
func BenchmarkLongSessionOverlayAfterAppend(b *testing.B) {
	fixture := newLongSessionFixture(b, benchmarkLongSessionRounds(), 16)
	read := func() {
		ctx := context.Background()
		state, err := fixture.history.readOverlayHistoryStateContext(ctx, fixture.workspacePath, fixture.session.SessionKey)
		if err != nil {
			b.Fatal(err)
		}
		index, err := fixture.history.readCanonicalRoundIndexContext(ctx, fixture.workspacePath, fixture.session, nil)
		if err != nil {
			b.Fatal(err)
		}
		if len(state.RoundMarkers) != fixture.rounds || len(index.Items) != fixture.rounds {
			b.Fatalf("markers=%d index=%d want %d", len(state.RoundMarkers), len(index.Items), fixture.rounds)
		}
	}
	read()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		fixture.startRound(b)
		b.StartTimer()
		read()
	}
}
