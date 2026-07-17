package message

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildSyntheticAssistantFromResultMapsStopReasonBySubtype(t *testing.T) {
	testCases := []struct {
		name               string
		subtype            string
		expectedStopReason string
	}{
		{
			name:               "success maps to end_turn",
			subtype:            "success",
			expectedStopReason: "end_turn",
		},
		{
			name:               "error maps to error",
			subtype:            "error",
			expectedStopReason: "error",
		},
		{
			name:               "interrupted maps to cancelled",
			subtype:            "interrupted",
			expectedStopReason: "cancelled",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			synthetic := BuildSyntheticAssistantFromResult(protocol.Message{
				"message_id":      "result-1",
				"session_key":     "agent:nexus:ws:dm:test",
				"agent_id":        "nexus",
				"round_id":        "round-1",
				"role":            "result",
				"subtype":         testCase.subtype,
				"timestamp":       int64(1000),
				"duration_ms":     12,
				"duration_api_ms": 8,
				"num_turns":       1,
				"is_error":        testCase.subtype == "error",
			})

			if normalizeString(synthetic["stop_reason"]) != testCase.expectedStopReason {
				t.Fatalf("stop_reason 不正确: got=%q want=%q synthetic=%+v", synthetic["stop_reason"], testCase.expectedStopReason, synthetic)
			}
			if !boolFromAny(synthetic["is_complete"]) {
				t.Fatalf("synthetic assistant 必须保持 is_complete=true: %+v", synthetic)
			}

			summary, ok := synthetic["result_summary"].(map[string]any)
			if !ok {
				t.Fatalf("synthetic assistant 应挂载 result_summary: %+v", synthetic)
			}
			if normalizeString(summary["subtype"]) != testCase.subtype {
				t.Fatalf("result_summary.subtype 不正确: got=%q want=%q summary=%+v", summary["subtype"], testCase.subtype, summary)
			}
		})
	}
}

func TestBuildSyntheticAssistantFromResultPreservesExecutionIdentity(t *testing.T) {
	synthetic := BuildSyntheticAssistantFromResult(protocol.Message{
		"message_id":     "result-1",
		"session_key":    "room:group:conversation-1",
		"agent_id":       "agent-1",
		"round_id":       "round-root",
		"agent_round_id": "agent-round-1",
		"parent_id":      "slot-message-1",
		"model":          "kimi-for-coding",
		"role":           "result",
		"subtype":        "success",
		"result":         "完成",
		"timestamp":      int64(1000),
	})

	for key, expected := range map[string]string{
		"agent_round_id": "agent-round-1",
		"parent_id":      "slot-message-1",
		"model":          "kimi-for-coding",
	} {
		if actual := normalizeString(synthetic[key]); actual != expected {
			t.Fatalf("synthetic assistant 未保留 %s: got=%q want=%q synthetic=%+v", key, actual, expected, synthetic)
		}
	}
}

func TestAttachResultSummaryMarksAssistantComplete(t *testing.T) {
	assistant := protocol.Message{
		"message_id":  "assistant-1",
		"session_key": "agent:nexus:ws:dm:test",
		"agent_id":    "nexus",
		"round_id":    "round-1",
		"role":        "assistant",
		"is_complete": false,
		"content": []map[string]any{{
			"type": "text",
			"text": "done",
		}},
	}
	result := protocol.Message{
		"message_id": "result-1",
		"round_id":   "round-1",
		"role":       "result",
		"subtype":    "success",
		"result":     "done",
	}

	merged, ok := AttachResultSummary(assistant, result)
	if !ok {
		t.Fatalf("assistant 应挂载 result_summary")
	}
	if !boolFromAny(merged["is_complete"]) {
		t.Fatalf("挂载 result_summary 后 assistant 必须变为完成态: %+v", merged)
	}
	if normalizeString(merged["stop_reason"]) != "end_turn" {
		t.Fatalf("挂载 result_summary 后 stop_reason 不正确: %+v", merged)
	}
	if _, ok := merged["result_summary"].(map[string]any); !ok {
		t.Fatalf("assistant 应保留 result_summary: %+v", merged)
	}
}

func TestAttachResultSummaryRejectsDifferentAgentRound(t *testing.T) {
	assistant := protocol.Message{
		"message_id":     "assistant-1",
		"agent_id":       "agent-1",
		"round_id":       "round-root",
		"agent_round_id": "agent-round-1",
		"role":           "assistant",
		"content":        []map[string]any{{"type": "text", "text": "Agent1"}},
	}
	result := protocol.Message{
		"message_id":     "result-2",
		"agent_id":       "agent-2",
		"round_id":       "round-root",
		"agent_round_id": "agent-round-2",
		"role":           "result",
		"subtype":        "success",
		"result":         "Agent2",
	}

	if _, ok := AttachResultSummary(assistant, result); ok {
		t.Fatalf("不同 Agent 执行轮的 result 不应挂到 assistant")
	}
}

func TestBuildAssistantResultSummaryPreservesPermissionDenials(t *testing.T) {
	summary := BuildAssistantResultSummary(protocol.Message{
		"message_id": "result-1",
		"role":       "result",
		"subtype":    "success",
		"result":     "无法完成搜索：WebSearch 未被允许",
		"permission_denials": []map[string]any{{
			"tool_name": "WebSearch",
		}},
		"errors": []string{"permission denied"},
	}, "")

	denials, ok := summary["permission_denials"].([]map[string]any)
	if !ok || len(denials) != 1 || denials[0]["tool_name"] != "WebSearch" {
		t.Fatalf("permission_denials 未进入 result_summary: %+v", summary)
	}
	errorsValue, ok := summary["errors"].([]string)
	if !ok || len(errorsValue) != 1 || errorsValue[0] != "permission denied" {
		t.Fatalf("errors 未进入 result_summary: %+v", summary)
	}
}
