package workspace

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestNormalizeHistoryRowsMergesAssistantSnapshotsByMessageID(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "今天天气怎么样呀",
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
					"type":     "thinking",
					"thinking": "先想一下",
				},
			},
			"is_complete": true,
			"timestamp":   2000,
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
					"text": "我现在不能直接查天气。",
				},
			},
			"stop_reason": "end_turn",
			"is_complete": true,
			"timestamp":   3000,
		},
		{
			"message_id":  "result-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "result",
			"subtype":     "success",
			"result":      "我现在不能直接查天气。",
			"timestamp":   4000,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("历史消息压缩数量不正确: got=%d want=2", len(normalized))
	}

	assistant := normalized[1]
	if assistant["role"] != "assistant" {
		t.Fatalf("第二条消息应为 assistant: %+v", assistant)
	}
	blocks := normalizeMessageContentBlocks(assistant["content"])
	if len(blocks) != 2 {
		t.Fatalf("assistant 历史快照未合并 thinking 与 text: %+v", assistant)
	}
	if blocks[0]["type"] != "thinking" || blocks[0]["thinking"] != "先想一下" {
		t.Fatalf("thinking 块丢失: %+v", blocks)
	}
	if blocks[1]["type"] != "text" || blocks[1]["text"] != "我现在不能直接查天气。" {
		t.Fatalf("text 块丢失: %+v", blocks)
	}
	if _, exists := assistant["stream_status"]; exists {
		t.Fatalf("assistant 历史回放不应反推 stream_status: %+v", assistant)
	}

	resultSummary, ok := assistant["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("assistant 应挂载 result_summary: %+v", assistant)
	}
	if _, exists := resultSummary["result"]; exists {
		t.Fatalf("assistant 正文与 result 相同的场景不应再保留重复 result 文本: %+v", resultSummary)
	}
}

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

func TestNormalizeHistoryRowsBuildsSyntheticAssistantWithoutAssistant(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:ws:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "测试一下",
			"timestamp":   1000,
		},
		{
			"message_id":      "result-1",
			"session_key":     "agent:nexus:ws:dm:test",
			"agent_id":        "nexus",
			"round_id":        "round-1",
			"role":            "result",
			"subtype":         "interrupted",
			"timestamp":       2000,
			"duration_ms":     0,
			"duration_api_ms": 0,
			"num_turns":       0,
			"is_error":        false,
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("无 assistant 的终态结果不应被吞掉: got=%d want=2", len(normalized))
	}
	if normalized[1]["role"] != "assistant" {
		t.Fatalf("第二条消息应改写为 assistant: %+v", normalized[1])
	}
	if strings.TrimSpace(stringFromAny(normalized[1]["stop_reason"])) != "cancelled" {
		t.Fatalf("合成 assistant 应补 stop_reason=cancelled: %+v", normalized[1])
	}
	summary, ok := normalized[1]["result_summary"].(map[string]any)
	if !ok || strings.TrimSpace(stringFromAny(summary["subtype"])) != "interrupted" {
		t.Fatalf("合成 assistant 应携带 interrupted result_summary: %+v", normalized[1])
	}
	if _, exists := summary["result"]; exists {
		t.Fatalf("interrupted synthetic assistant 不应再塞默认 result 文本: %+v", summary)
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
	if normalized[0]["role"] != "user" || strings.TrimSpace(stringFromAny(normalized[0]["content"])) != "继续" {
		t.Fatalf("内部 user prompt 未被过滤: %+v", normalized)
	}
	if normalized[1]["role"] != "assistant" {
		t.Fatalf("结果应继续收口到 assistant: %+v", normalized)
	}
}

func TestPaginateNormalizedHistoryRowsKeepsDMRoundComplete(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "user-1",
			"round_id":   "round-1",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "assistant-1",
			"round_id":   "round-1",
			"role":       "assistant",
			"timestamp":  2000,
		},
		{
			"message_id": "result-1",
			"round_id":   "round-1",
			"role":       "result",
			"timestamp":  3000,
		},
		{
			"message_id": "user-2",
			"round_id":   "round-2",
			"role":       "user",
			"timestamp":  4000,
		},
		{
			"message_id": "assistant-2",
			"round_id":   "round-2",
			"role":       "assistant",
			"timestamp":  5000,
		},
		{
			"message_id": "result-2",
			"round_id":   "round-2",
			"role":       "result",
			"timestamp":  6000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, false)
	if !page.HasMore {
		t.Fatalf("按 round 分页时应仍有更早历史: %+v", page)
	}
	if len(page.Items) != 3 {
		t.Fatalf("最新一页应保留完整 round: got=%d want=3", len(page.Items))
	}
	if page.Items[0]["message_id"] != "user-2" || page.Items[2]["message_id"] != "result-2" {
		t.Fatalf("最新一页 round 边界不正确: %+v", page.Items)
	}

	olderPage := paginateNormalizedHistoryRows(
		rows,
		1,
		derefString(page.NextBeforeRoundID),
		derefInt64(page.NextBeforeRoundTimestamp),
		false,
	)
	if olderPage.HasMore {
		t.Fatalf("更早一页不应继续有历史: %+v", olderPage)
	}
	if len(olderPage.Items) != 3 {
		t.Fatalf("更早一页应保留完整 round: got=%d want=3", len(olderPage.Items))
	}
	if olderPage.Items[0]["message_id"] != "user-1" || olderPage.Items[2]["message_id"] != "result-1" {
		t.Fatalf("更早一页 round 边界不正确: %+v", olderPage.Items)
	}
}

func TestPaginateNormalizedHistoryRowsCollapsesRoomAgentSubRounds(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "room-user-1",
			"round_id":   "room-round-1",
			"role":       "user",
			"timestamp":  1000,
		},
		{
			"message_id": "room-assistant-a-1",
			"round_id":   "room-round-1:agent-a",
			"agent_id":   "agent-a",
			"role":       "assistant",
			"timestamp":  2000,
		},
		{
			"message_id": "room-result-a-1",
			"round_id":   "room-round-1:agent-a",
			"agent_id":   "agent-a",
			"role":       "result",
			"timestamp":  3000,
		},
		{
			"message_id": "room-assistant-b-1",
			"round_id":   "room-round-1:agent-b",
			"agent_id":   "agent-b",
			"role":       "assistant",
			"timestamp":  4000,
		},
		{
			"message_id": "room-result-b-1",
			"round_id":   "room-round-1:agent-b",
			"agent_id":   "agent-b",
			"role":       "result",
			"timestamp":  5000,
		},
		{
			"message_id": "room-user-2",
			"round_id":   "room-round-2",
			"role":       "user",
			"timestamp":  6000,
		},
		{
			"message_id": "room-assistant-c-2",
			"round_id":   "room-round-2:agent-c",
			"agent_id":   "agent-c",
			"role":       "assistant",
			"timestamp":  7000,
		},
	}

	page := paginateNormalizedHistoryRows(rows, 1, "", 0, true)
	if !page.HasMore {
		t.Fatalf("Room 最新页应仍有更早主 round: %+v", page)
	}
	if len(page.Items) != 2 {
		t.Fatalf("Room 最新页应保留完整主 round: got=%d want=2", len(page.Items))
	}
	if page.Items[0]["message_id"] != "room-user-2" || page.Items[1]["message_id"] != "room-assistant-c-2" {
		t.Fatalf("Room 最新页主 round 不正确: %+v", page.Items)
	}
}

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
	if strings.TrimSpace(stringFromAny(normalized[1]["stop_reason"])) != "" {
		t.Fatalf("partial assistant 不应被提前写成终态: %+v", normalized[1])
	}
	assistant := normalized[2]
	if assistant["role"] != "assistant" {
		t.Fatalf("物化结果应为 assistant: %+v", assistant)
	}
	if strings.TrimSpace(stringFromAny(assistant["stop_reason"])) != "cancelled" {
		t.Fatalf("未完成 round 物化后 stop_reason 应为 cancelled: %+v", assistant)
	}
	summary, ok := assistant["result_summary"].(map[string]any)
	if !ok || strings.TrimSpace(stringFromAny(summary["subtype"])) != "interrupted" {
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

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
