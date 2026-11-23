package message

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// 只追加 transcript 的真实形态：assistant 逐块落盘且不带 stop_reason，round 终态只由 result 行承载。
func TestMergeHistoryResultSummariesSettlesAssistantFromResult(t *testing.T) {
	tests := []struct {
		name           string
		subtype        string
		wantStopReason string
	}{
		{name: "success", subtype: "success", wantStopReason: "end_turn"},
		{name: "error", subtype: "error", wantStopReason: "error"},
		{name: "interrupted", subtype: "interrupted", wantStopReason: "cancelled"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := []protocol.Message{
				{"message_id": "u1", "role": "user", "round_id": "round-1", "content": "hi"},
				{
					"message_id": "a1", "role": "assistant", "round_id": "round-1", "agent_round_id": "ar-1",
					"content": []any{map[string]any{"type": "thinking", "thinking": "plan"}}, "is_complete": false,
				},
				{
					"message_id": "a2", "role": "assistant", "round_id": "round-1", "agent_round_id": "ar-1",
					"content": []any{map[string]any{"type": "text", "text": "done"}}, "is_complete": false,
				},
				{
					// 同 root round 的另一 Agent 执行轮尚无 result，必须保持未完成。
					"message_id": "b1", "role": "assistant", "round_id": "round-1", "agent_round_id": "ar-2",
					"content": []any{map[string]any{"type": "text", "text": "still going"}}, "is_complete": false,
				},
				{
					"message_id": "r1", "role": "result", "round_id": "round-1", "agent_round_id": "ar-1",
					"subtype": test.subtype, "duration_ms": 1200, "result": "done",
				},
			}

			merged := MergeHistoryResultSummaries(rows)

			final := findHistoryRow(t, merged, "a2")
			if final["result_summary"] == nil {
				t.Fatalf("final assistant lost result_summary: %#v", final)
			}
			if final["is_complete"] != true {
				t.Fatalf("final assistant is_complete = %#v, want true", final["is_complete"])
			}
			if final["stop_reason"] != test.wantStopReason {
				t.Fatalf("final assistant stop_reason = %#v, want %q", final["stop_reason"], test.wantStopReason)
			}
			earlier := findHistoryRow(t, merged, "a1")
			if earlier["is_complete"] != true {
				t.Fatalf("earlier block of the settled agent round is_complete = %#v, want true", earlier["is_complete"])
			}
			if earlier["stop_reason"] != nil || earlier["result_summary"] != nil {
				t.Fatalf("terminal metadata must stay on the final block only: %#v", earlier)
			}
			if other := findHistoryRow(t, merged, "b1"); other["is_complete"] != false {
				t.Fatalf("another agent round without result must stay open, got %#v", other["is_complete"])
			}
			if rows[2]["is_complete"] != false {
				t.Fatalf("input history must stay untouched, got %#v", rows[2])
			}
		})
	}
}

func findHistoryRow(t *testing.T, rows []protocol.Message, messageID string) protocol.Message {
	t.Helper()
	for _, row := range rows {
		if row["message_id"] == messageID {
			return row
		}
	}
	t.Fatalf("message %q missing from merged history", messageID)
	return nil
}
