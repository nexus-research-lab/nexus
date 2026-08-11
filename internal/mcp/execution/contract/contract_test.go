package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func TestServerContextActorReadsGoalAuthorityDynamically(t *testing.T) {
	authority := runtimectx.NewGoalAuthorityState("", 0, "")
	serverContext := ServerContext{
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

func TestServerContextActorClonesTrustedWorkBinding(t *testing.T) {
	binding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	serverContext := ServerContext{
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
		t.Fatal("Actor mutation changed MCP ServerContext WorkBinding")
	}
}

func TestServerContextActorClonesTrustedReviewBinding(t *testing.T) {
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
	serverContext := ServerContext{
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
		t.Fatal("Actor mutation changed MCP ServerContext ReviewBinding")
	}
}
