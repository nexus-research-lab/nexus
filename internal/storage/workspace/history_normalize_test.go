package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeHistoryRowsKeepsResultUsageWhenSuppressingDuplicateResultText(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "你好",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"parent_id":   "round-1",
			"role":        "assistant",
			"content": []map[string]any{
				{
					"type": "text",
					"text": "你好，世界",
				},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   2000,
		},
		{
			"message_id":      "result-1",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "round-1",
			"role":            "result",
			"subtype":         "success",
			"result":          "你好，世界",
			"timestamp":       3000,
			"duration_ms":     1200,
			"duration_api_ms": 1100,
			"num_turns":       1,
			"usage": map[string]any{
				"input_tokens":  3,
				"output_tokens": 5,
			},
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("历史消息数量不正确: got=%d want=2", len(normalized))
	}

	assistant := normalized[1]
	resultSummary, ok := assistant["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("assistant 应挂载 result_summary: %+v", assistant)
	}
	if _, exists := resultSummary["result"]; exists {
		t.Fatalf("重复 result 文本应被移除: %+v", resultSummary)
	}
	usage, ok := resultSummary["usage"].(map[string]any)
	if !ok {
		t.Fatalf("result usage 不应丢失: %+v", resultSummary)
	}
	if intFromAny(usage["output_tokens"]) != 5 {
		t.Fatalf("result usage 不正确: %+v", usage)
	}
}

func TestNormalizeHistoryRowsFiltersInternalTransportRows(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "system-retry",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "system",
			"content":     "API 正在重试",
			"timestamp":   900,
			"metadata": map[string]any{
				"subtype": "api_retry",
			},
		},
		{
			"message_id":  "user-internal",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "[Request interrupted by user]",
			"timestamp":   950,
		},
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-2",
			"role":        "user",
			"content":     "继续",
			"timestamp":   1000,
		},
		{
			"message_id":  "assistant-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-2",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "好的，继续。"},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   1100,
		},
		{
			"message_id":      "result-1",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "round-2",
			"role":            "result",
			"subtype":         "success",
			"timestamp":       1200,
			"duration_ms":     10,
			"duration_api_ms": 8,
			"num_turns":       1,
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 2,
			},
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("内部 transport 行应被过滤: got=%d want=2 rows=%+v", len(normalized), normalized)
	}
	if normalized[0]["role"] != "user" || stringFromAny(normalized[0]["content"]) != "继续" {
		t.Fatalf("内部 user prompt 未被过滤: %+v", normalized)
	}
	if normalized[1]["role"] != "assistant" {
		t.Fatalf("结果应继续收口到 assistant: %+v", normalized)
	}
}

func TestNormalizeHistoryRowsSuppressesTranscriptGuidanceWhenDurableUserExists(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":      "source-round",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "root-round",
			"role":            "system",
			"content":         "然后给点建议",
			"timestamp":       1000,
			"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
			"metadata": map[string]any{
				"subtype":         message.SystemMessageSubtypeGuidedInput,
				"source_round_id": "source-round",
			},
		},
		{
			"message_id":      "user-source-round",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "root-round",
			"source_round_id": "source-round",
			"role":            "user",
			"content":         "然后给点建议",
			"timestamp":       1001,
			"delivery_policy": string(protocol.ChatDeliveryPolicyGuide),
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	userCount := 0
	for _, row := range normalized {
		if row["role"] == "system" {
			t.Fatalf("同一引导不应保留 transcript system 投影: %+v", normalized)
		}
		if row["role"] == "user" && row["source_round_id"] == "source-round" {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("同一引导应只保留一条 durable user 投影: %+v", normalized)
	}
}

func TestSortHistoryRowsUsesDisplayOrderForSameTimestamp(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "assistant-b", "role": "assistant", "round_id": "root", "timestamp": int64(10), "display_order": int64(10001)},
		{"message_id": "assistant-a", "role": "assistant", "round_id": "root", "timestamp": int64(10), "display_order": int64(10000)},
	}
	sortHistoryRows(rows)
	if rows[0]["message_id"] != "assistant-a" || rows[1]["message_id"] != "assistant-b" {
		t.Fatalf("同一时间应按 display_order 稳定排序: %#v", rows)
	}
}

func TestCompactMessagesMergesGoalCompletionReceiptIntoOriginalAssistant(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"round_id":   "round-1",
			"content":    []map[string]any{{"type": "text", "text": "最终交付"}},
			"timestamp":  int64(1000),
		},
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"round_id":   "round-1",
			protocol.GoalCompletionReceiptField: protocol.GoalCompletionReceipt{
				GoalID: "goal-1", RoundID: "round-1", ActualTokens: int64TestPointer(42),
			},
			"timestamp": int64(1000),
		},
	}

	compacted := compactMessages(rows)
	if len(compacted) != 1 {
		t.Fatalf("compacted = %+v, want one assistant", compacted)
	}
	if compacted[0][protocol.GoalCompletionReceiptField] == nil {
		t.Fatalf("completion receipt was lost: %+v", compacted[0])
	}
	blocks := normalizeMessageContentBlocks(compacted[0]["content"])
	if len(blocks) != 1 || stringFromAny(blocks[0]["text"]) != "最终交付" {
		t.Fatalf("original assistant content was lost: %+v", compacted[0])
	}
}

func TestSortHistoryRowsPlacesGoalControlBeforeContinuationAtSameMillisecond(t *testing.T) {
	rows := []protocol.Message{
		{"message_id": "msg_a", "round_id": "goal-continuation", "role": "assistant", "timestamp": int64(1000)},
		{
			"message_id":   "msg_z",
			"round_id":     "goal-command",
			"role":         "user",
			"timestamp":    int64(1000),
			"control_only": true,
			"metadata":     map[string]string{"subtype": "goal_set"},
		},
	}

	sortHistoryRows(rows)

	if rows[0]["message_id"] != "msg_z" || rows[1]["message_id"] != "msg_a" {
		t.Fatalf("same-millisecond Goal order = %#v, want control before continuation", rows)
	}
}

func int64TestPointer(value int64) *int64 { return &value }
