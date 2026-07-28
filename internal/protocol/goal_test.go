package protocol

import (
	"encoding/json"
	"testing"
	"time"
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

func TestIsRuntimeGoalStatusOnlyAllowsActiveGoal(t *testing.T) {
	if !IsRuntimeGoalStatus(GoalStatusActive) {
		t.Fatal("active goal should provide runtime context")
	}
	for _, status := range []GoalStatus{
		GoalStatusPaused,
		GoalStatusBlocked,
		GoalStatusBudgetLimited,
		GoalStatusUsageLimited,
		GoalStatusComplete,
	} {
		if IsRuntimeGoalStatus(status) {
			t.Fatalf("status %q should not provide runtime context", status)
		}
	}
}

func TestIsRuntimeAccountingGoalStatusAllowsActiveAndBudgetLimitedGoals(t *testing.T) {
	for _, status := range []GoalStatus{GoalStatusActive, GoalStatusBudgetLimited} {
		if !IsRuntimeAccountingGoalStatus(status) {
			t.Fatalf("status %q should be a runtime accounting target", status)
		}
	}
	for _, status := range []GoalStatus{
		GoalStatusPaused,
		GoalStatusBlocked,
		GoalStatusUsageLimited,
		GoalStatusComplete,
	} {
		if IsRuntimeAccountingGoalStatus(status) {
			t.Fatalf("status %q should not be a runtime accounting target", status)
		}
	}
}

func TestIsGoalUsageFinalizableStatusOnlyAllowsComplete(t *testing.T) {
	if !IsGoalUsageFinalizableStatus(GoalStatusComplete) {
		t.Fatal("complete Goal should allow terminal usage finalization")
	}
	for _, status := range []GoalStatus{
		GoalStatusActive,
		GoalStatusPaused,
		GoalStatusBlocked,
		GoalStatusBudgetLimited,
		GoalStatusUsageLimited,
	} {
		if IsGoalUsageFinalizableStatus(status) {
			t.Fatalf("status %q should remain unfinalized", status)
		}
	}
}

func TestGoalUsageReportCarriesPersistentFinalizationFence(t *testing.T) {
	finalizedAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	report := (Goal{
		ID:               "goal-final",
		SessionKey:       "agent:nexus:ws:dm:final",
		Status:           GoalStatusComplete,
		Usage:            GoalUsage{InputTokens: 10, OutputTokens: 2, ActualTotalTokens: 30},
		TimeUsedSeconds:  4,
		UsageFinalized:   true,
		UsageFinalizedAt: &finalizedAt,
		UpdatedAt:        finalizedAt,
	}).UsageReport()

	if report.GoalID != "goal-final" || report.Usage.BudgetTokens() != 12 ||
		report.Usage.ActualTokens() != 30 || !report.UsageFinalized ||
		report.UsageFinalizedAt == nil || !report.UsageFinalizedAt.Equal(finalizedAt) {
		t.Fatalf("UsageReport() = %#v, want stable final usage projection", report)
	}
}

func TestGoalUsageReportJSONPreservesAuthoritativeZeroActual(t *testing.T) {
	report := (Goal{
		ID:         "goal-zero-actual",
		SessionKey: "agent:nexus:ws:dm:zero-actual",
		Status:     GoalStatusComplete,
		Usage: GoalUsage{
			InputTokens:       9,
			OutputTokens:      1,
			ActualTotalTokens: 0,
			ActualTotalKnown:  true,
		},
		UsageFinalized: true,
	}).UsageReport()

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	actual, actualPresent := decoded.Usage["actual_tokens"]
	budget, budgetPresent := decoded.Usage["budget_tokens"]
	if !actualPresent || actual != float64(0) {
		t.Fatalf("actual_tokens = %#v present=%v; JSON=%s, want authoritative zero", actual, actualPresent, payload)
	}
	if !budgetPresent || budget != float64(10) {
		t.Fatalf("budget_tokens = %#v present=%v; JSON=%s, want 10", budget, budgetPresent, payload)
	}
}

func TestGoalUsageSeparatesActualAndBudgetTokens(t *testing.T) {
	usage := GoalUsage{
		InputTokens:              900,
		OutputTokens:             80,
		CacheCreationInputTokens: 300,
		CacheReadInputTokens:     400,
		ReasoningTokens:          20,
		ActualTotalTokens:        1_300,
	}

	if got := usage.BudgetTokens(); got != 980 {
		t.Fatalf("BudgetTokens() = %d, want 980", got)
	}
	if got := usage.ActualTokens(); got != 1_300 {
		t.Fatalf("ActualTokens() = %d, want 1300", got)
	}
}

func TestGoalUsageBudgetTokensDoNotSubtractSeparatedCacheRead(t *testing.T) {
	usage := GoalUsage{
		InputTokens:          20,
		OutputTokens:         7,
		CacheReadInputTokens: 50,
		TotalTokens:          77,
	}

	if got := usage.BudgetTokens(); got != 27 {
		t.Fatalf("BudgetTokens() = %d, want 27", got)
	}
	if got := usage.ActualTokens(); got != 77 {
		t.Fatalf("ActualTokens() = %d, want 77", got)
	}
}

func TestGoalUsageAddAccumulatesActualAndBudgetTokens(t *testing.T) {
	first := GoalUsage{InputTokens: 100, OutputTokens: 20, CacheReadInputTokens: 90, ActualTotalTokens: 210}
	second := GoalUsage{InputTokens: 50, OutputTokens: 5, CacheReadInputTokens: 10, ReasoningTokens: 40, ActualTotalTokens: 105}

	got := first.Add(second)
	if got.TotalTokens != 175 || got.BudgetTotalTokens != 175 {
		t.Fatalf("budget totals = %#v, want 175", got)
	}
	if got.ActualTotalTokens != 315 {
		t.Fatalf("ActualTotalTokens = %d, want 315", got.ActualTotalTokens)
	}
	if got.InputTokens != 150 || got.OutputTokens != 25 || got.CacheReadInputTokens != 100 || got.ReasoningTokens != 40 {
		t.Fatalf("usage details = %#v, want accumulated details", got)
	}
}
