// INPUT: 已发布读模型的长会话、进程重启、半行追加、截断、替换、原地改写、读模型版本变化、触及旧条目的追加，以及检查点之前未按内容对齐的轮次。
// OUTPUT: 重启后只从持久化的 transcript 检查点续读且结果与完整重建逐字段一致；无法证明前缀不变时回退完整重建。
// POS: DM transcript 投影位置持久化的回归；等价性比对沿用 agent_history_incremental_test.go 的参照重建。
package workspace

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// refreshTailAfterRestart 模拟进程重启后读取首页，要求本次刷新走增量路径；不做参照比对，
// 因而已发布的尾轮状态保持为增量刷新写下的那一份，供后续步骤继续续接。
func refreshTailAfterRestart(t *testing.T, f *longSessionFixture) {
	t.Helper()
	simulateHistoryProcessRestart(t, f)
	f.readFirstPage(t)
	if f.history.cache.tailRefreshes.Load() != 1 {
		t.Fatal("refresh after restart did not resume from the persisted checkpoint")
	}
}

// assertTailRefreshAfterRestart 重启后刷新并与完整重建比对。
func assertTailRefreshAfterRestart(t *testing.T, f *longSessionFixture, incremental bool) {
	t.Helper()
	simulateHistoryProcessRestart(t, f)
	assertTailRefresh(t, f, incremental)
}

func readAgentHistoryCheckpointForTest(t *testing.T, f *longSessionFixture) *agentHistoryCheckpoint {
	t.Helper()
	access := f.history.historyPageAccess(f.workspacePath, f.session)
	db, err := access.ReadModel.database(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	scope, ok, err := readHistoryReadModelScope(context.Background(), db, access.Scope)
	if err != nil || !ok {
		t.Fatalf("scope unavailable: %v", err)
	}
	checkpoint, err := readAgentHistoryCheckpoint(context.Background(), db, access.Scope, scope.Generation)
	if err != nil {
		t.Fatal(err)
	}
	return checkpoint
}

func TestAgentHistoryCheckpointResumesAfterRestart(t *testing.T) {
	// 每个 chain 先连续做 chain-1 次“重启、追加、增量刷新”，最后一次再与完整重建比对，
	// 覆盖由增量刷新（而非完整重建）写下的检查点被再次续接的情形。
	for chain := 1; chain <= 4; chain++ {
		t.Run(fmt.Sprintf("appended rounds x%d", chain), func(t *testing.T) {
			f := newLongSessionFixture(t, 6, 64)
			f.readFirstPage(t)
			for range chain - 1 {
				f.appendRound(t)
				refreshTailAfterRestart(t, f)
			}
			f.appendRound(t)
			assertTailRefreshAfterRestart(t, f, true)
		})
	}

	t.Run("checkpoint advances to the last round start", func(t *testing.T) {
		f := newLongSessionFixture(t, 6, 64)
		f.readFirstPage(t)
		built := readAgentHistoryCheckpointForTest(t, f)
		// 前五轮都按内容对齐并定型，检查点只留下起点这一轮还没定型的 marker。
		if built == nil || built.Transcript.UUID != "u-00005" || !reflect.DeepEqual(built.Transcript.Markers, transcriptMarkerFrontier{
			Markers: 6, SealedTimestamp: 1_700_000_040_000, OpenMarkers: []int{5},
		}) {
			t.Fatalf("full rebuild checkpoint = %+v", built)
		}
		f.appendRound(t)
		refreshTailAfterRestart(t, f)
		resumed := readAgentHistoryCheckpointForTest(t, f)
		if resumed == nil || resumed.Transcript.UUID != "u-00006" || !reflect.DeepEqual(resumed.Transcript.Markers, transcriptMarkerFrontier{
			Markers: 7, SealedTimestamp: 1_700_000_050_000, OpenMarkers: []int{6},
		}) || resumed.Transcript.Offset <= built.Transcript.Offset || resumed.Transcript.Line != built.Transcript.Line+4 {
			t.Fatalf("resumed checkpoint = %+v, previous %+v", resumed, built)
		}
		// 偏移必须正好是起点条目所在行的行首。
		data, err := os.ReadFile(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		line, _, _ := bytes.Cut(data[resumed.Transcript.Offset:], []byte("\n"))
		var entry map[string]any
		if err = json.Unmarshal(line, &entry); err != nil || entry["uuid"] != "u-00006" {
			t.Fatalf("checkpoint offset does not start the round entry: %v %q", err, line)
		}
	})

	t.Run("streaming round across restarts", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		rows := f.startRound(t)
		assertTailRefreshAfterRestart(t, f, true)
		for _, row := range rows {
			f.appendTranscript(t, row)
			assertTailRefreshAfterRestart(t, f, true)
		}
	})

	t.Run("partial trailing line", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		rows := f.startRound(t)
		f.appendTranscript(t, rows[:3]...)
		last, err := json.Marshal(rows[3])
		if err != nil {
			t.Fatal(err)
		}
		// 最后一行只写了一半：本次只投影完整行，半行不计入。
		appendTranscriptBytes(t, f, last[:len(last)/2])
		assertTailRefreshAfterRestart(t, f, true)
		// 半行补全后从同一个检查点续读，这一行不能丢。
		appendTranscriptBytes(t, f, append(last[len(last)/2:], '\n'))
		assertTailRefreshAfterRestart(t, f, true)
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
	})

	t.Run("truncated transcript", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		info, err := os.Stat(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		f.appendRound(t)
		f.readFirstPage(t)
		if err = os.Truncate(f.transcript, info.Size()); err != nil {
			t.Fatal(err)
		}
		assertTailRefreshAfterRestart(t, f, false)
	})

	t.Run("replaced transcript", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		data, err := os.ReadFile(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		replacement := f.transcript + ".new"
		if err = os.WriteFile(replacement, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err = os.Rename(replacement, f.transcript); err != nil {
			t.Fatal(err)
		}
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, false)
		// 回退的完整重建重新建立检查点，下一轮恢复增量。
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
	})

	t.Run("prefix rewritten in place", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		data, err := os.ReadFile(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		// 同一文件、同样长度，只改了第一轮回复的正文：只有首尾指纹能发现。
		rewritten := bytes.Replace(data, []byte("answer answer"), []byte("ANSWER ANSWER"), 1)
		file, err := os.OpenFile(f.transcript, os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.WriteAt(rewritten, 0); err != nil {
			t.Fatal(err)
		}
		if err = file.Close(); err != nil {
			t.Fatal(err)
		}
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, false)
	})

	t.Run("read model version changed", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		f.appendRound(t)
		simulateHistoryProcessRestart(t, f)
		// 落盘的读模型来自另一个投影规则版本：整库作废，检查点不能续接。
		db, err := sql.Open("sqlite", historyReadModelPath(filepath.Dir(f.workspacePath)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, historyReadModelSchemaVersion-1)); err != nil {
			t.Fatal(err)
		}
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
		assertTailRefresh(t, f, false)
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
	})

	t.Run("append reuses an earlier entry uuid", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		// 新追加的条目与已冻结前缀里的条目同 uuid：主链前缀的条目选择会变，只能完整重建。
		rows := f.startRound(t)
		rows[1]["uuid"] = "a2-00001"
		rows[2]["parentUuid"] = "a2-00001"
		f.appendTranscript(t, rows...)
		assertTailRefreshAfterRestart(t, f, false)
	})

	t.Run("append answers an earlier tool call", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		// 新一轮里出现旧轮工具调用的结果：并行条目归属可能改变，只能完整重建。
		rows := f.startRound(t)
		rows[2]["message"].(map[string]any)["content"].([]map[string]any)[0]["tool_use_id"] = "t-00001"
		f.appendTranscript(t, rows...)
		assertTailRefreshAfterRestart(t, f, false)
	})

	t.Run("transcript above the source budget", func(t *testing.T) {
		const toolBytes = 200 << 10
		f := newLongSessionFixture(t, historyPageSourceMaxBytes/toolBytes+1, toolBytes)
		if info, err := os.Stat(f.transcript); err != nil || info.Size() <= historyPageSourceMaxBytes {
			t.Fatalf("transcript size = %v, want above %d: %v", info, historyPageSourceMaxBytes, err)
		}
		// 超长会话照常建立读模型与检查点，之后的刷新只读新增字节，与解析缓存能否容纳整个文件无关。
		f.readFirstPage(t)
		if readAgentHistoryCheckpointForTest(t, f) == nil {
			t.Fatal("long transcript did not publish a checkpoint")
		}
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
	})

	t.Run("marker before the checkpoint rewritten", func(t *testing.T) {
		f := newLongSessionFixture(t, 5, 64)
		f.readFirstPage(t)
		// overlay 仍是同一文件的纯追加，但新 marker 与第二轮的输入内容相同且时间更近，
		// 第二轮重新对齐到它：检查点之前的投影已变，只能完整重建。
		if err := f.history.AppendRoundMarker(f.workspacePath, f.session.SessionKey, "round-rebound", "question 1", 1_700_000_010_000); err != nil {
			t.Fatal(err)
		}
		assertTailRefreshAfterRestart(t, f, false)
	})

	t.Run("goal continuation round before the checkpoint", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		// 隐藏的 Goal 续跑 marker 没有内容，对应的用户条目只有 goal_context：这一轮只能靠回退对齐，
		// 并入前缀后留在检查点的未定型部分，之后每次续读都与新增轮次一起重新对齐。
		timestamp := int64(1_700_000_000_000 + f.rounds*10_000)
		if err := f.history.AppendRoundMarkerWithOptions(f.workspacePath, f.session.SessionKey, "round-goal", "", timestamp,
			RoundMarkerOptions{HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation"}); err != nil {
			t.Fatal(err)
		}
		appendRoundWithoutMarker(t, f, "<goal_context>\nContinue working toward the active thread goal.\n</goal_context>")
		assertTailRefreshAfterRestart(t, f, true)
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
		checkpoint := readAgentHistoryCheckpointForTest(t, f)
		if checkpoint == nil || len(checkpoint.Transcript.Markers.OpenTurns) != 1 || checkpoint.Transcript.Markers.OpenTurns[0].Marker != 4 ||
			!reflect.DeepEqual(checkpoint.Transcript.Markers.OpenMarkers, []int{4, 5}) {
			t.Fatalf("checkpoint after a goal continuation round = %+v", checkpoint)
		}
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
	})

	t.Run("round without a marker before the checkpoint", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		timestamp := appendRoundWithoutMarker(t, f, "typed in the terminal")
		assertTailRefreshAfterRestart(t, f, true)
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
		// 没有 marker 的轮次以“未对齐”留在检查点里；后续轮次的 marker 时间够不着它，续读照常成立。
		checkpoint := readAgentHistoryCheckpointForTest(t, f)
		if checkpoint == nil || len(checkpoint.Transcript.Markers.OpenTurns) != 1 || checkpoint.Transcript.Markers.OpenTurns[0].Marker != -1 {
			t.Fatalf("checkpoint after a round without a marker = %+v", checkpoint)
		}
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
		// 迟到的 marker 落在那一轮的回退范围内，全局对齐会把它绑给那一轮：检查点之前的投影已变，只能完整重建。
		if err := f.history.AppendRoundMarker(f.workspacePath, f.session.SessionKey, "round-late", "typed elsewhere", timestamp+1_000); err != nil {
			t.Fatal(err)
		}
		assertTailRefreshAfterRestart(t, f, false)
	})

	t.Run("second input in the same round", func(t *testing.T) {
		f := newLongSessionFixture(t, 4, 64)
		f.readFirstPage(t)
		// 同一轮内的后续输入也会重建 processor，但它之前的行仍属于尾部分组：
		// 检查点必须留在该轮的第一个起点，下一次刷新才能重放整组。
		rows := f.startRound(t)
		roundID := fmt.Sprintf("round-%05d", f.rounds-1)
		if err := f.history.AppendRoundMarker(f.workspacePath, f.session.SessionKey, roundID, "follow up", int64(1_700_000_000_000+(f.rounds-1)*10_000+4_000)); err != nil {
			t.Fatal(err)
		}
		followUp := map[string]any{"type": "user", "uuid": "u-follow-up", "parentUuid": f.lastUUID, "sessionId": *f.session.SessionID,
			"timestamp": "2023-11-14T22:20:00Z", "message": map[string]any{"role": "user", "content": "follow up"}}
		followUp["timestamp"] = rows[3]["timestamp"]
		f.appendTranscript(t, append(rows, followUp)...)
		f.lastUUID = "u-follow-up"
		assertTailRefreshAfterRestart(t, f, true)
		if checkpoint := readAgentHistoryCheckpointForTest(t, f); checkpoint == nil {
			t.Fatal("no checkpoint after a second input in the same round")
		}
		f.appendRound(t)
		assertTailRefreshAfterRestart(t, f, true)
	})
}

// appendRoundWithoutMarker 追加一轮只写进 transcript 的输入与回复，返回该轮的时间；marker 由调用方决定是否另写。
func appendRoundWithoutMarker(t *testing.T, f *longSessionFixture, content string) int64 {
	t.Helper()
	round := f.rounds
	f.rounds++
	timestamp := int64(1_700_000_000_000 + round*10_000)
	at := time.UnixMilli(timestamp).UTC().Format(time.RFC3339Nano)
	user, reply := fmt.Sprintf("u-%05d", round), fmt.Sprintf("a2-%05d", round)
	f.appendTranscript(t,
		map[string]any{"type": "user", "uuid": user, "parentUuid": nilIfEmpty(f.lastUUID), "sessionId": *f.session.SessionID, "timestamp": at,
			"message": map[string]any{"role": "user", "content": content}},
		map[string]any{"type": "assistant", "uuid": reply, "parentUuid": user, "sessionId": *f.session.SessionID, "timestamp": at,
			"message": map[string]any{"id": fmt.Sprintf("m2-%05d", round), "role": "assistant", "stop_reason": "end_turn",
				"content": []map[string]any{{"type": "text", "text": "done"}}}},
	)
	f.lastUUID = reply
	return timestamp
}

func appendTranscriptBytes(t *testing.T, f *longSessionFixture, data []byte) {
	t.Helper()
	file, err := os.OpenFile(f.transcript, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
}
