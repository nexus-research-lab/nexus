// execution_and_queue_test.go 覆盖执行输出作用域、Goal 与输入队列协议测试。
package protocol

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeWorkOutputScopeRejectsInvalidGrammar(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		mode  WorkOutputScopeMode
	}{
		{name: "empty", scope: ""},
		{name: "untyped", scope: "reports/final"},
		{name: "unknown kind", scope: "resource:reports/final"},
		{name: "empty file", scope: "file:"},
		{name: "empty directory", scope: "dir:  "},
		{name: "empty semantic", scope: "semantic:  "},
		{name: "absolute", scope: "file:/etc/passwd"},
		{name: "windows absolute", scope: "dir:C:/workspace"},
		{name: "dot", scope: "dir:."},
		{name: "parent", scope: "file:../outside"},
		{name: "cleaned parent escape", scope: "dir:web/../../outside"},
		{name: "backslash", scope: `file:web\main.go`},
		{name: "path control", scope: "file:web/\nmain.go"},
		{name: "semantic control", scope: "semantic:report\nfinal"},
		{name: "invalid mode", scope: "semantic:report", mode: "private"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeWorkOutputScope(WorkOutputScope{
				Scope: test.scope,
				Mode:  test.mode,
			})
			if !errors.Is(err, ErrInvalidWorkOutputScope) {
				t.Fatalf("error = %v, want ErrInvalidWorkOutputScope", err)
			}
		})
	}
}

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
