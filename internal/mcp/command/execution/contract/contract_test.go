package contract

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

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
