package protocol

import (
	"encoding/json"
	"testing"
)

func TestGoalJSONProjectsServerDerivedContinuationState(t *testing.T) {
	tests := []struct {
		name  string
		goal  Goal
		state GoalContinuationState
	}{
		{name: "active is ready", goal: Goal{Status: GoalStatusActive}, state: GoalContinuationStateReady},
		{name: "first empty turn is recovering", goal: Goal{Status: GoalStatusActive, EmptyProgressCount: 1}, state: GoalContinuationStateRecovering},
		{name: "second empty turn is suspended", goal: Goal{Status: GoalStatusActive, EmptyProgressCount: GoalContinuationSuppressionThreshold}, state: GoalContinuationStateSuspended},
		{name: "runtime error is suspended", goal: Goal{Status: GoalStatusActive, LastError: "runtime failed"}, state: GoalContinuationStateSuspended},
		{name: "paused lifecycle is inactive", goal: Goal{Status: GoalStatusPaused}, state: GoalContinuationStateInactive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.goal.ContinuationState(); got != test.state {
				t.Fatalf("continuation state = %q, want %q", got, test.state)
			}
			data, err := json.Marshal(test.goal)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if got := wire["continuation_state"]; got != string(test.state) {
				t.Fatalf("wire continuation_state = %v, want %q; payload=%s", got, test.state, data)
			}
		})
	}
}
