package runtime

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestResponsibilityAuthorityRetargetRevokesReviewAndBindsSuccessor(t *testing.T) {
	goal := NewGoalAuthorityState("goal-1", 1, "execution-old")
	review := responsibilityTestReviewBinding("execution-old")
	state := NewResponsibilityAuthorityState(goal, "execution-old", nil, review)

	before, _ := state.Load()
	if before.Lane != ResponsibilityLaneReview || before.ReviewBinding == nil {
		t.Fatalf("before = %#v, want exact review lane", before)
	}

	retargeted := protocol.Goal{
		ID: "goal-1",
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision:     int64(2),
			protocol.GoalMetadataExecutionMode:         string(protocol.GoalExecutionModeManaged),
			protocol.GoalMetadataExecutionBindingState: string(protocol.GoalExecutionBindingStateReserved),
			protocol.GoalMetadataExecutionID:           "execution-successor",
		},
	}
	if !state.ApplyGoalMutation(retargeted) {
		t.Fatal("ApplyGoalMutation(retargeted) = false")
	}
	afterRetarget, _ := state.Load()
	if afterRetarget.ExecutionID != "" ||
		afterRetarget.ReservedExecutionID != "execution-successor" ||
		afterRetarget.ReviewBinding != nil ||
		afterRetarget.WorkBinding != nil ||
		afterRetarget.Lane != ResponsibilityLanePlanning {
		t.Fatalf("after retarget = %#v, want successor planning with old lanes revoked", afterRetarget)
	}
	if state.BindCoordination("execution-old") {
		t.Fatal("old execution receipt rebound predecessor after retarget")
	}
	if !state.BindCoordination("execution-successor") {
		t.Fatal("successor materialization receipt was rejected")
	}
	afterPlan, _ := state.Load()
	if afterPlan.ExecutionID != "execution-successor" ||
		afterPlan.ReservedExecutionID != "" ||
		afterPlan.Lane != ResponsibilityLaneCoordination ||
		afterPlan.ReviewBinding != nil {
		t.Fatalf("after plan = %#v, want successor coordination", afterPlan)
	}
	if !state.BindCoordination("execution-successor") {
		t.Fatal("replayed successor receipt was rejected")
	}
	replayed, _ := state.Load()
	if replayed.Generation != afterPlan.Generation {
		t.Fatalf("replayed receipt advanced generation: before=%d after=%d", afterPlan.Generation, replayed.Generation)
	}
	if state.RevokeExecution("execution-old") {
		t.Fatal("late predecessor terminal receipt revoked successor")
	}
}

func TestResponsibilityAuthorityKeepsExactWorkLaneAcrossGoalConfirmation(t *testing.T) {
	goal := NewGoalAuthorityState("", 0, "")
	work := responsibilityTestWorkBinding("execution-1", "assignment-1")
	state := NewResponsibilityAuthorityState(goal, "execution-1", work, nil)

	if !state.ConfirmGoalExecution("goal-1", 1, "execution-1") {
		t.Fatal("ConfirmGoalExecution() = false")
	}
	authority, _ := state.Load()
	if authority.Lane != ResponsibilityLaneWork || authority.WorkBinding == nil ||
		authority.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatalf("authority = %#v, Goal confirmation must preserve exact WorkBinding", authority)
	}
	if state.BindWork(responsibilityTestWorkBinding("execution-1", "assignment-sibling")) {
		t.Fatal("sibling WorkBinding replaced exact responsibility")
	}
	if state.BindWork(responsibilityTestWorkBinding("execution-other", "assignment-1")) {
		t.Fatal("cross-Execution WorkBinding replaced exact responsibility")
	}
}

func TestWorkBindingStateSharesResponsibilityAuthority(t *testing.T) {
	state := NewResponsibilityAuthorityState(nil, "execution-1", nil, nil)
	workState := NewWorkBindingStateFromResponsibility(state)
	binding := responsibilityTestWorkBinding("execution-1", "assignment-1")
	if !workState.Bind(binding) {
		t.Fatal("Bind() = false")
	}
	authority, _ := state.Load()
	if authority.Lane != ResponsibilityLaneWork || authority.WorkBinding == nil {
		t.Fatalf("authority = %#v, want work lane", authority)
	}
	workState.Clear()
	authority, _ = state.Load()
	if authority.Lane != ResponsibilityLaneCoordination || authority.WorkBinding != nil {
		t.Fatalf("authority = %#v, want coordination after clear", authority)
	}
}

func TestResponsibilityAuthorityTerminalReleaseRejectsStaticExecutionReseed(t *testing.T) {
	state := NewResponsibilityAuthorityState(nil, "execution-old", nil, nil)
	if !state.RevokeExecution("execution-old") {
		t.Fatal("RevokeExecution() = false")
	}
	if state.SeedExecution("execution-old") {
		t.Fatal("stale static actor reseeded terminal execution")
	}
	if state.BindCoordination("execution-old") {
		t.Fatal("late coordination receipt rebound terminal execution")
	}
	authority, _ := state.Load()
	if authority.ExecutionID != "" || authority.Lane != ResponsibilityLaneUnbound {
		t.Fatalf("authority = %#v, want released execution to remain unbound", authority)
	}
}

func TestResponsibilityAuthorityConflictingInitialBindingsFailClosed(t *testing.T) {
	state := NewResponsibilityAuthorityState(
		NewGoalAuthorityState("goal-1", 1, "execution-1"),
		"execution-1",
		responsibilityTestWorkBinding("execution-1", "assignment-1"),
		responsibilityTestReviewBinding("execution-1"),
	)
	authority, _ := state.Load()
	if authority.ExecutionID != "" || authority.WorkBinding != nil ||
		authority.ReviewBinding != nil || authority.GoalID != "" ||
		authority.Lane != ResponsibilityLaneUnbound {
		t.Fatalf("conflicting authority = %#v, want fully unbound", authority)
	}
	if state.SeedExecution("execution-1") ||
		state.GrantGoalAuthority("goal-1", 1, "execution-1") {
		t.Fatal("invalid initial responsibility was upgraded by a later static seed")
	}
}

func TestResponsibilityAuthorityIncompleteInitialBindingFailsClosed(t *testing.T) {
	state := NewResponsibilityAuthorityState(
		NewGoalAuthorityState("goal-1", 1, "execution-1"),
		"execution-1",
		&protocol.ExecutionWorkBinding{ExecutionID: "execution-1"},
		nil,
	)
	authority, _ := state.Load()
	if authority.ExecutionID != "" || authority.GoalID != "" ||
		authority.Lane != ResponsibilityLaneUnbound ||
		state.SeedExecution("execution-1") {
		t.Fatalf("incomplete authority = %#v, want permanent fail-close", authority)
	}
}

func responsibilityTestWorkBinding(
	executionID string,
	assignmentID string,
) *protocol.ExecutionWorkBinding {
	return &protocol.ExecutionWorkBinding{
		ExecutionID:  executionID,
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: assignmentID,
		AttemptID:    "attempt-1",
	}
}

func responsibilityTestReviewBinding(executionID string) *protocol.ExecutionReviewBinding {
	return &protocol.ExecutionReviewBinding{
		ExecutionID:      executionID,
		PlanID:           "plan-1",
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		SubmissionID:     "submission-1",
		ReviewDispatchID: "review-dispatch-1",
		TargetAgentID:    "agent-lead",
	}
}
