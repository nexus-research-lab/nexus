package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
