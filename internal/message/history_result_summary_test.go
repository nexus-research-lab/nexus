package message

import (
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestHistoryResultProjectionKeepsPhysicalAgentIdentity(t *testing.T) {
	rows := []protocol.Message{
		{"role": "assistant", "message_id": "assistant-a", "round_id": "root", "agent_round_id": "a-round", "agent_id": "a", "content": []map[string]any{{"type": "text", "text": "A"}}},
		{"role": "assistant", "message_id": "assistant-b", "round_id": "root", "agent_round_id": "b-round", "agent_id": "b", "content": []map[string]any{{"type": "text", "text": "B"}}},
		{"role": "result", "message_id": "result-a", "round_id": "root", "agent_round_id": "a-round", "agent_id": "a", "subtype": "success", "duration_ms": 10},
		{"role": "result", "message_id": "result-c", "round_id": "root", "agent_round_id": "c-round", "agent_id": "c", "subtype": "success", "duration_ms": 20},
	}
	original := make([]protocol.Message, len(rows))
	for i, row := range rows {
		original[i] = protocol.Clone(row)
	}
	result := MergeHistoryResultSummaries(rows)
	if !reflect.DeepEqual(rows, original) {
		t.Fatal("投影修改了原始历史")
	}
	if len(result) != 3 {
		t.Fatalf("rows=%+v", result)
	}
	summary, _ := result[0]["result_summary"].(map[string]any)
	if summary["message_id"] != "result-a" {
		t.Fatalf("A 的结果丢失: %+v", summary)
	}
	if _, exists := result[1]["result_summary"]; exists {
		t.Fatal("C 的结果误挂给 B")
	}
	if result[2]["agent_id"] != "c" {
		t.Fatalf("未匹配结果丢失身份: %+v", result[2])
	}
	if replay := MergeHistoryResultSummaries(result); !reflect.DeepEqual(replay, result) {
		t.Fatal("历史投影重放不幂等")
	}
}
