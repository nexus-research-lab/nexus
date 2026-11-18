// INPUT: 追加、半轮追加、分支、改写、marker 变化与投影身份变化后的 transcript。
// OUTPUT: 检查点投影与完整投影逐行一致，且调用方修改结果不会污染检查点。
// POS: 投影前缀检查点的差分回归。
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTranscriptProjectionCheckpointMatchesFullProjection(t *testing.T) {
	// 无 marker 时 marker 前缀恒等，分叉只能由主链前缀比对发现。
	for _, withMarkers := range []bool{true, false} {
		t.Run(fmt.Sprintf("markers=%v", withMarkers), func(t *testing.T) {
			testTranscriptProjectionCheckpoint(t, withMarkers)
		})
	}
}

func testTranscriptProjectionCheckpoint(t *testing.T, withMarkers bool) {
	f := newLongSessionFixture(t, 0, 64)
	sessionID := *f.session.SessionID
	markers := []transcriptRoundMarker{}
	appendRows := func(rows ...map[string]any) {
		t.Helper()
		file, err := os.OpenFile(f.transcript, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		encoder := json.NewEncoder(file)
		for _, row := range rows {
			if err := encoder.Encode(row); err != nil {
				t.Fatal(err)
			}
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	userRow := func(round int, parent string) map[string]any {
		timestamp := int64(1_700_000_000_000 + round*10_000)
		if withMarkers {
			markers = append(markers, transcriptRoundMarker{RoundID: fmt.Sprintf("round-%d", round), Content: fmt.Sprintf("q%d", round), Timestamp: timestamp})
		}
		return map[string]any{"type": "user", "uuid": fmt.Sprintf("u-%d", round), "parentUuid": nilIfEmpty(parent), "sessionId": sessionID,
			"timestamp": time.UnixMilli(timestamp).UTC().Format(time.RFC3339Nano), "message": map[string]any{"role": "user", "content": fmt.Sprintf("q%d", round)}}
	}
	toolUse := func(round int) map[string]any {
		return map[string]any{"type": "assistant", "uuid": fmt.Sprintf("a1-%d", round), "parentUuid": fmt.Sprintf("u-%d", round), "sessionId": sessionID,
			"message": map[string]any{"id": fmt.Sprintf("m1-%d", round), "role": "assistant", "stop_reason": "tool_use",
				"content": []map[string]any{{"type": "tool_use", "id": fmt.Sprintf("t-%d", round), "name": "Bash", "input": map[string]any{"command": "ls"}}}}}
	}
	toolResultAndAnswer := func(round int) []map[string]any {
		return []map[string]any{
			{"type": "user", "uuid": fmt.Sprintf("r-%d", round), "parentUuid": fmt.Sprintf("a1-%d", round), "sessionId": sessionID,
				"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "tool_use_id": fmt.Sprintf("t-%d", round), "content": "ok"}}}},
			{"type": "assistant", "uuid": fmt.Sprintf("a2-%d", round), "parentUuid": fmt.Sprintf("r-%d", round), "sessionId": sessionID,
				"message": map[string]any{"id": fmt.Sprintf("m2-%d", round), "role": "assistant", "stop_reason": "end_turn",
					"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("answer %d", round)}}}},
		}
	}
	appendRound := func(round int, parent string) {
		appendRows(userRow(round, parent), toolUse(round))
		appendRows(toolResultAndAnswer(round)...)
	}

	sessionKey, agentID := f.session.SessionKey, "amy"
	check := func(label string) {
		t.Helper()
		got, err := f.history.readTranscriptMessages(f.workspacePath, sessionKey, agentID, sessionID, markers, "")
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(f.transcript)
		if err != nil {
			t.Fatal(err)
		}
		entries, err := readTranscriptEntriesFileContext(context.Background(), file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		want := projectTranscriptChain(f.workspacePath, sessionKey, agentID, buildPrimaryTranscriptChain(entries), markers)
		if strings.HasPrefix(label, "marker") && !strings.Contains(fmt.Sprint(want), "rewritten") {
			t.Fatalf("%s: markers did not align, fixture is not exercising marker checks", label)
		}
		if len(want) == 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: checkpointed projection diverged\ngot  %v\nwant %v", label, got, want)
		}
		// 调用方随意修改结果，不能影响下一次从检查点恢复的输出。
		for _, row := range got {
			row["round_id"] = "mutated"
			if content, ok := row["content"].([]map[string]any); ok && len(content) > 0 {
				content[0]["type"] = "mutated"
			}
		}
	}
	checkpointRows := func() int {
		path, err := f.history.resolveTranscriptPath(f.workspacePath, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		f.history.cache.mu.Lock()
		defer f.history.cache.mu.Unlock()
		if projection := f.history.cache.entries[path].Projection; projection != nil {
			return len(projection.Rows)
		}
		return 0
	}

	appendRound(0, "")
	check("first round")
	for round := 1; round < 6; round++ {
		appendRound(round, fmt.Sprintf("a2-%d", round-1))
		check(fmt.Sprintf("append round %d", round))
	}
	if checkpointRows() == 0 {
		t.Fatal("append path did not keep a projection checkpoint")
	}
	check("unchanged")

	// 半轮：用户输入和工具调用已写入，工具结果稍后到达。
	appendRows(userRow(6, "a2-5"), toolUse(6))
	check("half round")
	appendRows(toolResultAndAnswer(6)...)
	check("half round completed")

	// 从较早的回答分叉，主链前缀改变，检查点必须失效。
	// 分叉后连续追加多轮，新主链比检查点更长，只能靠逐个比对前缀条目发现。
	appendRound(7, "a2-3")
	appendRound(70, "a2-7")
	appendRound(71, "a2-70")
	check("branch")
	appendRound(8, "a2-71")
	check("append after branch")

	// marker 被改写、投影身份改变都只能用完整投影。
	if withMarkers {
		markers[1].RoundID = "rewritten"
		check("marker rewrite")
	}
	agentID = "other-agent"
	check("identity change")
	agentID = "amy"

	// 原地改写（非纯追加）使解析代系失效。
	content, err := os.ReadFile(f.transcript)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.transcript, []byte(strings.Replace(string(content), "answer 2", "answer two", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	check("rewrite")
	appendRound(9, "a2-8")
	check("append after rewrite")
}
