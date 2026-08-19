package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestContextActorReadsUnifiedResponsibilityAtomically(t *testing.T) {
	goal := runtimectx.NewGoalAuthorityState("goal-1", 1, "execution-old")
	review := &protocol.ExecutionReviewBinding{
		ExecutionID: "execution-old", PlanID: "plan-old", WorkItemID: "work-old",
		SpecID: "spec-old", AssignmentID: "assignment-old", SubmissionID: "submission-old",
		ReviewDispatchID: "review-dispatch-old", TargetAgentID: "agent-lead",
	}
	authority := runtimectx.NewResponsibilityAuthorityState(
		goal, "execution-old", nil, review,
	)
	sctx := Context{
		OwnerUserID: "owner-1", AgentID: "agent-lead",
		ScopeKind: protocol.ExecutionScopeRoom, ScopeSessionKey: "room:group:topic-1",
		ExecutionID: "execution-old", ReviewBinding: review, GoalAuthority: goal,
		ResponsibilityAuthority: authority,
	}
	actor := sctx.Actor()
	if actor.ExecutionID != "execution-old" || actor.ReviewBinding == nil {
		t.Fatalf("initial actor = %#v, want predecessor review lane", actor)
	}

	retargeted := protocol.Goal{ID: "goal-1", Metadata: map[string]any{
		protocol.GoalMetadataObjectiveRevision:     int64(2),
		protocol.GoalMetadataExecutionMode:         string(protocol.GoalExecutionModeManaged),
		protocol.GoalMetadataExecutionBindingState: string(protocol.GoalExecutionBindingStateReserved),
		protocol.GoalMetadataExecutionID:           "execution-successor",
	}}
	if !authority.ApplyGoalMutation(retargeted) {
		t.Fatal("retarget authority transition failed")
	}
	actor = sctx.Actor()
	if actor.ExecutionID != "" || actor.ReviewBinding != nil ||
		actor.GoalID != "goal-1" || actor.GoalObjectiveRevision != 2 {
		t.Fatalf("retargeted actor = %#v, want planning authority without stale review", actor)
	}

	if !authority.BindCoordination("execution-successor") {
		t.Fatal("successor coordination receipt failed")
	}
	actor = sctx.Actor()
	if actor.ExecutionID != "execution-successor" || actor.ReviewBinding != nil ||
		actor.WorkBinding != nil || actor.GoalObjectiveRevision != 2 {
		t.Fatalf("successor actor = %#v, want exact successor coordination", actor)
	}
}
