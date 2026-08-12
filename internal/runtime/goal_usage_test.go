package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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

func TestResultUsageLimitReachedIgnoresTokenAndContextLimits(t *testing.T) {
	tests := []*sdkprotocol.ResultMessage{
		{TerminalReason: "max_output_tokens", Result: "Maximum output tokens reached."},
		{TerminalReason: "context_length", Result: "Context length exceeded."},
		{TerminalReason: "rate_limit", Result: "Provider rate limit, retry later."},
	}
	for _, result := range tests {
		if ok, reason := ResultUsageLimitReached(result); ok {
			t.Fatalf("ResultUsageLimitReached(%#v) = true reason %q, want false", result, reason)
		}
	}
}

func TestGoalUsageFromTokenUsagePreservesProviderActualTotal(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:          3420,
		OutputTokens:         206,
		CacheReadInputTokens: 59136,
		TotalTokens:          62762,
		Raw: map[string]any{
			"input_tokens":            3420,
			"output_tokens":           206,
			"cache_read_input_tokens": 59136,
			"total_tokens":            62762,
		},
	})

	if usage.ActualTokens() != 62762 || usage.ActualTokensAreEstimated() {
		t.Fatalf("actual usage = %#v, want exact provider total 62762", usage)
	}
	if usage.BudgetTokens() != 3626 || usage.TotalTokens != 3626 {
		t.Fatalf("budget usage = %#v, want normalized input+output 3626", usage)
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

func TestGoalUsageFromRawRejectsNestedProviderZeroThatContradictsBreakdown(t *testing.T) {
	usage, present := GoalUsageFromRaw(map[string]any{
		"input_tokens":  100,
		"output_tokens": 20,
		"total_tokens":  120,
		"raw": map[string]any{
			"input_tokens":            100,
			"output_tokens":           20,
			"cache_read_input_tokens": 80,
			"total_tokens":            0,
		},
	})

	if !present || usage.ActualTokens() != 200 || !usage.ActualTokensAreEstimated() {
		t.Fatalf("nested provider usage = %#v, present = %v, want estimated actual 200", usage, present)
	}
}

func TestGoalUsageFromTokenUsageWithPresenceDistinguishesExplicitZeroFromMissing(t *testing.T) {
	explicitZero, present := GoalUsageFromTokenUsageWithPresence(sdkprotocol.TokenUsage{
		Raw: map[string]any{"total_tokens": 0},
	})
	if !present || !explicitZero.ActualTotalKnown || explicitZero.ActualTokens() != 0 {
		t.Fatalf("explicit zero = %#v, present = %v, want authoritative zero", explicitZero, present)
	}

	empty, present := GoalUsageFromTokenUsageWithPresence(sdkprotocol.TokenUsage{})
	if present || empty.ActualTokens() != 0 {
		t.Fatalf("empty usage = %#v, present = %v, want missing usage", empty, present)
	}
}

func TestGoalUsageFromTokenUsageWithPresenceRecognizesZeroBreakdown(t *testing.T) {
	usage, present := GoalUsageFromTokenUsageWithPresence(sdkprotocol.TokenUsage{
		Raw: map[string]any{
			"input_tokens":  0,
			"output_tokens": 0,
		},
	})
	if !present || usage.ActualTokens() != 0 {
		t.Fatalf("zero breakdown = %#v, present = %v, want observed zero usage", usage, present)
	}
}

func TestGoalUsageFromTokenUsageMarksBridgeFallbackAsEstimated(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:          100,
		OutputTokens:         20,
		CacheReadInputTokens: 80,
		ReasoningTokens:      10,
		TotalTokens:          210,
		Raw: map[string]any{
			"input_tokens":            100,
			"output_tokens":           20,
			"cache_read_input_tokens": 80,
			"reasoning_tokens":        10,
		},
	})

	if usage.ActualTokens() != 200 || !usage.ActualTokensAreEstimated() {
		t.Fatalf("actual usage = %#v, want estimated non-double-counted total 200", usage)
	}
	if usage.BudgetTokens() != 120 {
		t.Fatalf("budget usage = %#v, want 120", usage)
	}
}

func TestGoalUsageFromTokenUsageTreatsNXSModelUsageSyntheticTotalAsEstimated(t *testing.T) {
	usage, present := GoalUsageFromRaw(map[string]any{
		"input_tokens":  100,
		"output_tokens": 20,
		"total_tokens":  120,
		"raw": map[string]any{
			"input_tokens":                100,
			"output_tokens":               20,
			"cache_creation_input_tokens": 10,
			"cache_read_input_tokens":     80,
			"reasoning_tokens":            30,
		},
	})

	if !present || usage.ActualTokens() != 220 || !usage.ActualTokensAreEstimated() {
		t.Fatalf("nxs wrapper usage = %#v, present = %v, want estimated actual 220", usage, present)
	}
	if usage.BudgetTokens() != 120 ||
		usage.CacheCreationInputTokens != 10 ||
		usage.CacheReadInputTokens != 80 ||
		usage.ReasoningTokens != 30 {
		t.Fatalf("nxs wrapper breakdown = %#v, want nested cache/reasoning with budget 120", usage)
	}
}

func TestGoalUsageFromTokenUsagePrefersNestedProviderTotalInNXSModelUsage(t *testing.T) {
	usage := GoalUsageFromTokenUsage(sdkprotocol.TokenUsage{
		InputTokens:  100,
		OutputTokens: 20,
		TotalTokens:  120,
		Raw: map[string]any{
			"input_tokens":  100,
			"output_tokens": 20,
			"total_tokens":  120,
			"raw": map[string]any{
				"input_tokens":            100,
				"output_tokens":           20,
				"cache_read_input_tokens": 80,
				"total_tokens":            200,
			},
		},
	})

	if usage.ActualTokens() != 200 || usage.ActualTokensAreEstimated() {
		t.Fatalf("nxs wrapper usage = %#v, want exact nested provider total 200", usage)
	}
	if usage.CacheReadInputTokens != 80 {
		t.Fatalf("nxs wrapper cache read = %d, want 80", usage.CacheReadInputTokens)
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

func TestGoalUsageFromTokenUsageRecognizesTotalOnlyModelUsageIncludingZero(t *testing.T) {
	usage, present := GoalUsageFromTokenUsageWithPresence(sdkprotocol.TokenUsage{
		TotalTokens: 60,
		Raw: map[string]any{
			"model-a": map[string]any{"total_tokens": 0},
			"model-b": map[string]any{"total_tokens": 60},
		},
	})
	if !present || usage.ActualTokens() != 60 || usage.ActualTokensAreEstimated() {
		t.Fatalf("total-only model usage = %#v, present = %v, want exact 60", usage, present)
	}

	zero, present := GoalUsageFromTokenUsageWithPresence(sdkprotocol.TokenUsage{
		Raw: map[string]any{
			"model-a": map[string]any{"total_tokens": 0},
			"model-b": map[string]any{"total_tokens": 0},
		},
	})
	if !present || !zero.ActualTotalKnown || zero.ActualTokens() != 0 {
		t.Fatalf("total-only zero model usage = %#v, present = %v, want authoritative zero", zero, present)
	}
}
