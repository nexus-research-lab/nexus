package workspace

import (
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

func TestNormalizeHistoryRowsMergesExternalDeliveryReceipt(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id":  "user-1",
			"session_key": "agent:nexus:telegram:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "user",
			"content":     "处理一下",
			"timestamp":   int64(1000),
		},
		{
			"message_id":  "assistant-1",
			"session_key": "agent:nexus:telegram:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "已处理。"},
			},
			"is_complete": true,
			"timestamp":   int64(2000),
		},
		{
			overlayKindField:              overlayKindExternalDelivery,
			"message_id":                  "external_delivery_receipt:round-1:assistant-1:telegram:-1001:42",
			"role":                        overlayKindExternalDelivery,
			"round_id":                    "round-1",
			"assistant_message_id":        "assistant-1",
			"channel":                     "telegram",
			"target":                      "-1001",
			"thread_id":                   "12",
			"primary_platform_message_id": "42",
			"platform_message_ids":        []any{"42", "43"},
			"timestamp":                   int64(3000),
		},
		{
			"message_id":  "result-1",
			"session_key": "agent:nexus:telegram:dm:test",
			"agent_id":    "nexus",
			"round_id":    "round-1",
			"role":        "result",
			"subtype":     "success",
			"result":      "已处理。",
			"timestamp":   int64(4000),
		},
	}

	normalized := normalizeHistoryRows(rows, nil)
	if len(normalized) != 2 {
		t.Fatalf("回执控制行不应作为可见消息返回: %+v", normalized)
	}
	assistant := normalized[1]
	delivery, ok := assistant["external_delivery"].(map[string]any)
	if !ok {
		t.Fatalf("assistant 未挂载 external_delivery: %+v", assistant)
	}
	if delivery["channel"] != "telegram" || delivery["target"] != "-1001" || delivery["thread_id"] != "12" {
		t.Fatalf("外部投递目标不正确: %+v", delivery)
	}
	if delivery["primary_platform_message_id"] != "42" {
		t.Fatalf("外部投递主平台 message id 不正确: %+v", delivery)
	}
	if delivery["delivered_at"] != int64(3000) {
		t.Fatalf("外部投递时间不正确: %+v", delivery)
	}
	ids, ok := delivery["platform_message_ids"].([]string)
	if !ok || len(ids) != 2 || ids[0] != "42" || ids[1] != "43" {
		t.Fatalf("外部投递平台 message ids 不正确: %+v", delivery)
	}
	deliveries, ok := assistant["external_deliveries"].([]map[string]any)
	if !ok || len(deliveries) != 1 {
		t.Fatalf("assistant 应保留 external_deliveries 列表: %+v", assistant)
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
	if stringFromAny(normalized[1]["stop_reason"]) != "cancelled" {
		t.Fatalf("合成 assistant 应补 stop_reason=cancelled: %+v", normalized[1])
	}
	summary, ok := normalized[1]["result_summary"].(map[string]any)
	if !ok || stringFromAny(summary["subtype"]) != "interrupted" {
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
	if normalized[0]["role"] != "user" || stringFromAny(normalized[0]["content"]) != "继续" {
		t.Fatalf("内部 user prompt 未被过滤: %+v", normalized)
	}
	if normalized[1]["role"] != "assistant" {
		t.Fatalf("结果应继续收口到 assistant: %+v", normalized)
	}
}
