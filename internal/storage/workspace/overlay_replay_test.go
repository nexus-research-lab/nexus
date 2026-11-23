package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// overlayReplayHarness 让同一个 store 走增量回放，再与整份重读的结果逐字段比较。
type overlayReplayHarness struct {
	t             *testing.T
	history       *AgentHistoryStore
	workspacePath string
	session       protocol.Session
	overlayPath   string
}

func newOverlayReplayHarness(t *testing.T) *overlayReplayHarness {
	t.Helper()
	return newOverlayReplayHarnessFor(t, newLongSessionFixture(t, 0, 16))
}

func newOverlayReplayHarnessFor(t *testing.T, fixture *longSessionFixture) *overlayReplayHarness {
	t.Helper()
	overlayPath := fixture.history.paths.SessionOverlayPath(fixture.workspacePath, fixture.session.SessionKey)
	if err := os.MkdirAll(filepath.Dir(overlayPath), 0o755); err != nil {
		t.Fatal(err)
	}
	return &overlayReplayHarness{
		t:             t,
		history:       fixture.history,
		workspacePath: fixture.workspacePath,
		session:       fixture.session,
		overlayPath:   overlayPath,
	}
}

func (h *overlayReplayHarness) appendRaw(raw string) {
	h.t.Helper()
	file, err := os.OpenFile(h.overlayPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		h.t.Fatal(err)
	}
	defer file.Close()
	if _, err = file.WriteString(raw); err != nil {
		h.t.Fatal(err)
	}
}

func (h *overlayReplayHarness) appendRows(rows ...map[string]any) {
	h.t.Helper()
	var builder strings.Builder
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			h.t.Fatal(err)
		}
		builder.Write(payload)
		builder.WriteByte('\n')
	}
	h.appendRaw(builder.String())
}

// assertMatchesFullRead 比较增量回放与整份重读；整份重读用不带缓存的原始读取函数。
func (h *overlayReplayHarness) assertMatchesFullRead(active []string) {
	h.t.Helper()
	ctx := context.Background()
	gotState, err := h.history.readOverlayHistoryStateContext(ctx, h.workspacePath, h.session.SessionKey)
	if err != nil {
		h.t.Fatalf("replayed overlay state: %v", err)
	}
	rows, err := h.readAllRows(ctx)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		h.t.Fatalf("full overlay read: %v", err)
	}
	if wantState := overlayHistoryStateFromRows(rows); !reflect.DeepEqual(gotState, wantState) {
		h.t.Fatalf("overlay state diverged from full read\n got: %#v\nwant: %#v", gotState, wantState)
	}

	gotIndex, gotErr := h.history.readCanonicalRoundIndexContext(ctx, h.workspacePath, h.session, active)
	root, err := h.history.files.openWorkspaceRoot(h.workspacePath, false)
	if err != nil {
		h.t.Fatal(err)
	}
	defer root.Close()
	relative, err := filepath.Rel(h.workspacePath, h.overlayPath)
	if err != nil {
		h.t.Fatal(err)
	}
	wantIndex, wantErr := readRoundIndexFromRootContext(
		ctx, root, filepath.ToSlash(relative), normalizeActiveRoundIDs(active), false, h.session.AgentID,
	)
	if (gotErr == nil) != (wantErr == nil) {
		h.t.Fatalf("round index error diverged: got %v want %v", gotErr, wantErr)
	}
	if !reflect.DeepEqual(gotIndex, wantIndex) {
		h.t.Fatalf("round index diverged from full read\n got: %#v\nwant: %#v", gotIndex, wantIndex)
	}
}

func (h *overlayReplayHarness) readAllRows(ctx context.Context) ([]map[string]any, error) {
	file, err := openWorkspaceFileAt(h.history.files, h.workspacePath, h.overlayPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readJSONLFileWithContext(ctx, file)
}

func (h *overlayReplayHarness) replayEntry() *overlayReplayEntry {
	return h.history.cache.overlays.entry(h.history.ownerUserID + "\x00" + h.overlayPath)
}

// randomOverlayRow 覆盖 round index 与历史行会区分的全部行形态：可见/隐藏 marker、
// 用户行、assistant/result 行（含重复 message_id）、result_summary 与控制行。
func randomOverlayRow(random *rand.Rand, sequence int) map[string]any {
	roundID := fmt.Sprintf("round-%02d", random.Intn(24))
	timestamp := int64(0)
	if random.Intn(8) != 0 {
		timestamp = int64(1_700_000_000_000 + random.Intn(40)*1000)
	}
	row := map[string]any{"round_id": roundID, "timestamp": timestamp}
	if random.Intn(3) == 0 {
		row["message_id"] = fmt.Sprintf("msg-%02d", random.Intn(30))
	}
	switch random.Intn(8) {
	case 0, 1:
		row[overlayKindField] = overlayKindRoundMarker
		row["content"] = fmt.Sprintf("prompt %d", sequence)
		row["hidden_from_user"] = random.Intn(5) == 0
		row["user_message_id"] = fmt.Sprintf("user-%d", sequence)
	case 2:
		row["role"] = "user"
		row["content"] = fmt.Sprintf("typed %d", sequence)
	case 3:
		row["role"] = "assistant"
		row["agent_id"] = []string{"", "amy", "bob"}[random.Intn(3)]
		row["content"] = []map[string]any{{"type": "text", "text": "answer"}}
	case 4, 5:
		row["role"] = "result"
		row["subtype"] = []string{"success", "error", "interrupted", ""}[random.Intn(4)]
		row["duration_ms"] = random.Intn(3) * 1500
	case 6:
		row["role"] = "system"
		row["result_summary"] = map[string]any{"subtype": "success", "duration_ms": 1200}
	default:
		row[overlayKindField] = []string{"history_rewrite", overlayKindRoomPublicCursor, "room_context_checkpoint"}[random.Intn(3)]
	}
	return row
}

func TestOverlayReplayMatchesFullReadAcrossRandomAppends(t *testing.T) {
	for seed := int64(1); seed <= 12; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			harness := newOverlayReplayHarness(t)
			random := rand.New(rand.NewSource(seed))
			harness.assertMatchesFullRead(nil)
			sequence := 0
			for step := 0; step < 60; step++ {
				rows := make([]map[string]any, 1+random.Intn(4))
				for index := range rows {
					rows[index] = randomOverlayRow(random, sequence)
					sequence++
				}
				harness.appendRows(rows...)
				var active []string
				if random.Intn(2) == 0 {
					active = []string{fmt.Sprintf("round-%02d", random.Intn(24)), "round-missing"}
				}
				harness.assertMatchesFullRead(active)
			}
		})
	}
}

func TestOverlayReplayParsesOnlyAppendedRows(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	for round := 0; round < 50; round++ {
		harness.appendRows(
			map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": fmt.Sprintf("round-%03d", round), "content": "hi", "timestamp": 1_700_000_000_000 + round*1000},
			map[string]any{"role": "result", "round_id": fmt.Sprintf("round-%03d", round), "message_id": fmt.Sprintf("result-%03d", round), "subtype": "success", "timestamp": 1_700_000_000_500 + round*1000},
		)
	}
	harness.assertMatchesFullRead(nil)
	info, err := os.Stat(harness.overlayPath)
	if err != nil {
		t.Fatal(err)
	}
	harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-050", "content": "next", "timestamp": 1_700_000_050_000})

	file, err := os.Open(harness.overlayPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entry := harness.replayEntry()
	for name, cursor := range map[string]*overlayReplayCursor{"history": entry.history.cursor, "round index": entry.roundIndex.cursor} {
		span, err := openOverlayReplaySpan(context.Background(), file, cursor)
		if err != nil {
			t.Fatal(err)
		}
		if !span.Resumed || span.Reader.Size() >= info.Size() || span.Next.Size-span.Reader.Size() != info.Size() {
			t.Fatalf("%s replay should resume at the previous end %d: resumed=%t span=%d", name, info.Size(), span.Resumed, span.Reader.Size())
		}
	}
	harness.assertMatchesFullRead(nil)
}

func TestOverlayReplayFallsBackToFullRead(t *testing.T) {
	seed := func(harness *overlayReplayHarness) {
		for round := 0; round < 6; round++ {
			harness.appendRows(
				map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": fmt.Sprintf("round-%d", round), "content": fmt.Sprintf("prompt %d", round), "timestamp": 1_700_000_000_000 + round*1000},
				map[string]any{"role": "result", "round_id": fmt.Sprintf("round-%d", round), "message_id": fmt.Sprintf("result-%d", round), "subtype": "success", "duration_ms": 900, "timestamp": 1_700_000_000_500 + round*1000},
			)
		}
		harness.assertMatchesFullRead(nil)
	}
	rewrite := func(harness *overlayReplayHarness, edit func(lines []string) []string) {
		harness.t.Helper()
		payload, err := os.ReadFile(harness.overlayPath)
		if err != nil {
			harness.t.Fatal(err)
		}
		lines := edit(strings.SplitAfter(strings.TrimSuffix(string(payload), "\n"), "\n"))
		if err = os.WriteFile(harness.overlayPath, []byte(strings.Join(lines, "")+"\n"), 0o644); err != nil {
			harness.t.Fatal(err)
		}
	}
	cases := map[string]func(harness *overlayReplayHarness){
		"later row overwrites an applied message_id": func(harness *overlayReplayHarness) {
			harness.appendRows(map[string]any{"role": "result", "round_id": "round-9", "message_id": "result-2", "subtype": "error", "timestamp": 1_700_000_090_000})
		},
		"file replaced by a new file": func(harness *overlayReplayHarness) {
			replacement := harness.overlayPath + ".next"
			if err := os.WriteFile(replacement, []byte(`{"nexus_overlay_kind":"round_marker","round_id":"round-new","content":"fresh","timestamp":1700000000000}`+"\n"), 0o644); err != nil {
				harness.t.Fatal(err)
			}
			if err := os.Rename(replacement, harness.overlayPath); err != nil {
				harness.t.Fatal(err)
			}
		},
		"truncated in place": func(harness *overlayReplayHarness) {
			rewrite(harness, func(lines []string) []string { return lines[:4] })
		},
		"rewritten in place with more rows": func(harness *overlayReplayHarness) {
			rewrite(harness, func(lines []string) []string {
				lines[len(lines)-1] = `{"role":"result","round_id":"round-5","message_id":"result-5","subtype":"error","timestamp":1700000005500}`
				return append(lines, "\n"+`{"nexus_overlay_kind":"round_marker","round_id":"round-6","content":"prompt 6","timestamp":1700000006000}`)
			})
		},
		"reordered in place at the same size": func(harness *overlayReplayHarness) {
			rewrite(harness, func(lines []string) []string {
				last := len(lines) - 1
				lines[last-2], lines[last] = strings.TrimSuffix(lines[last], "\n")+"\n", strings.TrimSuffix(lines[last-2], "\n")
				return lines
			})
		},
		"removed": func(harness *overlayReplayHarness) {
			if err := os.Remove(harness.overlayPath); err != nil {
				harness.t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			harness := newOverlayReplayHarness(t)
			seed(harness)
			mutate(harness)
			harness.assertMatchesFullRead(nil)
			harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-after", "content": "after", "timestamp": 1_700_000_100_000})
			harness.assertMatchesFullRead([]string{"round-after"})
		})
	}
}

// 边缘指纹只覆盖首尾各 4KB；中段被改写时要靠文件身份与同尺寸下的修改时间发现。
func TestOverlayReplayDetectsRewritesOutsideTheEdgeFingerprint(t *testing.T) {
	cases := map[string]func(t *testing.T, path string, rewritten []byte){
		"same size rewritten in place": func(t *testing.T, path string, rewritten []byte) {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, rewritten, 0o644); err != nil {
				t.Fatal(err)
			}
			if err = os.Chtimes(path, info.ModTime().Add(2*time.Second), info.ModTime().Add(2*time.Second)); err != nil {
				t.Fatal(err)
			}
		},
		"replaced by a longer file with the same edges": func(t *testing.T, path string, rewritten []byte) {
			rewritten = append(rewritten, `{"nexus_overlay_kind":"round_marker","round_id":"round-new","content":"new","timestamp":1700009000000}`+"\n"...)
			if err := os.WriteFile(path+".next", rewritten, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path+".next", path); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			harness := newOverlayReplayHarness(t)
			for round := 0; round < 200; round++ {
				harness.appendRows(
					map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": fmt.Sprintf("round-%03d", round), "content": fmt.Sprintf("prompt %03d", round), "timestamp": 1_700_000_000_000 + round*1000},
					map[string]any{"role": "assistant", "round_id": fmt.Sprintf("round-%03d", round), "message_id": fmt.Sprintf("answer-%03d", round), "timestamp": 1_700_000_000_500 + round*1000},
				)
			}
			harness.assertMatchesFullRead(nil)
			payload, err := os.ReadFile(harness.overlayPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(payload) < 4*historyPageEdgeHashBytes {
				t.Fatalf("fixture too small to hide a rewrite from the edge fingerprint: %d bytes", len(payload))
			}
			rewritten := bytes.Replace(payload, []byte("prompt 100"), []byte("edited 100"), 1)
			rewritten = bytes.Replace(rewritten, []byte("answer-100"), []byte("answer-1xx"), 1)
			if bytes.Equal(rewritten, payload) {
				t.Fatal("fixture rewrite did not change the overlay")
			}
			mutate(t, harness.overlayPath, rewritten)
			harness.assertMatchesFullRead(nil)
		})
	}
}

func TestOverlayReplayKeepsNoStateForPartiallyWrittenLine(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-0", "content": "first", "timestamp": 1_700_000_000_000})
	harness.assertMatchesFullRead(nil)

	complete := `{"nexus_overlay_kind":"round_marker","round_id":"round-1","content":"second","timestamp":1700000001000}`
	harness.appendRaw(complete[:40])
	state, err := harness.history.readOverlayHistoryStateContext(context.Background(), harness.workspacePath, harness.session.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.RoundMarkers) != 1 {
		t.Fatalf("half-written marker must not be visible: %#v", state.RoundMarkers)
	}
	if entry := harness.replayEntry(); entry.history.cursor != nil {
		t.Fatalf("a read ending mid-line must not leave a resume point: %#v", entry.history.cursor)
	}

	harness.appendRaw(complete[40:] + "\n")
	harness.assertMatchesFullRead(nil)
	state, err = harness.history.readOverlayHistoryStateContext(context.Background(), harness.workspacePath, harness.session.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.RoundMarkers) != 2 || state.RoundMarkers[1].Content != "second" {
		t.Fatalf("completed marker missing after the line was finished: %#v", state.RoundMarkers)
	}
}

// 追加方把一行写到一半时，导航读取返回已写完的行，不把半行当成读取失败；
// 这一行写完后再读必须出现。
func TestRoundIndexReadSkipsPartiallyWrittenLastRow(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	marker := func(round int) string {
		return fmt.Sprintf(`{"nexus_overlay_kind":"round_marker","round_id":"round-%d","content":"第 %d 轮","timestamp":%d}`,
			round, round, 1_700_000_000_000+round*1000)
	}
	readRounds := func() []string {
		t.Helper()
		index, err := harness.history.ReadRoundIndex(harness.workspacePath, harness.session, nil)
		if err != nil {
			t.Fatalf("round index read: %v", err)
		}
		rounds := make([]string, len(index.Items))
		for i, item := range index.Items {
			rounds[i] = item.RoundID
		}
		return rounds
	}
	harness.appendRaw(marker(0) + "\n")
	want := []string{"round-0"}

	// 断点依次落在行首、多字节字符中间和只差收尾括号处。
	for index, cut := range []func(row string) int{
		func(string) int { return 1 },
		func(row string) int { return strings.Index(row, "轮") + 1 },
		func(row string) int { return len(row) - 1 },
	} {
		if got := readRounds(); !reflect.DeepEqual(got, want) {
			t.Fatalf("complete rows: got %v want %v", got, want)
		}
		round := index + 1
		row := marker(round)
		written := cut(row)
		harness.appendRaw(row[:written])
		if got := readRounds(); !reflect.DeepEqual(got, want) {
			t.Fatalf("%d of %d bytes of the next row written: got %v want %v", written, len(row), got, want)
		}
		if entry := harness.replayEntry(); entry.roundIndex.cursor != nil {
			t.Fatalf("a read ending mid-row must not leave a resume point: %#v", entry.roundIndex.cursor)
		}
		harness.assertMatchesFullRead(nil)

		harness.appendRaw(row[written:] + "\n")
		want = append(want, fmt.Sprintf("round-%d", round))
		harness.assertMatchesFullRead(nil)
	}
	if got := readRounds(); !reflect.DeepEqual(got, want) {
		t.Fatalf("finished rows: got %v want %v", got, want)
	}
}

// 一行中断在等值的位置后又有完整行追加：后一行会被解码成前一行的字段值，文件以换行结尾。
// 这不是写到一半的行，读取必须报错，不能悄悄丢掉后一轮并把续读位置推到它之后。
func TestRoundIndexReadRejectsIncompleteRowBeforeFinishedAppend(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	read := func() (protocol.SessionRoundIndex, error) {
		return harness.history.ReadRoundIndex(harness.workspacePath, harness.session, nil)
	}
	harness.appendRaw(`{"nexus_overlay_kind":"round_marker","round_id":"round-0","content":"第 0 轮","timestamp":1700000000000}` + "\n")
	harness.appendRaw(`{"nexus_overlay_kind":"round_marker","round_id":"round-1","content":`)
	index, err := read()
	if err != nil || len(index.Items) != 1 {
		t.Fatalf("interrupted last row: got %#v, %v", index.Items, err)
	}

	harness.appendRaw(`{"nexus_overlay_kind":"round_marker","round_id":"round-2","content":"第 2 轮","timestamp":1700000002000}` + "\n")
	if index, err = read(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("finished append after an interrupted row: got %#v, %v", index.Items, err)
	}
	if entry := harness.replayEntry(); entry.roundIndex.cursor != nil {
		t.Fatalf("a failed read must not leave a resume point: %#v", entry.roundIndex.cursor)
	}
}

func TestOverlayReplayResultIsIsolatedFromCallers(t *testing.T) {
	harness := newOverlayReplayHarness(t)
	harness.appendRows(map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-0", "content": "first", "timestamp": 1_700_000_000_000})
	for index, agentID := range []string{"amy", "bob", "cat"} {
		harness.appendRows(map[string]any{"role": "assistant", "round_id": "round-0", "agent_id": agentID, "message_id": fmt.Sprintf("a-%d", index), "timestamp": 1_700_000_000_100 + index})
	}
	ctx := context.Background()
	state, err := harness.history.readOverlayHistoryStateContext(ctx, harness.workspacePath, harness.session.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	index, err := harness.history.readCanonicalRoundIndexContext(ctx, harness.workspacePath, harness.session, []string{"round-0"})
	if err != nil {
		t.Fatal(err)
	}

	harness.appendRows(
		map[string]any{"role": "assistant", "round_id": "round-0", "agent_id": "dan", "message_id": "a-3", "timestamp": 1_700_000_000_200},
		map[string]any{overlayKindField: overlayKindRoundMarker, "round_id": "round-1", "content": "second", "timestamp": 1_700_000_001_000},
	)
	harness.assertMatchesFullRead(nil)

	// 旧结果上的 append 与条目改写都不能进入回放状态。
	_ = append(state.MessageRows, protocol.Message{"message_id": "caller-row"})
	_ = append(state.RoundMarkers, transcriptRoundMarker{RoundID: "caller-marker"})
	_ = append(index.Items[0].AgentIDs, "caller-agent")
	index.Items[0].Title = "caller edit"
	harness.assertMatchesFullRead(nil)
}

// 回放状态里的行会被多次读取共用：真实读取链路并发读、同时有新行追加时，
// 任何读取方都不能改写共用的行，race detector 与事后的整份比对共同把关。
// 行里带上读取链路会改写的嵌套形态：超出内联预算的 tool_result、已有的投递记录与投递回执。
func TestOverlayReplaySharedRowsSurviveConcurrentHistoryReads(t *testing.T) {
	fixture := newLongSessionFixture(t, 12, 64)
	harness := newOverlayReplayHarnessFor(t, fixture)
	largeToolOutput := strings.Repeat("x", historyMessageDetailThresholdBytes)
	appendOverlayRows := func(round int) {
		roundID := fmt.Sprintf("round-%05d", round)
		for _, message := range []protocol.Message{
			{"message_id": "overlay-assistant-" + roundID, "role": "assistant", "round_id": roundID, "agent_id": "amy",
				"content": []map[string]any{
					{"type": "text", "text": "overlay answer"},
					{"type": "tool_result", "tool_use_id": "tool-" + roundID, "content": largeToolOutput},
				},
				"external_deliveries": []map[string]any{{"channel": "im", "target": "earlier"}},
				"timestamp":           1_700_000_000_000 + round*10_000 + 4000},
			{"message_id": "overlay-result-" + roundID, "role": "result", "round_id": roundID, "subtype": "success",
				"duration_ms": 1200, "timestamp": 1_700_000_000_000 + round*10_000 + 5000},
		} {
			if err := fixture.history.AppendOverlayMessage(fixture.workspacePath, fixture.session.SessionKey, message); err != nil {
				t.Fatal(err)
			}
		}
		if err := fixture.history.AppendExternalDeliveryReceipt(fixture.workspacePath, fixture.session.SessionKey, ExternalDeliveryReceipt{
			RoundID: roundID, MessageID: "overlay-assistant-" + roundID, Channel: "im", Target: "later",
			PrimaryPlatformMessageID: "platform-" + roundID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for round := 0; round < fixture.rounds; round++ {
		appendOverlayRows(round)
	}
	fixture.readFirstPage(t)

	ctx := context.Background()
	readers := []func() error{
		func() error {
			_, err := fixture.history.ReadMessagesPageContext(ctx, fixture.workspacePath, fixture.session, nil, HistoryPageQuery{Limit: 20})
			return err
		},
		func() error {
			_, err := fixture.history.ReadMessages(fixture.workspacePath, fixture.session, []string{"round-00003"})
			return err
		},
		func() error {
			_, err := fixture.history.ReadRoundIndex(fixture.workspacePath, fixture.session, []string{"round-00003"})
			return err
		},
		func() error {
			_, err := fixture.history.HasCanonicalUserInput(fixture.workspacePath, fixture.session.SessionKey)
			return err
		},
	}
	var group sync.WaitGroup
	failures := make(chan error, len(readers)*8)
	for _, read := range readers {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 8 {
				if err := read(); err != nil {
					failures <- err
					return
				}
			}
		}()
	}
	for range 6 {
		round := fixture.rounds
		fixture.appendRound(t)
		appendOverlayRows(round)
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	waitHistoryPageIndexFutureTB(t, fixture.history.historyPageAccess(fixture.workspacePath, fixture.session).Scope)
	harness.assertMatchesFullRead([]string{"round-00003"})
}

func TestOverlayReplayStoreEvictsLeastRecentlyUsedWithinBudget(t *testing.T) {
	var store overlayReplayStore
	retain := func(key string, size int64) {
		if err := store.replay(key, func(entry *overlayReplayEntry) error {
			entry.history.cursor = &overlayReplayCursor{Size: size}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	retain("old", overlayReplayBudgetBytes/2)
	retain("recent", overlayReplayBudgetBytes/2)
	retain("old", overlayReplayBudgetBytes/2)
	retain("new", overlayReplayBudgetBytes/2)
	if _, ok := store.entries["recent"]; ok {
		t.Fatal("least recently used overlay state should be evicted first")
	}
	if len(store.entries) != 2 || store.entries["old"] == nil || store.entries["new"] == nil {
		t.Fatalf("unexpected retained overlays: %v", store.order)
	}
	for index := 0; index < maxOverlayReplaySessions+8; index++ {
		retain(fmt.Sprintf("small-%d", index), 1)
	}
	if len(store.entries) != maxOverlayReplaySessions || len(store.order) != maxOverlayReplaySessions {
		t.Fatalf("retained %d overlays, want %d", len(store.entries), maxOverlayReplaySessions)
	}
}
