package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

type staticGoalSessionOwnershipVerifier struct {
	trustedAgentID   string
	trustedAgentName string
	err              error
}

func (v staticGoalSessionOwnershipVerifier) VerifyGoalSessionOwnership(
	_ context.Context,
	request GoalSessionOwnershipRequest,
) (GoalSessionOwnershipProof, error) {
	if v.err != nil {
		return GoalSessionOwnershipProof{}, v.err
	}
	return GoalSessionOwnershipProof{
		TrustedAgentID:   v.trustedAgentID,
		TrustedAgentName: v.trustedAgentName,
	}, nil
}

type recordingGoalSessionOwnershipVerifier struct {
	err      error
	proof    GoalSessionOwnershipProof
	requests []GoalSessionOwnershipRequest
}

type roomMemberGoalSessionOwnershipVerifier struct {
	members  map[string]string
	requests []GoalSessionOwnershipRequest
}

func (v *roomMemberGoalSessionOwnershipVerifier) VerifyGoalSessionOwnership(
	_ context.Context,
	request GoalSessionOwnershipRequest,
) (GoalSessionOwnershipProof, error) {
	v.requests = append(v.requests, request)
	agentID := request.TrustedAgentID
	agentName, exists := v.members[agentID]
	if !exists {
		return GoalSessionOwnershipProof{}, errors.New("not a Room member")
	}
	return GoalSessionOwnershipProof{
		TrustedAgentID:   agentID,
		TrustedAgentName: agentName,
	}, nil
}

func (v *recordingGoalSessionOwnershipVerifier) VerifyGoalSessionOwnership(
	_ context.Context,
	request GoalSessionOwnershipRequest,
) (GoalSessionOwnershipProof, error) {
	v.requests = append(v.requests, request)
	if v.err != nil {
		return GoalSessionOwnershipProof{}, v.err
	}
	return v.proof, nil
}

func TestGoalCreateRejectsForeignDMSessionBeforePersistenceOrContinuation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.idFactory = sequentialID()
	verifier := &recordingGoalSessionOwnershipVerifier{err: errors.New("foreign Agent")}
	service.SetSessionOwnershipVerifier(verifier)
	dispatcher := &fakeContinuationDispatcher{}
	service.SetContinuationDispatcher(dispatcher)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner-attacker", Role: authctx.RoleOwner,
	})

	_, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:victim-agent:nexus:dm:victim-thread",
		Objective:   "attach a Goal to the victim Agent",
		CreatedBy:   "user",
		OwnerUserID: "owner-attacker",
	})
	if !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("Create() error = %v, want ErrGoalForbidden", err)
	}
	if len(verifier.requests) != 1 ||
		verifier.requests[0].OwnerUserID != "owner-attacker" {
		t.Fatalf("ownership requests = %#v", verifier.requests)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 ||
		len(dispatcher.plans) != 0 || dispatcher.deferCalls != 0 {
		t.Fatalf("goals=%d events=%d plans=%d deferCalls=%d, want no side effects",
			len(repo.goals), len(repo.events), len(dispatcher.plans), dispatcher.deferCalls)
	}
}

func TestAppServerNoCurrentSetRejectsForeignRoomBeforePersistenceOrContinuation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.idFactory = sequentialID()
	verifier := &recordingGoalSessionOwnershipVerifier{err: errors.New("foreign Room")}
	service.SetSessionOwnershipVerifier(verifier)
	dispatcher := &fakeContinuationDispatcher{}
	service.SetContinuationDispatcher(dispatcher)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner-attacker", Role: authctx.RoleOwner,
	})
	objective := "attach a Goal to the victim Room"

	_, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:    protocol.BuildRoomSharedSessionKey("victim-conversation"),
		Objective:   &objective,
		OwnerUserID: "owner-attacker",
	})
	if !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("SetFromThreadGoalParams() error = %v, want ErrGoalForbidden", err)
	}
	if len(verifier.requests) != 1 ||
		verifier.requests[0].SessionKey != protocol.BuildRoomSharedSessionKey("victim-conversation") {
		t.Fatalf("ownership requests = %#v", verifier.requests)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 ||
		len(dispatcher.plans) != 0 || dispatcher.deferCalls != 0 {
		t.Fatalf("goals=%d events=%d plans=%d deferCalls=%d, want no side effects",
			len(repo.goals), len(repo.events), len(dispatcher.plans), dispatcher.deferCalls)
	}
}

func TestRoomGoalOwnershipMetadataUsesOnlyVerifiedRuntimeIdentity(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	service.SetSessionOwnershipVerifier(staticGoalSessionOwnershipVerifier{
		trustedAgentID: "agent-verified", trustedAgentName: "Verified Lead",
	})
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("verified-room"),
		Objective:  "coordinate the verified Room",
		CreatedBy:  "model",
		AgentID:    "agent-verified",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalCreatorAgentID: "agent-forged",
			protocol.GoalMetadataRoomGoalLeadAgentID:    "agent-forged",
			protocol.GoalMetadataRoomGoalLeadAgentName:  "Forged Lead",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalMetadataString(created.Metadata, protocol.GoalMetadataRoomGoalCreatorAgentID); got != "agent-verified" {
		t.Fatalf("creator = %q, want verified runtime Agent", got)
	}
	if got := RoomLeadAgentID(*created); got != "agent-verified" {
		t.Fatalf("lead = %q, want verified runtime Agent", got)
	}
	if got := RoomLeadAgentName(*created); got != "Verified Lead" {
		t.Fatalf("lead name = %q, want server-verified display name", got)
	}
}

func TestModelCreatedRoomGoalDoesNotPersistMemberCountAsCompletionPolicy(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	service.SetSessionOwnershipVerifier(staticGoalSessionOwnershipVerifier{
		trustedAgentID:   "agent-lead",
		trustedAgentName: "Verified Lead",
	})

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("verified-multi-member-room"),
		Objective:  "coordinate all verified Room members",
		CreatedBy:  "model",
		AgentID:    "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := created.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequired]; exists {
		t.Fatalf("metadata = %#v, member count must not become Goal completion policy", created.Metadata)
	}
}

func TestUserCreatedRoomGoalUsesVerifiedSelectedLead(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	verifier := &roomMemberGoalSessionOwnershipVerifier{members: map[string]string{
		"agent-selected": "Directory Lead",
	}}
	service.SetSessionOwnershipVerifier(verifier)
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:      protocol.BuildRoomSharedSessionKey("user-selected-lead"),
		Objective:       "coordinate the Room",
		CreatedBy:       "user",
		RoomLeadAgentID: "agent-selected",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalCreatorAgentID:        "agent-forged",
			protocol.GoalMetadataRoomGoalLeadAgentID:           "agent-forged",
			protocol.GoalMetadataRoomGoalLeadAgentName:         "Forged Lead",
			protocol.GoalMetadataRoomGoalCollaborationRequired: false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalMetadataString(created.Metadata, protocol.GoalMetadataRoomGoalCreatorAgentID); got != "" {
		t.Fatalf("user-created Room Goal creator = %q, want no forged model creator", got)
	}
	if got := RoomLeadAgentID(*created); got != "agent-selected" {
		t.Fatalf("lead = %q, want verified selected member", got)
	}
	if got := RoomLeadAgentName(*created); got != "Directory Lead" {
		t.Fatalf("lead name = %q, want server directory value", got)
	}
	if _, exists := created.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequired]; exists {
		t.Fatalf("Room collaboration requirement = %#v, want external value removed", created.Metadata)
	}
	if len(verifier.requests) != 1 ||
		verifier.requests[0].TrustedAgentID != "agent-selected" {
		t.Fatalf("membership verification requests = %#v", verifier.requests)
	}
}

func TestUserCreatedRoomGoalReplacementCommitsVerifiedRoomStateInOneMutation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetSessionOwnershipVerifier(&roomMemberGoalSessionOwnershipVerifier{members: map[string]string{
		"agent-old": "Old Lead",
		"agent-new": "New Lead",
	}})
	sessionKey := protocol.BuildRoomSharedSessionKey("user-replaced-lead")
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:      sessionKey,
		Objective:       "old Room objective",
		CreatedBy:       "user",
		RoomLeadAgentID: "agent-old",
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err = service.RecordRoomGoalCollaborationEvidence(
		context.Background(),
		created.ID,
		"peer-round-before-replace",
		"agent-peer",
		created.ObjectiveRevision(),
	)
	if err != nil || !RoomCollaborationObserved(*created) {
		t.Fatalf("initial collaboration = %#v err=%v", created, err)
	}
	eventsBefore := len(repo.events)
	replaced, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:      sessionKey,
		Objective:       "new Room objective",
		CreatedBy:       "user",
		ReplaceExisting: true,
		RoomLeadAgentID: "agent-new",
		RoundID:         "goal-command-round",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.ID != created.ID || replaced.Version != created.Version+1 {
		t.Fatalf("replaced Goal = %#v, want same Goal and one version transition", replaced)
	}
	if replaced.Objective != "new Room objective" ||
		RoomLeadAgentID(*replaced) != "agent-new" ||
		RoomLeadAgentName(*replaced) != "New Lead" ||
		!RoomCollaborationObserved(*replaced) {
		t.Fatalf("replaced Room state = %#v, want verified lead and Goal-lifecycle collaboration", replaced)
	}
	if got := protocol.GoalMetadataString(
		replaced.Metadata,
		protocol.GoalMetadataRoomGoalCollaborationAgentID,
	); got != "agent-peer" {
		t.Fatalf("collaboration agent = %q, want agent-peer retained", got)
	}
	if _, exists := replaced.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequirementRound]; exists {
		t.Fatalf("replacement metadata = %#v, requirement round must not be written", replaced.Metadata)
	}
	if len(repo.events) != eventsBefore+1 || repo.events[len(repo.events)-1].EventType != "updated" {
		t.Fatalf("replacement events = %#v, want one atomic updated event", repo.events[eventsBefore:])
	}
}

func TestUserCreatedRoomGoalRejectsUnverifiedSelectedLead(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	verifier := &roomMemberGoalSessionOwnershipVerifier{members: map[string]string{}}
	service.SetSessionOwnershipVerifier(verifier)
	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:      protocol.BuildRoomSharedSessionKey("invalid-selected-lead"),
		Objective:       "coordinate the Room",
		CreatedBy:       "user",
		RoomLeadAgentID: "agent-outsider",
	})
	if !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("Create() error = %v, want ErrGoalForbidden", err)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 {
		t.Fatalf("goals=%d events=%d, want no side effects", len(repo.goals), len(repo.events))
	}
}

func TestSetRoomGoalLeadRequiresFreshServerMembershipProof(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("lead-proof"),
		Objective:  "coordinate the Room",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = service.SetRoomGoalLead(context.Background(), created.ID, "agent-outsider"); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("SetRoomGoalLead() without verifier error = %v, want ErrGoalForbidden", err)
	}
	stored, err := repo.GetGoal(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := RoomLeadAgentID(*stored); got != "" {
		t.Fatalf("lead after unverified assignment = %q, want empty", got)
	}

	rejected := &recordingGoalSessionOwnershipVerifier{err: errors.New("not a Room member")}
	service.SetSessionOwnershipVerifier(rejected)
	if _, err = service.SetRoomGoalLead(context.Background(), created.ID, "agent-outsider"); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("SetRoomGoalLead() rejected membership error = %v, want ErrGoalForbidden", err)
	}
	stored, err = repo.GetGoal(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := RoomLeadAgentID(*stored); got != "" {
		t.Fatalf("lead after rejected membership = %q, want empty", got)
	}

	verified := &recordingGoalSessionOwnershipVerifier{proof: GoalSessionOwnershipProof{
		TrustedAgentID:   "agent-member",
		TrustedAgentName: "Directory Name",
	}}
	service.SetSessionOwnershipVerifier(verified)
	updated, err := service.SetRoomGoalLead(context.Background(), created.ID, "agent-member")
	if err != nil {
		t.Fatal(err)
	}
	if got := RoomLeadAgentID(*updated); got != "agent-member" {
		t.Fatalf("verified lead = %q, want agent-member", got)
	}
	if got := RoomLeadAgentName(*updated); got != "Directory Name" {
		t.Fatalf("verified lead name = %q, want server directory value", got)
	}
	if len(verified.requests) != 1 ||
		verified.requests[0].SessionKey != created.SessionKey ||
		verified.requests[0].TrustedAgentID != "agent-member" {
		t.Fatalf("membership verification requests = %#v", verified.requests)
	}
}

func TestCurrentModelMutationAuthorityUsesRoomLeadAndExactRevision(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:  protocol.BuildRoomSharedSessionKey("owner-authority"),
		Objective:   "finish the Room work",
		CreatedBy:   "model",
		OwnerUserID: "owner-1",
		AgentID:     "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}
	created.Metadata[protocol.GoalMetadataObjectiveRevision] = int64(3)
	repo.goals[created.ID] = *created

	granted, err := service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-lead",
	)
	if err != nil {
		t.Fatal(err)
	}
	if granted.ID != created.ID || granted.ObjectiveRevision() != 3 {
		t.Fatalf("granted Goal = %#v, want exact current revision", granted)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-peer",
	); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("peer authority error = %v, want ErrGoalForbidden", err)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-2",
		"agent-lead",
	); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("foreign owner authority error = %v, want ErrGoalForbidden", err)
	}
}

func TestCurrentModelMutationAuthorityUsesDMAgentIdentity(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:  protocol.BuildAgentSessionKey("agent-owner", protocol.SessionChannelWebSocketSegment, "dm", "chat-1", ""),
		Objective:   "finish the DM work",
		CreatedBy:   "model",
		OwnerUserID: "owner-1",
		AgentID:     "agent-owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-owner",
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CurrentModelMutationAuthority(
		context.Background(),
		created.SessionKey,
		"owner-1",
		"agent-other",
	); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("other DM Agent error = %v, want ErrGoalForbidden", err)
	}
}

func TestServiceGoalByIDForOwnerRequiresExactDurableProvenance(t *testing.T) {
	repo := newMemoryRepository()
	repo.goals["goal-owned"] = protocol.Goal{
		ID:         "goal-owned",
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-source"),
		Objective:  "coordinate across topics",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID: "owner-exact",
		},
	}
	repo.goals["goal-ownerless"] = protocol.Goal{
		ID:         "goal-ownerless",
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-legacy"),
		Objective:  "legacy goal without durable owner",
		Status:     protocol.GoalStatusActive,
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)

	item, err := service.GoalByIDForOwner(context.Background(), "goal-owned", "owner-exact")
	if err != nil || item == nil || item.ID != "goal-owned" {
		t.Fatalf("exact owner read = %+v, %v", item, err)
	}
	for name, goalID := range map[string]string{
		"foreign owner":    "goal-owned",
		"ownerless legacy": "goal-ownerless",
	} {
		t.Run(name, func(t *testing.T) {
			if item, err := service.GoalByIDForOwner(
				context.Background(), goalID, "owner-foreign",
			); item != nil || !errors.Is(err, ErrGoalForbidden) {
				t.Fatalf("GoalByIDForOwner() = %+v, %v; want forbidden", item, err)
			}
		})
	}
}
