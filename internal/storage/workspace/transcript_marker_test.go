// transcript_marker_test.go 覆盖 Transcript 标记、压缩边界、空 Goal 与投影检查点测试。
package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func writeAgentTranscriptFixture(t *testing.T, workspacePath string, sessionID string, rows []map[string]any) {
	t.Helper()

	projectDir := filepath.Join(
		transcriptProjectsDirForWorkspace(workspacePath),
		sanitizeTranscriptPath(canonicalizeTranscriptPath(workspacePath)),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 项目目录失败: %v", err)
	}
	transcriptPath := filepath.Join(projectDir, sessionID+".jsonl")

	file, err := os.Create(transcriptPath)
	if err != nil {
		t.Fatalf("创建 transcript fixture 失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 transcript fixture 失败: %v", err)
		}
	}
}

func TestEmptyTranscriptGoalMarkerMatchingBoundaries(t *testing.T) {
	goalMarker := transcriptRoundMarker{RoundID: "goal", Timestamp: 86401000, HiddenFromUser: true, Synthetic: true, Purpose: "goal_continuation"}
	for _, tc := range []struct {
		name       string
		timestamps []string
		markers    []transcriptRoundMarker
		wantIndex  int
	}{
		{"empty_goal", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, 0},
		{"ordinary_empty_without_marker", []string{"1970-01-02T00:00:01.005Z"}, nil, -1},
		{"visible_marker", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{{RoundID: "visible", Timestamp: 86401000}}, -1},
		{"other_hidden_input", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{{RoundID: "echo", Timestamp: 86401000, HiddenFromUser: true, Synthetic: true, Purpose: "echo_followup"}}, -1},
		{"missing_timestamp", []string{""}, []transcriptRoundMarker{goalMarker}, -1},
		{"stale_marker", []string{"1970-01-03T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
		{"future_marker", []string{"1970-01-01T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
		{"ambiguous_markers", []string{"1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker, goalMarker}, -1},
		{"incidental_empty_before_goal", []string{"1970-01-02T00:00:00Z", "1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, 1},
		{"ambiguous_empty_inputs", []string{"1970-01-02T00:00:01.005Z", "1970-01-02T00:00:01.005Z"}, []transcriptRoundMarker{goalMarker}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := make([]transcriptEntry, len(tc.timestamps))
			for i, timestamp := range tc.timestamps {
				chain[i] = transcriptEntry{Index: i, Data: map[string]any{"type": "user", "timestamp": timestamp, "message": map[string]any{"role": "user", "content": ""}}}
			}
			aligned := alignTranscriptRoundMarkers(chain, tc.markers)
			for i := range chain {
				got := i < len(aligned) && transcriptRoundMarkerPresent(aligned[i])
				if got != (i == tc.wantIndex) {
					t.Errorf("empty input %d matched = %v, want %v", i, got, i == tc.wantIndex)
				}
			}
		})
	}
}

func TestAgentHistoryStoreProjectsHookAdditionalContextGuidance(t *testing.T) {
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
	sessionID := "4035f197-ca97-43fc-b9ae-06ac04903213"
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-1", "写一个五子棋游戏", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}

	writeAgentTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-27T11:40:00.000Z",
			"message": map[string]any{
				"role":    "user",
				"content": "写一个五子棋游戏",
			},
		},
		{
			"type":       "attachment",
			"uuid":       "transcript-guidance-1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"timestamp":  "2026-04-27T11:41:00.000Z",
			"attachment": map[string]any{
				"type": "hook_additional_context",
				"content": []string{
					"<nexus_guidance>\n用户在你执行当前 round 时补充了以下引导。请在继续下一步前结合这些要求；如果与原任务冲突，以最新引导为准。\n1. round_id=queue_guide_1: 需要可以与 bot 对战\n</nexus_guidance>",
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

	var guidance *protocol.Message
	for index := range rows {
		if rows[index]["message_id"] == "queue_guide_1" {
			guidance = &rows[index]
			break
		}
	}
	if guidance == nil {
		t.Fatalf("runtime hook additionalContext 应投影成引导系统消息: %+v", rows)
	}
	if (*guidance)["role"] != "system" || (*guidance)["round_id"] != "round-1" {
		t.Fatalf("引导系统消息应归入当前 round: %+v", *guidance)
	}
	if stringFromAny((*guidance)["content"]) != "需要可以与 bot 对战" {
		t.Fatalf("引导内容解析错误: %+v", *guidance)
	}
	metadata, _ := (*guidance)["metadata"].(map[string]any)
	if metadata["subtype"] != message.SystemMessageSubtypeGuidedInput ||
		metadata["source_round_id"] != "queue_guide_1" {
		t.Fatalf("引导 metadata 不正确: %+v", *guidance)
	}
}

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
