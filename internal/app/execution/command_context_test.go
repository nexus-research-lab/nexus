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

func TestResolveCommandContextAcceptsExactDMGoalContinuation(t *testing.T) {
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"conversation-1",
		"",
	)
	goal := runtimectx.NewGoalAuthorityState("goal-1", 3, "execution-1")
	responsibility := runtimectx.NewResponsibilityAuthorityState(goal, "execution-1", nil, nil)
	commandContext := runtimectx.RuntimeCommandContext{
		Agent:           &protocol.Agent{OwnerUserID: "owner-1", AgentID: "agent-1"},
		ScopeSessionKey: sessionKey, RuntimeSessionKey: sessionKey,
		ExecutionID: "execution-1", RootRoundID: "round-1",
		SourceContextType: runtimectx.SourceContextGoalContinuation,
		SourceContextID:   "agent-1",
		GoalAuthority:     goal, ResponsibilityAuthority: responsibility,
		GoalContinuationAuthority: &runtimectx.GoalContinuationAuthority{
			OwnerUserID: "owner-1", AgentID: "agent-1", ScopeSessionKey: sessionKey,
			GoalID: "goal-1", ObjectiveRevision: 3, ExecutionID: "execution-1", RootRoundID: "round-1",
		},
	}

	resolved, ok := ResolveCommandContext(context.Background(), commandContextSnapshotReader{}, commandContext)
	if !ok {
		t.Fatal("exact DM Goal continuation was rejected")
	}
	if resolved.ScopeKind != protocol.ExecutionScopeDM ||
		resolved.Role != orchestrationsvc.ExecutionActorCoordinator ||
		resolved.ExecutionID != "execution-1" {
		t.Fatalf("resolved continuation context = %+v", resolved)
	}
}

func TestResolveCommandContextRejectsMismatchedDMGoalContinuation(t *testing.T) {
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"conversation-1",
		"",
	)
	goal := runtimectx.NewGoalAuthorityState("goal-1", 3, "execution-1")
	responsibility := runtimectx.NewResponsibilityAuthorityState(goal, "execution-1", nil, nil)
	commandContext := runtimectx.RuntimeCommandContext{
		Agent:           &protocol.Agent{OwnerUserID: "owner-1", AgentID: "agent-1"},
		ScopeSessionKey: sessionKey, RuntimeSessionKey: sessionKey,
		ExecutionID: "execution-1", RootRoundID: "round-1",
		SourceContextType: runtimectx.SourceContextGoalContinuation,
		SourceContextID:   "agent-1",
		GoalAuthority:     goal, ResponsibilityAuthority: responsibility,
		GoalContinuationAuthority: &runtimectx.GoalContinuationAuthority{
			OwnerUserID: "owner-1", AgentID: "agent-1", ScopeSessionKey: sessionKey,
			GoalID: "goal-1", ObjectiveRevision: 2, ExecutionID: "execution-1", RootRoundID: "round-1",
		},
	}

	if _, ok := ResolveCommandContext(context.Background(), commandContextSnapshotReader{}, commandContext); ok {
		t.Fatal("mismatched Goal revision must be rejected")
	}
}

func TestResolveCommandContextKeepsOrdinaryInternalRoundClosed(t *testing.T) {
	if _, ok := ResolveCommandContext(context.Background(), commandContextSnapshotReader{}, runtimectx.RuntimeCommandContext{
		Agent:             &protocol.Agent{OwnerUserID: "owner-1", AgentID: "agent-1"},
		ScopeSessionKey:   "agent:agent-1:websocket:dm:conversation-1",
		RuntimeSessionKey: "agent:agent-1:websocket:dm:conversation-1",
		RootRoundID:       "round-1",
		SourceContextType: "agent_internal",
		SourceContextID:   "agent-1",
	}); ok {
		t.Fatal("ordinary agent_internal round must remain closed")
	}
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
