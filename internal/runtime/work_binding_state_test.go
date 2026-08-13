package runtime

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestWorkBindingStateRequiresExplicitClearBeforeResponsibilitySwitch(t *testing.T) {
	first := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	state := NewWorkBindingState(nil)
	if !state.Bind(first) || !state.Bind(first) {
		t.Fatal("exact host binding must bind and replay idempotently")
	}
	loaded, ok := state.Load()
	if !ok || loaded == first || loaded.AssignmentID != "assignment-1" {
		t.Fatalf("Load() = %#v, %t", loaded, ok)
	}
	second := *first
	second.WorkItemID = "work-2"
	second.SpecID = "spec-2"
	second.AssignmentID = "assignment-2"
	second.AttemptID = "attempt-2"
	if state.Bind(&second) {
		t.Fatal("unreleased responsibility switched WorkBinding")
	}
	state.Clear()
	if !state.Bind(&second) {
		t.Fatal("explicitly cleared responsibility did not accept next binding")
	}
}

func TestWorkBindingStateRejectsIncompleteReceipt(t *testing.T) {
	state := NewWorkBindingState(nil)
	if state.Bind(&protocol.ExecutionWorkBinding{ExecutionID: "execution-1"}) {
		t.Fatal("incomplete receipt minted WorkBinding")
	}
	if binding, ok := state.Load(); ok || binding != nil {
		t.Fatalf("incomplete state Load() = %#v, %t", binding, ok)
	}
}
