package protocol

import (
	"encoding/json"
	"testing"
)

func TestUpdateGoalRequestTokenBudgetTriState(t *testing.T) {
	var missing UpdateGoalRequest
	if err := json.Unmarshal([]byte(`{}`), &missing); err != nil {
		t.Fatalf("unmarshal missing token_budget: %v", err)
	}
	if missing.TokenBudget.Present {
		t.Fatalf("missing token_budget should not be present: %+v", missing.TokenBudget)
	}

	var cleared UpdateGoalRequest
	if err := json.Unmarshal([]byte(`{"token_budget":null}`), &cleared); err != nil {
		t.Fatalf("unmarshal null token_budget: %v", err)
	}
	if !cleared.TokenBudget.Present || cleared.TokenBudget.Value != nil {
		t.Fatalf("null token_budget = %+v, want present nil", cleared.TokenBudget)
	}

	var updated UpdateGoalRequest
	if err := json.Unmarshal([]byte(`{"token_budget":1200}`), &updated); err != nil {
		t.Fatalf("unmarshal numeric token_budget: %v", err)
	}
	if !updated.TokenBudget.Present || updated.TokenBudget.Value == nil || *updated.TokenBudget.Value != 1200 {
		t.Fatalf("numeric token_budget = %+v, want present 1200", updated.TokenBudget)
	}
}

func TestGoalUsageBudgetTokensExcludeCachedAndReasoningTokens(t *testing.T) {
	usage := GoalUsage{
		InputTokens:              100,
		OutputTokens:             20,
		CacheCreationInputTokens: 30,
		CacheReadInputTokens:     90,
		ReasoningTokens:          50,
		TotalTokens:              290,
	}

	if got := usage.BudgetTokens(); got != 120 {
		t.Fatalf("BudgetTokens() = %d, want 120", got)
	}
	if got := usage.Total(); got != 120 {
		t.Fatalf("Total() = %d, want 120", got)
	}
}

func TestGoalUsageBudgetTokensDoNotSubtractCacheRead(t *testing.T) {
	usage := GoalUsage{
		InputTokens:          20,
		OutputTokens:         7,
		CacheReadInputTokens: 50,
		TotalTokens:          77,
	}

	if got := usage.BudgetTokens(); got != 27 {
		t.Fatalf("BudgetTokens() = %d, want 27", got)
	}
}

func TestGoalUsageAddAccumulatesBudgetTokens(t *testing.T) {
	first := GoalUsage{InputTokens: 100, OutputTokens: 20, CacheReadInputTokens: 90, TotalTokens: 210}
	second := GoalUsage{InputTokens: 50, OutputTokens: 5, CacheReadInputTokens: 10, ReasoningTokens: 40, TotalTokens: 105}

	got := first.Add(second)
	if got.TotalTokens != 175 {
		t.Fatalf("TotalTokens = %d, want 175", got.TotalTokens)
	}
	if got.InputTokens != 150 || got.OutputTokens != 25 || got.CacheReadInputTokens != 100 || got.ReasoningTokens != 40 {
		t.Fatalf("usage details = %#v, want accumulated details", got)
	}
}
