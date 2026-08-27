package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestContextActorReadsGoalAuthorityDynamically(t *testing.T) {
	authority := runtimectx.NewGoalAuthorityState("", 0, "")
	serverContext := Context{
		OwnerUserID:     "owner-1",
		AgentID:         "agent-1",
		ScopeSessionKey: "agent:agent-1:ws:dm:conversation-1",
		GoalAuthority:   authority,
	}
	if actor := serverContext.Actor(); actor.GoalID != "" || actor.GoalObjectiveRevision != 0 {
		t.Fatalf("ordinary round actor Goal authority = %+v", actor)
	}
	if !authority.Bind("goal-1", 1, "") {
		t.Fatal("bind Goal authority")
	}
	actor := serverContext.Actor()
	if actor.GoalID != "goal-1" || actor.GoalObjectiveRevision != 1 {
		t.Fatalf("Actor did not observe create_goal authority: %+v", actor)
	}
	if !authority.Bind("goal-1", 2, "execution-2") {
		t.Fatal("advance Goal authority")
	}
	actor = serverContext.Actor()
	if actor.GoalID != "goal-1" || actor.GoalObjectiveRevision != 2 ||
		actor.ExecutionID != "execution-2" {
		t.Fatalf("Actor did not observe retargeted authority: %+v", actor)
	}
}

func TestContextActorClonesTrustedWorkBinding(t *testing.T) {
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	serverContext := Context{
		OwnerUserID:     "owner-1",
		AgentID:         "agent-member",
		ScopeSessionKey: "room:group:conversation-1",
		WorkBinding:     binding,
	}

	actor := serverContext.Actor()
	if actor.WorkBinding == nil ||
		actor.WorkBinding.AssignmentID != "assignment-1" ||
		actor.WorkBinding == binding {
		t.Fatalf("actor WorkBinding = %#v", actor.WorkBinding)
	}
	actor.WorkBinding.AssignmentID = "assignment-mutated"
	if serverContext.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatal("Actor mutation changed command Context WorkBinding")
	}
}

func TestContextActorReadsRoomWorkBindingDynamically(t *testing.T) {
	state := runtimectx.NewWorkBindingState(nil)
	serverContext := Context{
		OwnerUserID: "owner-1", AgentID: "agent-lead", ScopeKind: protocol.ExecutionScopeRoom,
		ScopeSessionKey: "room:group:conversation-1", ExecutionID: "execution-1",
		WorkBindingState: state,
	}
	if actor := serverContext.Actor(); actor.WorkBinding != nil {
		t.Fatalf("unbound Room actor = %+v", actor)
	}
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	if !state.Bind(binding) {
		t.Fatal("bind host receipt")
	}
	actor := serverContext.Actor()
	if actor.WorkBinding == nil || actor.WorkBinding.AssignmentID != "assignment-1" ||
		actor.WorkBinding == binding {
		t.Fatalf("dynamic Room actor = %+v", actor)
	}
	state.Clear()
	if actor = serverContext.Actor(); actor.WorkBinding != nil {
		t.Fatalf("cleared Room actor = %+v", actor)
	}
}

func TestContextActorClonesTrustedReviewBinding(t *testing.T) {
	binding := &protocol.ExecutionReviewBinding{
		ExecutionID:      "execution-1",
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "agent-lead",
	}
	serverContext := Context{
		OwnerUserID:     "owner-1",
		AgentID:         "agent-lead",
		ScopeSessionKey: "room:group:conversation-1",
		ReviewBinding:   binding,
	}
	actor := serverContext.Actor()
	if actor.ReviewBinding == nil ||
		actor.ReviewBinding.SubmissionID != "submission-1" ||
		actor.ReviewBinding == binding {
		t.Fatalf("actor ReviewBinding = %#v", actor.ReviewBinding)
	}
	actor.ReviewBinding.SubmissionID = "submission-mutated"
	if serverContext.ReviewBinding.SubmissionID != "submission-1" {
		t.Fatal("Actor mutation changed command Context ReviewBinding")
	}
}

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
