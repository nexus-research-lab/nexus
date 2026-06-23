package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeHistoryRowsTreatsCompletedAssistantWithoutResultAsSuccess(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-finished-1",
			"session_key": "agent:nexus:ws:dm:test-finished",
			"agent_id":    "nexus",
			"round_id":    "round-finished-1",
			"role":        "user",
			"content":     "解释一下这个错误",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-finished-1",
			"session_key": "agent:nexus:ws:dm:test-finished",
			"agent_id":    "nexus",
			"round_id":    "round-finished-1",
			"parent_id":   "round-finished-1",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "这个错误表示参数为空。"},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   2000,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("completed assistant round 不应物化 interrupted result: got=%d want=2", len(normalized))
	}
	if normalized[1]["role"] != "assistant" {
		t.Fatalf("第二条消息应为 assistant: %+v", normalized[1])
	}
	if _, exists := normalized[1]["stream_status"]; exists {
		t.Fatalf("completed assistant 不应补写 stream_status: %+v", normalized[1])
	}
}

func TestNormalizeHistoryRowsTreatsAssistantStopReasonAsTerminalTruth(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-stop-1",
			"session_key": "agent:nexus:ws:dm:test-stop-reason",
			"agent_id":    "nexus",
			"round_id":    "round-stop-1",
			"role":        "user",
			"content":     "继续",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-stop-1",
			"session_key": "agent:nexus:ws:dm:test-stop-reason",
			"agent_id":    "nexus",
			"round_id":    "round-stop-1",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "已结束。"},
			},
			"stop_reason": "end_turn",
			"timestamp":   2000,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("stop_reason=end_turn 不应再物化 interrupted: got=%d want=2 rows=%+v", len(normalized), normalized)
	}
}

func TestNormalizeHistoryRowsTreatsCancelledAssistantAsInterrupted(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-cancelled-1",
			"session_key": "agent:nexus:ws:dm:test-cancelled",
			"agent_id":    "nexus",
			"round_id":    "round-cancelled-1",
			"role":        "user",
			"content":     "停止",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-cancelled-1",
			"session_key": "agent:nexus:ws:dm:test-cancelled",
			"agent_id":    "nexus",
			"round_id":    "round-cancelled-1",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "处理中"},
			},
			"stop_reason": "cancelled",
			"timestamp":   2000,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("cancelled assistant 不应额外物化新消息: got=%d want=2 rows=%+v", len(normalized), normalized)
	}
}

func TestNormalizeHistoryRowsMaterializesRunningAssistantWithoutStopReason(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-running-1",
			"session_key": "agent:nexus:ws:dm:test-running",
			"agent_id":    "nexus",
			"round_id":    "round-running-1",
			"role":        "user",
			"content":     "还在吗",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-running-1",
			"session_key": "agent:nexus:ws:dm:test-running",
			"agent_id":    "nexus",
			"round_id":    "round-running-1",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "处理中"},
			},
			"timestamp": 2000,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 3 {
		t.Fatalf("running assistant 应保留 partial assistant 并追加终态 assistant: got=%d want=3 rows=%+v", len(normalized), normalized)
	}
	if stringFromAny(normalized[1]["stop_reason"]) != "" {
		t.Fatalf("partial assistant 不应被提前写成终态: %+v", normalized[1])
	}
	assistant := normalized[2]
	if assistant["role"] != "assistant" {
		t.Fatalf("物化结果应为 assistant: %+v", assistant)
	}
	if stringFromAny(assistant["stop_reason"]) != "cancelled" {
		t.Fatalf("未完成 round 物化后 stop_reason 应为 cancelled: %+v", assistant)
	}
	summary, ok := assistant["result_summary"].(map[string]any)
	if !ok || stringFromAny(summary["subtype"]) != "interrupted" {
		t.Fatalf("未完成 round 物化后应挂 interrupted result_summary: %+v", assistant)
	}
	if _, exists := summary["result"]; exists {
		t.Fatalf("未完成 round 物化后不应保留默认 result 文本: %+v", summary)
	}
}

func TestBuildRoomTranscriptReferenceRejectsResultRole(t *testing.T) {
	row := buildRoomTranscriptReference(protocol.Message{
		"message_id": "result-1",
		"session_id": "sdk-session-1",
		"agent_id":   "nexus",
		"round_id":   "round-1",
		"role":       "result",
		"timestamp":  1000,
	}, "/tmp/workspace", "agent:nexus:ws:dm:test")
	if row != nil {
		t.Fatalf("result 行不应生成 transcript_ref: %+v", row)
	}
}
