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
