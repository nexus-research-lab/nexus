package server

import (
	"context"
	"errors"
	"testing"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type stubExecutionCurrentReader struct {
	snapshot         *protocol.ExecutionSnapshot
	err              error
	actor            orchestrationsvc.ActorContext
	requestedID      string
	getCurrentCalls  int
	getSnapshotCalls int
}

func (r *stubExecutionCurrentReader) GetCurrent(
	_ context.Context,
	actor orchestrationsvc.ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	r.actor = actor
	r.getCurrentCalls++
	return r.snapshot, r.err
}

func (r *stubExecutionCurrentReader) GetSnapshot(
	_ context.Context,
	actor orchestrationsvc.ActorContext,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	r.actor = actor
	r.requestedID = executionID
	r.getSnapshotCalls++
	return r.snapshot, r.err
}

func TestResolveExecutionMCPServerContextMapsDMIdentity(t *testing.T) {
	reader := &stubExecutionCurrentReader{}
	goalAuthority := runtimectx.NewGoalAuthorityState("", 0, "")
	serverContext, ok := resolveExecutionMCPServerContext(
		context.Background(),
		reader,
		runtimectx.ExecutionToolContext{
			Agent: &protocol.Agent{
				AgentID:     " agent-lead ",
				OwnerUserID: " owner-1 ",
			},
			ScopeSessionKey:   " agent:agent-lead:ws:dm:conversation-1 ",
			RuntimeSessionKey: " runtime-session-1 ",
			RootRoundID:       " root-round-1 ",
			AgentRoundID:      " agent-round-1 ",
			SourceContextType: "agent",
			SourceContextID:   "agent-lead",
			PermissionMode:    sdkpermission.ModePlan,
			GoalAuthority:     goalAuthority,
		},
	)
	if !ok {
		t.Fatal("DM runtime identity should produce an Execution MCP context")
	}
	if serverContext.OwnerUserID != "owner-1" ||
		serverContext.AgentID != "agent-lead" ||
		serverContext.ScopeKind != protocol.ExecutionScopeDM ||
		serverContext.Role != orchestrationsvc.ExecutionActorCoordinator ||
		serverContext.ScopeSessionKey != "agent:agent-lead:ws:dm:conversation-1" ||
		serverContext.RuntimeSessionKey != "runtime-session-1" ||
		serverContext.RootRoundID != "root-round-1" ||
		serverContext.RuntimeRoundID != "root-round-1" ||
		serverContext.AgentRoundID != "agent-round-1" ||
		!serverContext.PlanMode {
		t.Fatalf("DM Execution server context = %+v", serverContext)
	}
	if reader.actor.AgentID != "" {
		t.Fatalf("DM role should not require a snapshot lookup, got actor %+v", reader.actor)
	}
	if !goalAuthority.Bind("goal-created-in-round", 1, "") {
		t.Fatal("bind same-round Goal authority")
	}
	actor := serverContext.Actor()
	if actor.GoalID != "goal-created-in-round" || actor.GoalObjectiveRevision != 1 {
		t.Fatalf("Execution MCP actor did not observe dynamic Goal authority: %+v", actor)
	}
}

func TestResolveExecutionMCPServerContextDerivesRoomRoleFromExecution(t *testing.T) {
	workBinding := &protocol.ExecutionWorkBinding{
		ExecutionID:  "execution-1",
		PlanID:       "plan-1",
		WorkItemID:   "work-1",
		SpecID:       "spec-1",
		AssignmentID: "assignment-1",
		AttemptID:    "attempt-1",
		DispatchID:   "dispatch-1",
	}
	reader := &stubExecutionCurrentReader{
		snapshot: &protocol.ExecutionSnapshot{Execution: protocol.Execution{
			ID:                 "execution-1",
			OwnerUserID:        "owner-1",
			SessionKey:         "room:group:conversation-1",
			ScopeKind:          protocol.ExecutionScopeRoom,
			RoomID:             "room-1",
			ConversationID:     "conversation-1",
			CoordinatorAgentID: "agent-lead",
		}},
	}
	serverContext, ok := resolveExecutionMCPServerContext(
		context.Background(),
		reader,
		runtimectx.ExecutionToolContext{
			Agent: &protocol.Agent{
				AgentID:     "agent-member",
				OwnerUserID: "owner-1",
			},
			ScopeSessionKey:    "room:group:conversation-1",
			RuntimeSessionKey:  "agent:agent-member:ws:group:conversation-1",
			ExecutionID:        "execution-1",
			WorkBinding:        workBinding,
			CoordinatorAgentID: "agent-lead",
			RootRoundID:        "root-round-1",
			AgentRoundID:       "agent-round-member",
			SourceContextType:  "room",
			SourceContextID:    "room-1",
			RoomID:             "room-1",
			ConversationID:     "conversation-1",
		},
	)
	if !ok {
		t.Fatal("Room runtime identity should produce an Execution MCP context")
	}
	if serverContext.ScopeKind != protocol.ExecutionScopeRoom ||
		serverContext.Role != orchestrationsvc.ExecutionActorMember ||
		serverContext.ExecutionID != "execution-1" ||
		serverContext.WorkBinding == nil ||
		serverContext.WorkBinding.AssignmentID != "assignment-1" ||
		serverContext.RuntimeRoundID != "agent-round-member" ||
		serverContext.RoomID != "room-1" ||
		serverContext.ConversationID != "conversation-1" ||
		serverContext.RoomSessionID != "" {
		t.Fatalf("Room Execution server context = %+v", serverContext)
	}
	if reader.actor.Role != "" ||
		reader.actor.AgentID != "agent-member" ||
		reader.actor.WorkBinding == nil ||
		reader.actor.WorkBinding.AssignmentID != "assignment-1" ||
		reader.actor.RoomID != "room-1" ||
		reader.actor.ConversationID != "conversation-1" {
		t.Fatalf("snapshot lookup actor = %+v", reader.actor)
	}
	if reader.requestedID != "execution-1" ||
		reader.getSnapshotCalls != 1 ||
		reader.getCurrentCalls != 0 {
		t.Fatalf("snapshot lookup = id %q, explicit %d, current %d", reader.requestedID, reader.getSnapshotCalls, reader.getCurrentCalls)
	}
	if serverContext.WorkBinding == workBinding ||
		reader.actor.WorkBinding == workBinding ||
		reader.actor.WorkBinding == serverContext.WorkBinding {
		t.Fatal("runtime, MCP and lookup actor must not share a mutable WorkBinding pointer")
	}
	workBinding.AssignmentID = "assignment-mutated"
	if serverContext.WorkBinding.AssignmentID != "assignment-1" ||
		reader.actor.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatal("upstream WorkBinding mutation crossed a clone boundary")
	}
}

func TestResolveExecutionMCPServerContextKeepsUnboundRoomMemberSurface(t *testing.T) {
	reader := &stubExecutionCurrentReader{}
	serverContext, ok := resolveExecutionMCPServerContext(
		context.Background(),
		reader,
		runtimectx.ExecutionToolContext{
			Agent:              &protocol.Agent{AgentID: "agent-member", OwnerUserID: "owner-1"},
			ScopeSessionKey:    "room:group:conversation-1",
			CoordinatorAgentID: "agent-lead",
			SourceContextType:  "room",
			RoomID:             "room-1",
			ConversationID:     "conversation-1",
		},
	)
	if !ok || serverContext.Role != orchestrationsvc.ExecutionActorMember ||
		serverContext.WorkBinding != nil || serverContext.ReviewBinding != nil {
		t.Fatalf("unbound Room member surface = %+v, ok=%t", serverContext, ok)
	}
}

func TestResolveExecutionMCPServerContextKeepsBootstrapCoordinator(t *testing.T) {
	reader := &stubExecutionCurrentReader{}
	serverContext, ok := resolveExecutionMCPServerContext(
		context.Background(),
		reader,
		runtimectx.ExecutionToolContext{
			Agent:              &protocol.Agent{AgentID: "agent-lead", OwnerUserID: "owner-1"},
			ScopeSessionKey:    "room:group:conversation-1",
			CoordinatorAgentID: "agent-lead",
			SourceContextType:  "room",
			RoomID:             "room-1",
			ConversationID:     "conversation-1",
		},
	)
	if !ok || serverContext.Role != orchestrationsvc.ExecutionActorCoordinator {
		t.Fatalf("bootstrap coordinator context = %+v, ok=%t", serverContext, ok)
	}
}

func TestResolveExecutionMCPServerContextKeepsSurfaceOnSnapshotError(t *testing.T) {
	reader := &stubExecutionCurrentReader{err: errors.New("database unavailable")}
	serverContext, ok := resolveExecutionMCPServerContext(
		context.Background(),
		reader,
		runtimectx.ExecutionToolContext{
			Agent:             &protocol.Agent{AgentID: "agent-1", OwnerUserID: "owner-1"},
			ScopeSessionKey:   "room:group:conversation-1",
			SourceContextType: "room",
			RoomID:            "room-1",
			ConversationID:    "conversation-1",
		},
	)
	if !ok || serverContext.Role != orchestrationsvc.ExecutionActorMember {
		t.Fatalf("snapshot failure changed Execution surface: %+v, ok=%t", serverContext, ok)
	}
}
