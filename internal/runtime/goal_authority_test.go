package runtime

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestGoalAuthorityStateRequiresGoalAndRevisionButAllowsEmptyExecution(t *testing.T) {
	t.Parallel()

	empty := NewGoalAuthorityState("", 0, "execution-ignored")
	if authority, ok := empty.Load(); ok || authority != (GoalAuthority{}) {
		t.Fatalf("empty authority = %#v, ok=%t", authority, ok)
	}
	state := NewGoalAuthorityState(" goal-1 ", 2, "")
	authority, ok := state.Load()
	if !ok || authority.GoalID != "goal-1" || authority.ObjectiveRevision != 2 || authority.ExecutionID != "" {
		t.Fatalf("Goal-only authority = %#v, ok=%t", authority, ok)
	}
}

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

func TestGoalAuthorityStateOnlyUpgradesExecutionWithinSameRevision(t *testing.T) {
	t.Parallel()

	state := NewGoalAuthorityState("goal-1", 2, "")
	if !state.Bind("goal-1", 2, "execution-1") {
		t.Fatal("an unbound authority should accept one confirmed Execution fence")
	}
	if state.Bind("goal-1", 2, "") {
		t.Fatal("a confirmed Execution fence must not be removed in the same revision")
	}
	if state.Bind("goal-1", 2, "execution-other") {
		t.Fatal("a confirmed Execution fence must not be replaced in the same revision")
	}
	if !state.Bind("goal-1", 3, "execution-next") {
		t.Fatal("a newer objective revision may adopt its successor Execution fence")
	}
}

func TestGoalAuthorityStateSharesRevisionAndContext(t *testing.T) {
	t.Parallel()

	revision := &atomic.Int64{}
	revision.Store(4)
	state := NewGoalAuthorityStateWithRevision("goal-1", "execution-1", revision)
	ctx := WithGoalAuthorityState(context.Background(), state)
	if GoalAuthorityStateFromContext(ctx) != state {
		t.Fatal("context did not preserve the exact authority state pointer")
	}
	revision.Store(5)
	authority, ok := state.Load()
	if !ok || authority.ObjectiveRevision != 5 {
		t.Fatalf("shared revision = %#v, ok=%t", authority, ok)
	}
}
