package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestGoalUsageFromRawRejectsProviderZeroThatContradictsBreakdown(t *testing.T) {
	usage, ok := GoalUsageFromRaw(map[string]any{
		"input_tokens":            100,
		"output_tokens":           20,
		"cache_read_input_tokens": 80,
		"total_tokens":            0,
	})
	if !ok {
		t.Fatal("GoalUsageFromRaw() ok = false, want true")
	}

	if !usage.ActualTotalKnown || usage.ActualTokens() != 200 || !usage.ActualTokensAreEstimated() {
		t.Fatalf("actual usage = %#v, want estimated breakdown total 200", usage)
	}
	if usage.BudgetTokens() != 120 {
		t.Fatalf("budget usage = %#v, want 120", usage)
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

func TestGoalUsageFromTokenUsageRecognizesExactModelUsageAggregation(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:  300,
		OutputTokens: 30,
		TotalTokens:  330,
		Raw: map[string]any{
			"model-a": map[string]any{
				"input_tokens":  100,
				"output_tokens": 10,
				"total_tokens":  110,
			},
			"model-b": map[string]any{
				"input_tokens":  200,
				"output_tokens": 20,
				"total_tokens":  220,
			},
		},
	})

	if usage.ActualTokens() != 330 || usage.ActualTokensAreEstimated() {
		t.Fatalf("model usage aggregate = %#v, want exact provider totals 330", usage)
	}
}

func TestGoalUsageFromTokenUsageRecognizesNestedProviderTotalAlias(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:  100,
		OutputTokens: 20,
		TotalTokens:  120,
		Raw: map[string]any{
			"input_tokens":  100,
			"output_tokens": 20,
			"total_tokens":  120,
			"raw": map[string]any{
				"promptTokenCount":     100,
				"candidatesTokenCount": 20,
				"totalTokenCount":      130,
				"thoughtsTokenCount":   10,
			},
		},
	})

	if usage.ActualTokens() != 130 || usage.ActualTokensAreEstimated() {
		t.Fatalf("provider alias usage = %#v, want exact nested total 130", usage)
	}
	if usage.ReasoningTokens != 10 {
		t.Fatalf("provider alias reasoning = %d, want 10", usage.ReasoningTokens)
	}
}

// GoalUsageFromTokenUsage 把 SDK usage 转成 Goal accounting 口径。
func GoalUsageFromTokenUsage(usage sdkprotocol.TokenUsage) protocol.GoalUsage {
	goalUsage, _ := GoalUsageFromTokenUsageWithPresence(usage)
	return goalUsage
}
