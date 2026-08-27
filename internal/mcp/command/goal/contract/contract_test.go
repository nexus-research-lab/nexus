package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestStoreGoalMutationAuthoritySeparatesReservationFromConfirmedBinding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		bindingState  protocol.GoalExecutionBindingState
		wantGranted   bool
		wantExecution string
	}{
		{name: "standalone", bindingState: protocol.GoalExecutionBindingStateStandalone, wantGranted: true},
		{name: "reserved", bindingState: protocol.GoalExecutionBindingStateReserved, wantGranted: true},
		{name: "confirmed", bindingState: protocol.GoalExecutionBindingStateConfirmed, wantGranted: true, wantExecution: "execution-1"},
		{name: "pending", bindingState: protocol.GoalExecutionBindingStatePending},
		{name: "conflict", bindingState: protocol.GoalExecutionBindingStateConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			state := runtimectx.NewGoalAuthorityState("", 0, "")
			sctx := Context{GoalAuthority: state}
			metadata := map[string]any{
				protocol.GoalMetadataObjectiveRevision:     int64(1),
				protocol.GoalMetadataExecutionID:           "execution-1",
				protocol.GoalMetadataExecutionBindingState: string(test.bindingState),
			}
			if test.bindingState == protocol.GoalExecutionBindingStateStandalone {
				delete(metadata, protocol.GoalMetadataExecutionBindingState)
			}
			sctx.StoreGoalMutationAuthority(protocol.Goal{ID: "goal-1", Metadata: metadata})
			authority, ok := state.Load()
			if ok != test.wantGranted {
				t.Fatalf("authority = %#v, ok=%t, want granted=%t", authority, ok, test.wantGranted)
			}
			if ok && authority.ExecutionID != test.wantExecution {
				t.Fatalf("ExecutionID = %q, want %q", authority.ExecutionID, test.wantExecution)
			}
		})
	}
}

func TestStoreGoalMutationAuthorityAtomicallyRevokesPredecessorReview(t *testing.T) {
	goalState := runtimectx.NewGoalAuthorityState("goal-1", 1, "execution-old")
	responsibility := runtimectx.NewResponsibilityAuthorityState(
		goalState,
		"execution-old",
		nil,
		&protocol.ExecutionReviewBinding{
			ExecutionID: "execution-old", PlanID: "plan-old", WorkItemID: "work-old",
			SpecID: "spec-old", AssignmentID: "assignment-old", SubmissionID: "submission-old",
			ReviewDispatchID: "review-dispatch-old", TargetAgentID: "agent-lead",
		},
	)
	sctx := Context{
		GoalAuthority: goalState, ResponsibilityAuthority: responsibility,
	}
	sctx.StoreGoalMutationAuthority(protocol.Goal{ID: "goal-1", Metadata: map[string]any{
		protocol.GoalMetadataObjectiveRevision:     int64(2),
		protocol.GoalMetadataExecutionMode:         string(protocol.GoalExecutionModeManaged),
		protocol.GoalMetadataExecutionBindingState: string(protocol.GoalExecutionBindingStateReserved),
		protocol.GoalMetadataExecutionID:           "execution-successor",
	}})
	authority, _ := responsibility.Load()
	if authority.ExecutionID != "" ||
		authority.ReservedExecutionID != "execution-successor" ||
		authority.ReviewBinding != nil || authority.Lane != runtimectx.ResponsibilityLanePlanning {
		t.Fatalf("authority = %#v, want exact successor reservation and revoked review", authority)
	}
	goalAuthority, ok := goalState.LoadBound()
	if !ok || goalAuthority.GoalID != "goal-1" ||
		goalAuthority.ObjectiveRevision != 2 || goalAuthority.ExecutionID != "" {
		t.Fatalf("goal authority = %#v, ok=%t", goalAuthority, ok)
	}
}
