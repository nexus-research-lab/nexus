package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type commandContextSnapshotReader struct{}

func (commandContextSnapshotReader) ReadCurrent(context.Context, orchestrationsvc.ActorContext) (*protocol.ExecutionSnapshot, error) {
	return nil, errors.New("test snapshot unavailable")
}

func (commandContextSnapshotReader) ReadSnapshot(context.Context, orchestrationsvc.ActorContext, string) (*protocol.ExecutionSnapshot, error) {
	return nil, errors.New("test snapshot unavailable")
}

func TestResolveCommandContextPreservesRoomFallbackRole(t *testing.T) {
	resolved, ok := ResolveCommandContext(context.Background(), commandContextSnapshotReader{}, runtimectx.RuntimeCommandContext{
		Agent:             &protocol.Agent{OwnerUserID: "owner-1", AgentID: "agent-1"},
		ScopeSessionKey:   "room:room-1:conversation-1",
		RuntimeSessionKey: "room:room-1:conversation-1",
		RootRoundID:       "root-1", AgentRoundID: "agent-round-1",
		SourceContextType: "room", SourceContextID: "room-1",
		RoomID: "room-1", ConversationID: "conversation-1",
	})
	if !ok || resolved.ScopeKind != protocol.ExecutionScopeRoom ||
		resolved.Role != orchestrationsvc.ExecutionActorMember {
		t.Fatalf("room context changed unexpectedly: resolved=%+v ok=%t", resolved, ok)
	}
}
