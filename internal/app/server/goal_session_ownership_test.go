package server

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type ownershipGoalRepository struct {
	createCalls int
}

func (r *ownershipGoalRepository) CreateGoal(context.Context, protocol.Goal) (*protocol.Goal, error) {
	r.createCalls++
	return nil, errors.New("unexpected Goal persistence")
}

func (r *ownershipGoalRepository) CreateGoalWithEvent(
	context.Context,
	protocol.Goal,
	protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.createCalls++
	return nil, errors.New("unexpected Goal persistence")
}

func (*ownershipGoalRepository) GetGoal(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) GetCurrentGoal(context.Context, string) (*protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) ListGoals(context.Context) ([]protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) ListCurrentGoals(context.Context) ([]protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) ListRunnableGoals(context.Context, int) ([]protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) UpdateGoal(context.Context, protocol.Goal, int64) (*protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) UpdateGoalWithEvents(
	context.Context,
	protocol.Goal,
	int64,
	[]protocol.GoalEvent,
) (*protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) FinalizeGoalUsage(context.Context, protocol.Goal, int64, protocol.GoalEvent) (*protocol.Goal, error) {
	return nil, nil
}

func (*ownershipGoalRepository) DeleteGoal(context.Context, string) (bool, error) {
	return false, nil
}

func (*ownershipGoalRepository) AppendEvent(context.Context, protocol.GoalEvent) error {
	return nil
}

func (*ownershipGoalRepository) ListEvents(context.Context, string, int) ([]protocol.GoalEvent, error) {
	return nil, nil
}

type ownershipContinuationDispatcher struct {
	calls int
}

func (*ownershipContinuationDispatcher) ShouldDeferGoalContinuation(context.Context, string) bool {
	return false
}

func (d *ownershipContinuationDispatcher) DispatchGoalContinuation(context.Context, protocol.GoalContinuation) error {
	d.calls++
	return nil
}

type ownerScopedGoalSessionAgentReader struct {
	agents map[string]map[string]protocol.Agent
}

func (r ownerScopedGoalSessionAgentReader) GetAgent(ctx context.Context, agentID string) (*protocol.Agent, error) {
	item, ok := r.agents[authctx.OwnerUserID(ctx)][agentID]
	if !ok {
		return nil, agentsvc.ErrAgentNotFound
	}
	return &item, nil
}

type ownerScopedGoalSessionRoomReader struct {
	contexts map[string]map[string]protocol.ConversationContextAggregate
}

func (r ownerScopedGoalSessionRoomReader) GetConversationContext(
	ctx context.Context,
	conversationID string,
) (*protocol.ConversationContextAggregate, error) {
	item, ok := r.contexts[authctx.OwnerUserID(ctx)][conversationID]
	if !ok {
		return nil, roomsvc.ErrConversationNotFound
	}
	return &item, nil
}

func TestGoalSessionOwnershipVerifierRejectsCrossOwnerAgentAndRoomKeys(t *testing.T) {
	const (
		attacker = "owner-attacker"
		victim   = "owner-victim"
	)
	verifier := newGoalSessionOwnershipVerifier(
		ownerScopedGoalSessionAgentReader{agents: map[string]map[string]protocol.Agent{
			victim: {
				"victim-agent": {AgentID: "victim-agent", OwnerUserID: victim, Status: "active"},
			},
		}},
		ownerScopedGoalSessionRoomReader{contexts: map[string]map[string]protocol.ConversationContextAggregate{
			victim: {
				"victim-conversation": {
					Room:         protocol.RoomRecord{ID: "victim-room", OwnerUserID: victim},
					Conversation: protocol.ConversationRecord{ID: "victim-conversation", RoomID: "victim-room"},
				},
			},
		}},
	)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: attacker, Role: authctx.RoleOwner,
	})
	for _, request := range []goalsvc.GoalSessionOwnershipRequest{
		{
			OwnerUserID: attacker,
			SessionKey:  "agent:victim-agent:nexus:dm:victim-thread",
		},
		{
			OwnerUserID: attacker,
			SessionKey:  protocol.BuildRoomSharedSessionKey("victim-conversation"),
		},
	} {
		if _, err := verifier.VerifyGoalSessionOwnership(ctx, request); err == nil {
			t.Fatalf("VerifyGoalSessionOwnership(%#v) succeeded for foreign owner", request)
		}
	}
}

func TestGoalSessionOwnershipVerifierReturnsOnlyServerVerifiedRoomMember(t *testing.T) {
	const owner = "owner-1"
	verifier := newGoalSessionOwnershipVerifier(
		ownerScopedGoalSessionAgentReader{agents: map[string]map[string]protocol.Agent{
			owner: {
				"agent-lead": {
					AgentID: "agent-lead", OwnerUserID: owner, Status: "active",
					DisplayName: "Verified Lead",
				},
			},
		}},
		ownerScopedGoalSessionRoomReader{contexts: map[string]map[string]protocol.ConversationContextAggregate{
			owner: {
				"conversation-1": {
					Room:         protocol.RoomRecord{ID: "room-1", OwnerUserID: owner},
					Conversation: protocol.ConversationRecord{ID: "conversation-1", RoomID: "room-1"},
					Members: []protocol.MemberRecord{
						{RoomID: "room-1", MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-lead"},
						{RoomID: "room-1", MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-peer"},
					},
				},
			},
		}},
	)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: owner, Role: authctx.RoleOwner,
	})
	proof, err := verifier.VerifyGoalSessionOwnership(ctx, goalsvc.GoalSessionOwnershipRequest{
		OwnerUserID:    owner,
		SessionKey:     protocol.BuildRoomSharedSessionKey("conversation-1"),
		TrustedAgentID: "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}
	if proof.TrustedAgentID != "agent-lead" ||
		proof.TrustedAgentName != "Verified Lead" {
		t.Fatalf("proof = %#v", proof)
	}
	if _, err = verifier.VerifyGoalSessionOwnership(ctx, goalsvc.GoalSessionOwnershipRequest{
		OwnerUserID:    owner,
		SessionKey:     protocol.BuildRoomSharedSessionKey("conversation-1"),
		TrustedAgentID: "agent-outsider",
	}); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("unverified Room member error = %v", err)
	}
}

func TestGoalSessionOwnershipVerifierUsesPersistedAgentForAllAgentSessionShapes(t *testing.T) {
	const owner = "owner-1"
	verifier := newGoalSessionOwnershipVerifier(
		ownerScopedGoalSessionAgentReader{agents: map[string]map[string]protocol.Agent{
			owner: {
				"agent-1": {
					AgentID: "agent-1", OwnerUserID: owner, Status: "active",
				},
			},
		}},
		ownerScopedGoalSessionRoomReader{},
	)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: owner, Role: authctx.RoleOwner,
	})

	for _, sessionKey := range []string{
		protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelDiscord, "group", "provider-chat", ""),
		protocol.BuildRoomAgentSessionKey("unresolved-room-ref", "agent-1", protocol.RoomTypeDM),
	} {
		if _, err := verifier.VerifyGoalSessionOwnership(ctx, goalsvc.GoalSessionOwnershipRequest{
			OwnerUserID: owner,
			SessionKey:  sessionKey,
		}); err != nil {
			t.Fatalf("VerifyGoalSessionOwnership(%q) error = %v", sessionKey, err)
		}
	}
	if _, err := verifier.VerifyGoalSessionOwnership(ctx, goalsvc.GoalSessionOwnershipRequest{
		OwnerUserID:    owner,
		SessionKey:     protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelDiscord, "group", "provider-chat", ""),
		TrustedAgentID: "agent-other",
	}); err == nil {
		t.Fatal("mismatched runtime Agent unexpectedly passed ownership verification")
	}
}

func TestGoalSessionOwnershipApplicationGateRejectsVictimDMAndRoomWithoutSideEffects(t *testing.T) {
	const (
		attacker = "owner-attacker"
		victim   = "owner-victim"
	)
	verifier := newGoalSessionOwnershipVerifier(
		ownerScopedGoalSessionAgentReader{agents: map[string]map[string]protocol.Agent{
			victim: {
				"victim-agent": {AgentID: "victim-agent", OwnerUserID: victim, Status: "active"},
			},
		}},
		ownerScopedGoalSessionRoomReader{contexts: map[string]map[string]protocol.ConversationContextAggregate{
			victim: {
				"victim-conversation": {
					Room:         protocol.RoomRecord{ID: "victim-room", OwnerUserID: victim},
					Conversation: protocol.ConversationRecord{ID: "victim-conversation", RoomID: "victim-room"},
				},
			},
		}},
	)
	repo := &ownershipGoalRepository{}
	service := goalsvc.NewService(config.Config{
		GoalEnabled: true, GoalAutoContinueEnabled: true,
	}, repo)
	service.SetSessionOwnershipVerifier(verifier)
	dispatcher := &ownershipContinuationDispatcher{}
	service.SetContinuationDispatcher(dispatcher)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: attacker, Role: authctx.RoleOwner,
	})

	_, createErr := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:victim-agent:nexus:dm:victim-thread",
		Objective:  "foreign DM Goal", CreatedBy: "user", OwnerUserID: attacker,
	})
	objective := "foreign Room Goal"
	_, setErr := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  protocol.BuildRoomSharedSessionKey("victim-conversation"),
		Objective: &objective, OwnerUserID: attacker,
	})
	if !errors.Is(createErr, goalsvc.ErrGoalForbidden) ||
		!errors.Is(setErr, goalsvc.ErrGoalForbidden) {
		t.Fatalf("createErr=%v setErr=%v, want owner-scoped rejection", createErr, setErr)
	}
	if repo.createCalls != 0 || dispatcher.calls != 0 {
		t.Fatalf("createCalls=%d continuationCalls=%d, want no side effects", repo.createCalls, dispatcher.calls)
	}
}
