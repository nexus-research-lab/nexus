package message

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
