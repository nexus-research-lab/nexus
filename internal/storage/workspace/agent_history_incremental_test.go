// INPUT: 长会话逐轮/分段追加、迟到的旧轮 result、投递回执、overlay 被替换与仅 overlay 的轮次。
// OUTPUT: DM 尾轮增量刷新与全新进程完整重建的读模型逐字段一致，且该走快路径时确实走了快路径。
// POS: DM 历史增量刷新的等价性回归。
package workspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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
		t.Fatalf("incremental read model diverged from full rebuild\ngot  %v\nwant %v", got, want)
	}
}

func dumpHistoryReadModelWithIndex(t *testing.T, f *longSessionFixture) []string {
	t.Helper()
	dump := dumpHistoryReadModelScope(t, f)
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var index, sources string
	if err = db.QueryRowContext(context.Background(), `SELECT round_index_json, sources_json FROM history_read_scopes WHERE scope=?`, access.Scope).Scan(&index, &sources); err != nil {
		t.Fatal(err)
	}
	return append(dump.Rows, index, sources)
}

func TestAgentHistoryTailRefreshMatchesFullRebuild(t *testing.T) {
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
