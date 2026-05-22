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
