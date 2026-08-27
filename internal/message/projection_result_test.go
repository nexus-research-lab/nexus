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

func TestGoalCompletionReceiptOnlyExposesKnownSettlementValues(t *testing.T) {
	seconds := int64(754)
	tests := []struct {
		name       string
		report     *protocol.GoalUsageReport
		wantTime   *int64
		wantTokens *int64
	}{
		{name: "completion only"},
		{
			name: "duration without unavailable tokens",
			report: &protocol.GoalUsageReport{
				Status:          protocol.GoalStatusComplete,
				TimeUsedSeconds: seconds,
			},
			wantTime: &seconds,
		},
		{
			name: "finalized zero is authoritative",
			report: &protocol.GoalUsageReport{
				Status:         protocol.GoalStatusComplete,
				UsageFinalized: true,
			},
			wantTokens: int64TestPointer(0),
		},
		{
			name: "duration and finalized actual tokens",
			report: &protocol.GoalUsageReport{
				Status:          protocol.GoalStatusComplete,
				TimeUsedSeconds: seconds,
				UsageFinalized:  true,
				Usage: protocol.GoalUsage{
					ActualTotalTokens: 62762,
					ActualTotalKnown:  true,
				},
			},
			wantTime:   &seconds,
			wantTokens: int64TestPointer(62762),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			receipt := BuildGoalCompletionReceipt("goal-1", "round-1", test.report)
			if !optionalInt64Equal(receipt.TimeUsedSeconds, test.wantTime) ||
				!optionalInt64Equal(receipt.ActualTokens, test.wantTokens) {
				t.Fatalf("receipt = %+v, want time=%v tokens=%v", receipt, test.wantTime, test.wantTokens)
			}
		})
	}
}

func TestAttachGoalCompletionReceiptRequiresExactRoundIdentity(t *testing.T) {
	assistant := protocol.Message{
		"message_id":     "assistant-final",
		"role":           "assistant",
		"round_id":       "root-round",
		"agent_round_id": "agent-round",
	}
	receipt := protocol.GoalCompletionReceipt{GoalID: "goal-1", RoundID: "agent-round"}
	attached, ok := AttachGoalCompletionReceipt(assistant, receipt)
	if !ok || attached[protocol.GoalCompletionReceiptField] == nil {
		t.Fatalf("exact receipt was not attached: %+v", attached)
	}
	receipt.RoundID = "other-round"
	if attached, ok := AttachGoalCompletionReceipt(assistant, receipt); ok || attached != nil {
		t.Fatalf("mismatched receipt was attached: %+v", attached)
	}
}

func int64TestPointer(value int64) *int64 { return &value }

func optionalInt64Equal(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func TestBuildSyntheticAssistantFromInterruptedResultNormalizesDisplayText(t *testing.T) {
	tests := map[string]struct {
		result      string
		wantText    string
		wantSummary bool
	}{
		"internal sentinel": {
			result: InterruptWithoutMessage,
		},
		"custom reason": {
			result:      "  收到新消息，上一轮已停止  ",
			wantText:    "收到新消息，上一轮已停止",
			wantSummary: true,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			synthetic := BuildSyntheticAssistantFromResult(protocol.Message{
				"message_id":  "result-interrupted",
				"session_key": "agent:nexus:ws:dm:test",
				"agent_id":    "nexus",
				"round_id":    "round-interrupted",
				"role":        "result",
				"subtype":     "interrupted",
				"result":      test.result,
			})
			if got := ExtractAssistantDisplayText(synthetic); got != test.wantText {
				t.Fatalf("assistant 展示文案=%q，期望=%q: %+v", got, test.wantText, synthetic)
			}
			summary, ok := synthetic["result_summary"].(map[string]any)
			if !ok {
				t.Fatalf("interrupted assistant 缺少 result_summary: %+v", synthetic)
			}
			resultText, hasResult := summary["result"]
			if hasResult != test.wantSummary || (hasResult && resultText != test.wantText) {
				t.Fatalf("result_summary 展示文案不正确: %+v", summary)
			}
		})
	}
}

func TestProjectResultMessageSkipsEmptySuccessfulResult(t *testing.T) {
	projected := ProjectResultMessage(nil, protocol.Message{
		"message_id": "result-empty",
		"round_id":   "round-empty",
		"role":       "result",
		"subtype":    "success",
		"is_error":   false,
	})
	if projected != nil {
		t.Fatalf("empty successful result should not create assistant bubble: %+v", projected)
	}
}

func TestProjectResultMessageKeepsEmptyErrorResult(t *testing.T) {
	projected := ProjectResultMessage(nil, protocol.Message{
		"message_id": "result-error",
		"round_id":   "round-error",
		"role":       "result",
		"subtype":    "error",
		"is_error":   true,
	})
	if projected == nil {
		t.Fatal("error result must remain visible even without result text")
	}
}

func TestProjectResultMessageKeepsRoomInterruptedSlotWithoutText(t *testing.T) {
	projected := ProjectResultMessage(nil, protocol.Message{
		"agent_id":        "agent-1",
		"agent_round_id":  "agent-round-1",
		"conversation_id": "conversation-1",
		"message_id":      "result-interrupted",
		"room_id":         "room-1",
		"round_id":        "round-1",
		"role":            "result",
		"subtype":         "interrupted",
		"is_error":        false,
	})
	if projected == nil {
		t.Fatal("Room interrupted slot should remain visible without a result bubble")
	}
	if projected["role"] != "assistant" {
		t.Fatalf("Room interrupted slot should project to assistant card: %+v", projected)
	}
	if projected["agent_round_id"] != "agent-round-1" {
		t.Fatalf("Room interrupted slot must preserve agent_round_id: %+v", projected)
	}
	if projected["content"] == nil {
		t.Fatalf("Room interrupted slot should expose empty content explicitly: %+v", projected)
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

func TestBuildAssistantResultSummaryPreservesRuntimeDiagnostics(t *testing.T) {
	summary := BuildAssistantResultSummary(protocol.Message{
		"message_id":        "result-diagnostics",
		"role":              "result",
		"subtype":           "error",
		"runtime_subtype":   "error_max_turns",
		"failure_phase":     "runtime_startup",
		"is_error":          true,
		"model_usage":       map[string]any{"glm-5.2": map[string]any{"input_tokens": 42}},
		"structured_output": map[string]any{"status": "failed"},
		"fast_mode_state":   "cooldown",
	}, "")

	if summary["runtime_subtype"] != "error_max_turns" || summary["fast_mode_state"] != "cooldown" {
		t.Fatalf("runtime diagnostics missing: %+v", summary)
	}
	if summary["failure_phase"] != "runtime_startup" {
		t.Fatalf("failure phase missing: %+v", summary)
	}
	if summary["model_usage"] == nil || summary["structured_output"] == nil {
		t.Fatalf("result payload diagnostics missing: %+v", summary)
	}
}
