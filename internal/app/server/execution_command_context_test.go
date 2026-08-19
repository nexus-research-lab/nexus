// INPUT: Host-fixed Room identity plus an optional current WorkGraph observation.
// OUTPUT: Coordinator/member command role derived without requiring a mutation binding.
// POS: Room runtime command admission regression tests for fresh, recovery, and member rounds.
package server

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionCommandContextReader struct {
	snapshot *protocol.ExecutionSnapshot
	err      error
}

func (r executionCommandContextReader) ReadCurrent(
	context.Context,
	orchestrationsvc.ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	return r.snapshot, r.err
}

func (r executionCommandContextReader) ReadSnapshot(
	context.Context,
	orchestrationsvc.ActorContext,
	string,
) (*protocol.ExecutionSnapshot, error) {
	return r.snapshot, r.err
}

func TestResolveExecutionCommandContextUsesObservationForRoomRole(t *testing.T) {
	for _, testCase := range []struct {
		name              string
		agentID           string
		hostCoordinator   string
		snapshot          *protocol.ExecutionSnapshot
		readErr           error
		wantRole          orchestrationsvc.ExecutionActorRole
	}{
		{
			name: "fresh Room host may create the first WorkGraph",
			agentID: "agent-lead", hostCoordinator: "agent-lead",
			wantRole: orchestrationsvc.ExecutionActorCoordinator,
		},
		{
			name: "current WorkGraph coordinator may recover coordination",
			agentID: "agent-lead", hostCoordinator: "agent-lead",
			snapshot: roomExecutionCommandSnapshot("agent-lead"),
			wantRole: orchestrationsvc.ExecutionActorCoordinator,
		},
		{
			name: "verified Room member remains observation-only",
			agentID: "agent-member", hostCoordinator: "agent-lead",
			snapshot: roomExecutionCommandSnapshot("agent-lead"),
			wantRole: orchestrationsvc.ExecutionActorMember,
		},
		{
			name: "read failure cannot elect the host",
			agentID: "agent-lead", hostCoordinator: "agent-lead",
			readErr: errors.New("Room observation unavailable"),
			wantRole: orchestrationsvc.ExecutionActorMember,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			runtimeContext := runtimectx.RuntimeCommandContext{
				Agent: &protocol.Agent{
					AgentID: testCase.agentID,
					OwnerUserID: "owner-1",
				},
				ScopeSessionKey: "room:group:conversation-1",
				RuntimeSessionKey: "agent:" + testCase.agentID + ":room:conversation-1",
				RootRoundID: "round-root",
				AgentRoundID: "round-agent",
				SourceContextType: "room",
				RoomID: "room-1",
				ConversationID: "conversation-1",
				CoordinatorAgentID: testCase.hostCoordinator,
			}
			resolved, ok := resolveExecutionCommandContext(
				context.Background(),
				executionCommandContextReader{snapshot: testCase.snapshot, err: testCase.readErr},
				runtimeContext,
			)
			if !ok || resolved.Role != testCase.wantRole {
				t.Fatalf("resolved=%+v ok=%t, want role %q", resolved, ok, testCase.wantRole)
			}
		})
	}
}

func roomExecutionCommandSnapshot(coordinatorAgentID string) *protocol.ExecutionSnapshot {
	return &protocol.ExecutionSnapshot{Execution: protocol.Execution{
		ID: "execution-1",
		OwnerUserID: "owner-1",
		SessionKey: "room:group:conversation-1",
		ScopeKind: protocol.ExecutionScopeRoom,
		RoomID: "room-1",
		ConversationID: "conversation-1",
		CoordinatorAgentID: coordinatorAgentID,
	}}
}
