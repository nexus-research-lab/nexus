package workspace

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

	var liveRows []protocol.Message
	var transcript []map[string]any
	var arrival int64
	for _, round := range rounds {
		if err := history.AppendRoundMarkerWithOptions(workspacePath, sessionKey, round.roundID, round.content, round.at, round.options); err != nil {
			t.Fatal(err)
		}
		// 实时链路在受理输入时用同一条 marker 物化用户行。
		liveRows = append(liveRows, materializeRoundMarkerMessages(sessionKey, "Amy", []transcriptRoundMarker{{
			RoundID: round.roundID, UserMessageID: round.options.UserMessageID, AgentRoundID: round.options.AgentRoundID,
			Content: round.content, Timestamp: round.at, Synthetic: round.options.Synthetic,
			Purpose: round.options.Purpose, Metadata: round.options.Metadata,
		}})...)
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
	for _, id := range []string{"msg-1", "system_compact_boundary_round-1", "msg-2", "msg_user_round-wake", "msg-3", "msg-4", "msg-5"} {
		if !slices.ContainsFunc(refreshed, func(row protocol.Message) bool { return row["message_id"] == id }) {
			t.Errorf("刷新后的历史缺少 %s", id)
		}
	}
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
