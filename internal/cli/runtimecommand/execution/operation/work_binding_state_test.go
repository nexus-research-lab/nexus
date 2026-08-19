// INPUT: host-issued WorkBinding receipts and Room/DM execution scopes
// OUTPUT: regression coverage for runtime WorkBinding bind and clear transitions
// POS: Execution command tests for host-owned per-assignment round identity

package operation

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestAssignWorkBindsOnlyConfirmedRoomSelfReceipt(t *testing.T) {
	snapshot := executionSnapshot(9)
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID: snapshot.Execution.ID, PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	for _, test := range []struct {
		name      string
		scope     protocol.ExecutionScopeKind
		receipt   bool
		state     bool
		wantBound bool
	}{
		{name: "Room host receipt", scope: protocol.ExecutionScopeRoom, receipt: true, state: true, wantBound: true},
		{name: "DM ignores receipt", scope: protocol.ExecutionScopeDM, receipt: true, state: true},
		{name: "Room requires receipt", scope: protocol.ExecutionScopeRoom, state: true},
		{name: "Room requires host state", scope: protocol.ExecutionScopeRoom, receipt: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := orchestration.AppliedResult(snapshot, nil, nil)
			if test.receipt {
				result.WorkBinding = &orchestration.WorkBindingReceipt{Binding: binding}
			}
			sctx := contract.Context{ScopeKind: test.scope}
			if test.state {
				sctx.WorkBindingState = runtimectx.NewWorkBindingState(nil)
			}
			if got := bindMutationWorkBinding(sctx, result); got != test.wantBound {
				t.Fatalf("bind = %t, want %t", got, test.wantBound)
			}
			if sctx.WorkBindingState != nil {
				loaded, ok := sctx.WorkBindingState.Load()
				if ok != test.wantBound || (ok && loaded.AssignmentID != binding.AssignmentID) {
					t.Fatalf("state = %#v, %t", loaded, ok)
				}
			}
		})
	}
}

func TestReviewWorkClearReceiptReturnsRoomRoundToCoordination(t *testing.T) {
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	state := runtimectx.NewWorkBindingState(binding)
	result := orchestration.MutationResult{
		Outcome:     orchestration.MutationApplied,
		WorkBinding: &orchestration.WorkBindingReceipt{Clear: true},
	}
	if !applyMutationWorkBindingTransition(contract.Context{WorkBindingState: state}, result) {
		t.Fatal("clear receipt was not applied")
	}
	if loaded, ok := state.Load(); ok || loaded != nil {
		t.Fatalf("cleared state = %#v, %t", loaded, ok)
	}
}
