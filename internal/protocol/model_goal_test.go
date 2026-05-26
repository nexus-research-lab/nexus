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

func TestThreadGoalSetParamsUseCodexCamelCase(t *testing.T) {
	var params ThreadGoalSetParams
	if err := json.Unmarshal([]byte(`{"threadId":"agent:nexus:ws:dm:chat","status":"usageLimited","tokenBudget":null}`), &params); err != nil {
		t.Fatalf("unmarshal thread goal params: %v", err)
	}
	if params.ThreadID != "agent:nexus:ws:dm:chat" {
		t.Fatalf("ThreadID = %q, want camelCase threadId", params.ThreadID)
	}
	if params.Status == nil || *params.Status != ThreadGoalStatusUsageLimited {
		t.Fatalf("Status = %#v, want usageLimited", params.Status)
	}
	if !params.TokenBudget.Present || params.TokenBudget.Value != nil {
		t.Fatalf("TokenBudget = %+v, want present null", params.TokenBudget)
	}
}

func TestAppServerRPCRequestIDPreservesStringAndInteger(t *testing.T) {
	for _, input := range []string{
		`{"id":7,"method":"thread/goal/get"}`,
		`{"id":"goal-get","method":"thread/goal/get"}`,
	} {
		var request AppServerJSONRPCRequest
		if err := json.Unmarshal([]byte(input), &request); err != nil {
			t.Fatalf("unmarshal %s: %v", input, err)
		}
		output, err := json.Marshal(AppServerJSONRPCResponse{ID: request.ID, Result: map[string]any{"ok": true}})
		if err != nil {
			t.Fatalf("marshal response: %v", err)
		}
		var roundtrip map[string]any
		if err := json.Unmarshal(output, &roundtrip); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if _, ok := roundtrip["id"]; !ok {
			t.Fatalf("response missing id: %s", string(output))
		}
	}

	var invalid AppServerJSONRPCRequest
	if err := json.Unmarshal([]byte(`{"id":1.5,"method":"thread/goal/get"}`), &invalid); err == nil {
		t.Fatal("fractional request id should be rejected")
	}
}

func TestThreadGoalFromGoalUsesCodexProjection(t *testing.T) {
	budget := int64(100)
	item := Goal{
		SessionKey:      "agent:nexus:ws:dm:chat",
		Objective:       "Ship parity",
		Status:          GoalStatusBudgetLimited,
		TokenBudget:     &budget,
		Usage:           GoalUsage{InputTokens: 20, OutputTokens: 5, TotalTokens: 25},
		TimeUsedSeconds: 7,
	}

	projected := ThreadGoalFromGoal(item)
	if projected.ThreadID != item.SessionKey ||
		projected.Status != ThreadGoalStatusBudgetLimited ||
		projected.TokenBudget == nil ||
		*projected.TokenBudget != budget ||
		projected.TokensUsed != 25 ||
		projected.TimeUsedSeconds != 7 {
		t.Fatalf("ThreadGoalFromGoal() = %#v", projected)
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
		GoalStatusCleared,
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
		GoalStatusCleared,
	} {
		if IsRuntimeAccountingGoalStatus(status) {
			t.Fatalf("status %q should not be a runtime accounting target", status)
		}
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
