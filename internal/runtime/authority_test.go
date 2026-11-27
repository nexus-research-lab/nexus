// authority_test.go 覆盖 Agent 撤销、Goal 权威与用量、职责权威与 WorkBinding 状态测试。
package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestGoalAuthorityStateRejectsGoalSwitchAndRevisionRegression(t *testing.T) {
	t.Parallel()

	state := NewGoalAuthorityState("goal-1", 2, "execution-1")
	if state.Bind("goal-2", 3, "execution-2") {
		t.Fatal("one physical round must not switch Goal identity")
	}
	if state.Bind("goal-1", 1, "execution-old") {
		t.Fatal("authority revision must not regress")
	}
	if !state.Bind("goal-1", 3, "") {
		t.Fatal("same Goal should adopt a newer revision with optional ExecutionID")
	}
	authority, ok := state.Load()
	if !ok || authority.ObjectiveRevision != 3 || authority.ExecutionID != "" {
		t.Fatalf("advanced authority = %#v, ok=%t", authority, ok)
	}
}

func TestResultUsageLimitReachedDetectsExplicitUsageLimit(t *testing.T) {
	result := &sdkprotocol.ResultMessage{
		IsError:        true,
		TerminalReason: "error",
		Result:         "You've hit your usage limit. Try again later.",
		Additional: map[string]any{
			"error": map[string]any{
				"type": "usage_limit_reached",
			},
		},
	}

	ok, reason := ResultUsageLimitReached(result)
	if !ok {
		t.Fatal("ResultUsageLimitReached() ok = false, want true")
	}
	if reason != result.Result {
		t.Fatalf("reason = %q, want result text", reason)
	}
}

func TestGoalUsageFromTokenUsageAggregatesNestedNXSModelUsageBreakdown(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:  150,
		OutputTokens: 30,
		TotalTokens:  180,
		Raw: map[string]any{
			"model-a": map[string]any{
				"input_tokens":  100,
				"output_tokens": 20,
				"total_tokens":  120,
				"raw": map[string]any{
					"input_tokens":            100,
					"output_tokens":           20,
					"cache_read_input_tokens": 50,
				},
			},
			"model-b": map[string]any{
				"input_tokens":  50,
				"output_tokens": 10,
				"total_tokens":  60,
				"raw": map[string]any{
					"input_tokens":          50,
					"output_tokens":         10,
					"reasoning_tokens":      15,
					"cache_creation_tokens": 5,
				},
			},
		},
	})

	if usage.ActualTokens() != 235 || !usage.ActualTokensAreEstimated() {
		t.Fatalf("model usage aggregate = %#v, want estimated actual 235", usage)
	}
	if usage.CacheReadInputTokens != 50 ||
		usage.CacheCreationInputTokens != 5 ||
		usage.ReasoningTokens != 15 {
		t.Fatalf("model usage nested breakdown = %#v, want aggregated cache/reasoning", usage)
	}
}

// GoalUsageFromTokenUsage 把 SDK usage 转成 Goal accounting 口径。
func GoalUsageFromTokenUsage(usage sdkprotocol.TokenUsage) protocol.GoalUsage {
	goalUsage, _ := GoalUsageFromTokenUsageWithPresence(usage)
	return goalUsage
}

func TestResponsibilityAuthorityTerminalReleaseRejectsStaticExecutionReseed(t *testing.T) {
	state := NewResponsibilityAuthorityState(nil, "execution-old", nil, nil)
	if !state.RevokeExecution("execution-old") {
		t.Fatal("RevokeExecution() = false")
	}
	if state.SeedExecution("execution-old") {
		t.Fatal("stale static actor reseeded terminal execution")
	}
	if state.BindCoordination("execution-old") {
		t.Fatal("late coordination receipt rebound terminal execution")
	}
	authority, _ := state.Load()
	if authority.ExecutionID != "" || authority.Lane != ResponsibilityLaneUnbound {
		t.Fatalf("authority = %#v, want released execution to remain unbound", authority)
	}
}
