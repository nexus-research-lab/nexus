package workspace

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const runtimeWakeTestSummary = `Background command "Wait for stage1 confirmation" completed (exit code 0)`

func runtimeWakeTestChain() []transcriptEntry {
	return []transcriptEntry{
		{Index: 0, Data: map[string]any{
			"type": "user", "uuid": "user-1", "timestamp": "2026-11-19T01:00:00Z",
			"message": map[string]any{"role": "user", "content": "生成 PPT"},
		}},
		{Index: 1, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-1", "parentUuid": "user-1", "timestamp": "2026-11-19T01:00:05Z",
			"message": map[string]any{"id": "msg-1", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "请打开 127.0.0.1 确认第一阶段"},
			}},
		}},
		{Index: 2, Data: map[string]any{
			"type": "user", "uuid": "wake-1", "parentUuid": "assistant-1", "timestamp": "2026-11-19T01:01:00Z",
			"origin": map[string]any{"kind": "task-notification"},
			"message": map[string]any{"role": "user", "content": "<task-notification>\n<task-id>b1</task-id>\n<status>completed</status>\n<summary>" +
				runtimeWakeTestSummary + "</summary>\n</task-notification>"},
		}},
		{Index: 3, Data: map[string]any{
			"type": "assistant", "uuid": "assistant-2", "parentUuid": "wake-1", "timestamp": "2026-11-19T01:01:03Z",
			"message": map[string]any{"id": "msg-2", "role": "assistant", "content": []any{
				map[string]any{"type": "text", "text": "第二阶段开始"},
			}},
		}},
	}
}

func findRuntimeWakeRow(t *testing.T, rows []protocol.Message) protocol.Message {
	t.Helper()
	var found protocol.Message
	for _, row := range rows {
		if row["role"] != "user" {
			continue
		}
		if strings.Contains(stringFromAny(row["content"]), "<task-notification>") {
			t.Fatalf("原始 task-notification 标签被投影: %+v", row)
		}
		metadata, _ := row["metadata"].(map[string]any)
		if metadata["input_kind"] == "runtime_wake" {
			found = row
		}
	}
	if found == nil {
		t.Fatalf("没有投影出 runtime 唤醒触发行: %+v", rows)
	}
	if found["content"] != runtimeWakeTestSummary || found["is_synthetic"] != true {
		t.Fatalf("唤醒行 = %+v", found)
	}
	return found
}

// 没有 durable marker 的旧会话：唤醒条目仍成为独立轮次的触发行，而不是带原始标签的用户气泡。
func TestTranscriptRuntimeWakeProjectsTriggerRowWithoutMarker(t *testing.T) {
	rows := projectTranscriptChain("/workspace", "agent:a:ws:dm:c", "a", runtimeWakeTestChain(), nil)
	wake := findRuntimeWakeRow(t, rows)
	for _, row := range rows {
		if row["message_id"] == "msg-2" && row["round_id"] != wake["round_id"] {
			t.Fatalf("唤醒后的回复必须属于唤醒轮次: reply=%v wake=%v", row["round_id"], wake["round_id"])
		}
	}
}

// live 侧写入的唤醒 marker 必须与 transcript 唤醒条目对齐，刷新前后 round 身份一致；
// 普通用户输入不能抢走唤醒 marker。
func TestTranscriptRuntimeWakeAlignsWithLiveMarker(t *testing.T) {
	markers := []transcriptRoundMarker{
		{RoundID: "round-user", UserMessageID: "msg_user_round-user", Content: "生成 PPT", Timestamp: 1795050000000},
		{
			RoundID: "round-wake", UserMessageID: "msg_user_round-wake", Content: runtimeWakeTestSummary,
			Timestamp: 1795050060000, Synthetic: true, Purpose: "runtime_wake",
			Metadata: map[string]string{"input_kind": "runtime_wake", "trigger_kind": "background_task", "task_id": "b1", "task_status": "completed"},
		},
	}
	chain := runtimeWakeTestChain()
	aligned := alignTranscriptRoundMarkers(chain, markers)
	if len(aligned) != 2 || aligned[0].RoundID != "round-user" || aligned[1].RoundID != "round-wake" {
		t.Fatalf("aligned = %+v", aligned)
	}
	rows := projectTranscriptChain("/workspace", "agent:a:ws:dm:c", "a", chain, aligned)
	wake := findRuntimeWakeRow(t, rows)
	if wake["round_id"] != "round-wake" || wake["message_id"] != "msg_user_round-wake" {
		t.Fatalf("唤醒行身份 = %+v", wake)
	}

	materialized := roundMarkerHistoryTailPart("agent:a:ws:dm:c", "a", markers, []int{0, 1})
	findRuntimeWakeRow(t, materialized.rows)
}
