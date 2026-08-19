package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

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
