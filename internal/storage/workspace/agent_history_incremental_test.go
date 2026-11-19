// INPUT: 长会话逐轮/分段追加、迟到的旧轮 result、投递回执、overlay 被替换与仅 overlay 的轮次。
// OUTPUT: DM 尾轮增量刷新与全新进程完整重建的读模型逐字段一致，且该走快路径时确实走了快路径。
// POS: DM 历史增量刷新的等价性回归。
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// assertTailRefresh 读取首页并与全新 store 的完整重建比对；incremental 指明本次是否必须走增量路径。
func assertTailRefresh(t *testing.T, f *longSessionFixture, incremental bool) {
	t.Helper()
	before := f.history.cache.tails.refreshed.Load()
	f.readFirstPage(t)
	if got := f.history.cache.tails.refreshed.Load() > before; got != incremental {
		t.Fatalf("incremental refresh = %v, want %v", got, incremental)
	}
	got := dumpHistoryReadModelWithIndex(t, f)

	// 全新 store 没有解析缓存、投影检查点与尾轮状态，得到的是完全独立的参照结果。
	reference := &longSessionFixture{
		history:       NewAgentHistoryStore(filepath.Dir(f.workspacePath)),
		workspacePath: f.workspacePath,
		session:       f.session,
	}
	access := reference.history.historyPageAccess(f.workspacePath, f.session)
	if err := access.ReadModel.deleteScope(context.Background(), access.Scope); err != nil {
		t.Fatal(err)
	}
	reference.readFirstPage(t)
	if want := dumpHistoryReadModelWithIndex(t, reference); !reflect.DeepEqual(got, want) {
		for i := range max(len(got), len(want)) {
			var g, w string
			if i < len(got) {
				g = got[i]
			}
			if i < len(want) {
				w = want[i]
			}
			if g != w {
				at := 0
				for at < min(len(g), len(w)) && g[at] == w[at] {
					at++
				}
				from := max(at-200, 0)
				t.Logf("row %d differs at %d\ngot  %.400s\nwant %.400s", i, at, g[from:], w[min(from, len(w)):])
				break
			}
		}
		t.Fatalf("incremental read model diverged from full rebuild (%d vs %d rows)", len(got), len(want))
	}
}

// dumpHistoryReadModelWithIndex 导出分组正文、detail、round index 与 sources。
// 增量刷新沿用已发布的 generation，完整重建按当前 sources 派生新 generation；
// detail 引用内嵌 generation，比对前统一替换掉，其余字节必须一致。
func dumpHistoryReadModelWithIndex(t *testing.T, f *longSessionFixture) []string {
	t.Helper()
	dumpHistoryReadModelScope(t, f)
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	ctx := context.Background()
	db, err := access.ReadModel.database(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var generation, index, sources string
	if err = db.QueryRowContext(ctx, `SELECT generation, round_index_json, sources_json FROM history_read_scopes WHERE scope=?`, access.Scope).Scan(&generation, &index, &sources); err != nil {
		t.Fatal(err)
	}
	// detail_ref 由 generation 派生，按出现顺序换成占位符；detail 表按内容摘要比对。
	refs := map[string]string{}
	refPattern := regexp.MustCompile(`"detail_ref":"([0-9a-f]+)"`)
	normalize := func(value string) string {
		value = refPattern.ReplaceAllStringFunc(value, func(match string) string {
			ref := refPattern.FindStringSubmatch(match)[1]
			if _, ok := refs[ref]; !ok {
				refs[ref] = fmt.Sprintf("ref-%d", len(refs))
			}
			return `"detail_ref":"` + refs[ref] + `"`
		})
		return strings.ReplaceAll(value, generation, "<generation>")
	}
	dump := []string{}
	collect := func(query string) {
		rows, err := db.QueryContext(ctx, query, access.Scope, generation)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		columns, _ := rows.Columns()
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err = rows.Scan(pointers...); err != nil {
				t.Fatal(err)
			}
			dump = append(dump, normalize(fmt.Sprintf("%s", values)))
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
	}
	collect(`SELECT sequence, cursor_round_id, cursor_timestamp, group_key, first_non_synthetic_timestamp, synthetic_interrupts_json, payload
		FROM history_read_groups WHERE scope=? AND generation=? ORDER BY sequence`)
	collect(`SELECT sequence, kind, media_type, byte_size, payload_digest
		FROM history_read_details WHERE scope=? AND generation=? ORDER BY sequence, payload_digest`)
	return append(dump, index, sources)
}

func TestAgentHistoryTailRefreshMatchesFullRebuild(t *testing.T) {
	t.Run("full cache", testAgentHistoryTailRefresh)
	// 骨架缓存：检查点之前的条目只剩建链与对齐字段，增量与完整重建仍须逐字段一致。
	t.Run("skeleton cache", func(t *testing.T) {
		forceTranscriptSkeleton(t)
		testAgentHistoryTailRefresh(t)
	})
}

func forceTranscriptSkeleton(t *testing.T) {
	t.Helper()
	previous := transcriptSlimSourceBytes
	transcriptSlimSourceBytes = 0
	t.Cleanup(func() { transcriptSlimSourceBytes = previous })
}

func testAgentHistoryTailRefresh(t *testing.T) {
	t.Run("appended rounds", func(t *testing.T) {
		f := newLongSessionFixture(t, 6, 64)
		f.readFirstPage(t)
		for range 4 {
			f.appendRound(t)
			assertTailRefresh(t, f, true)
		}
	})

	t.Run("streaming round", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		rows := f.startRound(t)
		assertTailRefresh(t, f, true)
		for _, row := range rows {
			f.appendTranscript(t, row)
			assertTailRefresh(t, f, true)
		}
		f.appendRound(t)
		assertTailRefresh(t, f, true)
	})

	t.Run("late result for an earlier round", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		if err := f.history.AppendOverlayMessage(f.workspacePath, f.session.SessionKey, protocol.Message{
			"message_id": "result_round-00001", "session_key": f.session.SessionKey, "agent_id": f.session.AgentID,
			"round_id": "round-00001", "role": "result", "subtype": "error", "timestamp": int64(1_700_000_015_000),
		}); err != nil {
			t.Fatal(err)
		}
		// 触及旧轮必须回退全量规范化；回退后重新建立尾轮状态，下一轮恢复增量。
		assertTailRefresh(t, f, false)
		f.appendRound(t)
		assertTailRefresh(t, f, true)
	})

	t.Run("late result with a fresh timestamp", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		// 时间上排在最后，只有身份集合能发现它属于旧轮。
		if err := f.history.AppendOverlayMessage(f.workspacePath, f.session.SessionKey, protocol.Message{
			"message_id": "result_round-00001", "session_key": f.session.SessionKey, "agent_id": f.session.AgentID,
			"round_id": "round-00001", "role": "result", "subtype": "error", "timestamp": int64(1_800_000_000_000),
		}); err != nil {
			t.Fatal(err)
		}
		assertTailRefresh(t, f, false)
	})

	t.Run("new round tied with the previous reply", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		// 上一组是仅 overlay 的轮次；新一轮 transcript 用户行与它同一毫秒，合并顺序会把新行排到前面。
		roundID, timestamp := "automation-delivery-1", int64(1_700_000_100_000)
		if err := f.history.AppendRoundMarker(f.workspacePath, f.session.SessionKey, roundID, "scheduled report", timestamp); err != nil {
			t.Fatal(err)
		}
		if err := f.history.AppendOverlayMessage(f.workspacePath, f.session.SessionKey, protocol.Message{
			"message_id": "assistant-" + roundID, "session_key": f.session.SessionKey, "agent_id": f.session.AgentID,
			"round_id": roundID, "role": "assistant", "stop_reason": "end_turn", "is_complete": true,
			"content": []map[string]any{{"type": "text", "text": "report"}}, "timestamp": timestamp + 1,
		}); err != nil {
			t.Fatal(err)
		}
		assertTailRefresh(t, f, true)
		for f.rounds*10_000 < 100_000 {
			f.rounds++
		}
		rows := f.startRound(t)
		rows[0]["timestamp"] = time.UnixMilli(timestamp + 1).UTC().Format(time.RFC3339Nano)
		f.appendTranscript(t, rows...)
		assertTailRefreshResultOnly(t, f)
	})

	t.Run("delivery receipt for the latest reply", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		f.appendRound(t)
		assertTailRefresh(t, f, true)
		if err := f.history.AppendExternalDeliveryReceipt(f.workspacePath, f.session.SessionKey, ExternalDeliveryReceipt{
			RoundID: "round-00004", MessageID: "m2-00004", Channel: "im", Target: "chat-1",
			PrimaryPlatformMessageID: "platform-1", Timestamp: time.UnixMilli(1_700_000_045_000),
		}); err != nil {
			t.Fatal(err)
		}
		assertTailRefresh(t, f, true)
	})

	t.Run("overlay only round", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		roundID := "automation-delivery-1"
		timestamp := int64(1_700_000_100_000)
		if err := f.history.AppendRoundMarker(f.workspacePath, f.session.SessionKey, roundID, "scheduled report", timestamp); err != nil {
			t.Fatal(err)
		}
		if err := f.history.AppendOverlayMessage(f.workspacePath, f.session.SessionKey, protocol.Message{
			"message_id": "assistant-" + roundID, "session_key": f.session.SessionKey, "agent_id": f.session.AgentID,
			"round_id": roundID, "role": "assistant", "stop_reason": "end_turn", "is_complete": true,
			"content": []map[string]any{{"type": "text", "text": "report"}}, "timestamp": timestamp + 1,
		}); err != nil {
			t.Fatal(err)
		}
		// 最后一组是仅 overlay 的轮次，transcript 尾部仍属于上一组：尾部覆盖两组。
		assertTailRefresh(t, f, true)
		f.appendRound(t)
		assertTailRefresh(t, f, true)
	})

	t.Run("cancelled refresh keeps the tail", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		f.appendRound(t)
		access := f.history.historyPageAccess(f.workspacePath, f.session)
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if _, ok := f.history.refreshAgentHistoryTail(cancelled, f.workspacePath, f.session, access.Scope); ok {
			t.Fatal("cancelled refresh must not succeed")
		}
		// 取消不说明前缀变化，状态放回后下一次刷新仍走增量。
		assertTailRefresh(t, f, true)
	})

	t.Run("replaced overlay", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		overlay := f.history.paths.SessionOverlayPath(f.workspacePath, f.session.SessionKey)
		data, err := os.ReadFile(overlay)
		if err != nil {
			t.Fatal(err)
		}
		replacement := overlay + ".new"
		if err = os.WriteFile(replacement, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err = os.Rename(replacement, overlay); err != nil {
			t.Fatal(err)
		}
		f.appendRound(t)
		assertTailRefresh(t, f, false)
	})
}

func TestLargeDMHistoryKeepsReadModelAndRefreshesIncrementally(t *testing.T) {
	// 每轮 1MB 的工具结果，transcript 超过旧版 64MB 读模型上限。
	f := newLongSessionFixture(t, historyPageSourceMaxBytes>>20+2, 1<<20)
	if info, err := os.Stat(f.transcript); err != nil || info.Size() <= historyPageSourceMaxBytes {
		t.Fatalf("fixture transcript too small: %v", err)
	}
	f.readFirstPage(t)
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	scope, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope)
	if err != nil || !ok || scope.DisabledReason != "" || scope.GroupCount == 0 {
		t.Fatalf("large DM read model unavailable: ok=%v err=%v disabled=%q groups=%d", ok, err, scope.DisabledReason, scope.GroupCount)
	}
	f.appendRound(t)
	assertTailRefresh(t, f, true)

	// 增量刷新沿用已发布 generation：新旧分组里的 detail 引用都必须仍能读出完整内容。
	page := f.readFirstPage(t)
	refs := regexp.MustCompile(`"detail_ref":"([0-9a-f]+)"`).FindAllStringSubmatch(fmt.Sprint(mustJSON(t, page.Items)), -1)
	if len(refs) < 2 {
		t.Fatalf("expected detail refs on the first page, got %d", len(refs))
	}
	for _, ref := range refs {
		detail, err := f.history.ReadMessageDetailContext(context.Background(), f.workspacePath, f.session, ref[1])
		if err != nil || detail.ByteSize < 1<<20 {
			t.Fatalf("detail %s unreadable after incremental refresh: size=%d err=%v", ref[1], detail.ByteSize, err)
		}
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestConcurrentTranscriptReadsParseOnce(t *testing.T) {
	f := newLongSessionFixture(t, 40, 4096)
	// 全新 store 没有解析缓存，模拟重启后预热与前台读取同时落到完整解析。
	history := NewAgentHistoryStore(filepath.Dir(f.workspacePath))
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if _, err := history.readHistoryRowsContext(context.Background(), f.workspacePath, f.session); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if parses := history.cache.fullParses.Load(); parses != 1 {
		t.Fatalf("full parses = %d, want 1", parses)
	}
}

func TestTranscriptSkeletonKeepsChainAndAlignment(t *testing.T) {
	forceTranscriptSkeleton(t)
	f := newLongSessionFixture(t, 12, 2048)
	f.readFirstPage(t)
	f.appendRound(t)
	assertTailRefresh(t, f, true)

	var entry transcriptCacheEntry
	f.history.cache.mu.RLock()
	for _, candidate := range f.history.cache.entries {
		entry = candidate
	}
	f.history.cache.mu.RUnlock()
	if entry.SlimEnd == 0 || entry.Projection == nil || entry.Projection.Rows != nil {
		t.Fatalf("skeleton cache not established: slim_end=%d", entry.SlimEnd)
	}
	if weight := entry.weight(); weight >= entry.Source.Size/2 {
		t.Fatalf("skeleton cache weight %d not below half of source %d", weight, entry.Source.Size)
	}

	// 骨架前缀与完整条目建出的主链、并行工具关联和 marker 对齐必须一致。
	transcript, err := os.Open(f.transcript)
	if err != nil {
		t.Fatal(err)
	}
	defer transcript.Close()
	fullEntries, _, err := readTranscriptEntriesParallelContext(context.Background(), transcript, entry.Source.Size)
	if err != nil {
		t.Fatal(err)
	}
	slimChain := buildPrimaryTranscriptChain(entry.Entries)
	fullChain := buildPrimaryTranscriptChain(fullEntries)
	indexes := func(chain []transcriptEntry) []int {
		out := make([]int, len(chain))
		for i, item := range chain {
			out[i] = item.Index
		}
		return out
	}
	if !reflect.DeepEqual(indexes(slimChain), indexes(fullChain)) {
		t.Fatal("skeleton changed the primary chain")
	}
	overlay, err := f.history.readOverlayHistoryStateContext(context.Background(), f.workspacePath, f.session.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	markers := overlay.RoundMarkers
	if !reflect.DeepEqual(alignTranscriptRoundMarkers(slimChain, markers), alignTranscriptRoundMarkers(fullChain, markers)) {
		t.Fatal("skeleton changed marker alignment")
	}
}

// assertTailRefreshResultOnly 只比对结果，不约束走哪条路径。
func assertTailRefreshResultOnly(t *testing.T, f *longSessionFixture) {
	t.Helper()
	f.readFirstPage(t)
	got := dumpHistoryReadModelWithIndex(t, f)
	reference := &longSessionFixture{history: NewAgentHistoryStore(filepath.Dir(f.workspacePath)), workspacePath: f.workspacePath, session: f.session}
	access := reference.history.historyPageAccess(f.workspacePath, f.session)
	if err := access.ReadModel.deleteScope(context.Background(), access.Scope); err != nil {
		t.Fatal(err)
	}
	reference.readFirstPage(t)
	if want := dumpHistoryReadModelWithIndex(t, reference); !reflect.DeepEqual(got, want) {
		t.Fatalf("read model diverged from full rebuild\ngot  %v\nwant %v", got, want)
	}
}
