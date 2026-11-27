// ownership_test.go 覆盖 Goal 拥有者认领、授权副作用与会话归属测试。
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

type legacyGoalOwnerClaimReadiness struct {
	resolution          protocol.GoalExecutionBindingResolution
	ownerMatches        bool
	resolveCalls        int
	ownerVerifyCalls    int
	verifiedExecutionID string
	verifiedGoalID      string
	verifiedRevision    int64
	verifiedOwner       string
}

func (r *legacyGoalOwnerClaimReadiness) ResolveGoalExecutionBinding(
	_ context.Context,
	_ protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	r.resolveCalls++
	return r.resolution, nil
}

func (r *legacyGoalOwnerClaimReadiness) ValidateGoalRevisionOwner(
	_ context.Context,
	executionID string,
	goalID string,
	revision int64,
	ownerUserID string,
) (bool, error) {
	r.ownerVerifyCalls++
	r.verifiedExecutionID = executionID
	r.verifiedGoalID = goalID
	r.verifiedRevision = revision
	r.verifiedOwner = ownerUserID
	return r.ownerMatches, nil
}

func (*legacyGoalOwnerClaimReadiness) ExecutionGoalCompletionBlocker(
	context.Context,
	protocol.Goal,
) (string, error) {
	return "", nil
}

func newOwnerClaimService(
	t *testing.T,
	status protocol.GoalStatus,
	state protocol.GoalExecutionBindingState,
) (*Service, *memoryRepository, *recordingGoalSessionOwnershipVerifier, *legacyGoalOwnerClaimReadiness, context.Context, protocol.Goal) {
	t.Helper()
	const ownerUserID = "owner-legacy"
	item := protocol.Goal{
		ID:         "goal-legacy",
		SessionKey: "agent:agent-legacy:nexus:dm:thread-legacy",
		Objective:  "legacy objective",
		Status:     status,
		Version:    1,
		Metadata:   map[string]any{},
	}
	repo := newMemoryRepository()
	repo.goals[item.ID] = item
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	verifier := &recordingGoalSessionOwnershipVerifier{}
	service.SetSessionOwnershipVerifier(verifier)
	readiness := &legacyGoalOwnerClaimReadiness{
		resolution:   protocol.GoalExecutionBindingResolution{State: state},
		ownerMatches: true,
	}
	if state == protocol.GoalExecutionBindingStateConfirmed {
		readiness.resolution.ExecutionID = "execution-legacy"
	}
	service.SetExecutionGoalCompletionReadiness(readiness)
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	})
	return service, repo, verifier, readiness, ctx, item
}

func TestLegacyGoalClaimFailsClosedForUnsettledBindingBeforeMutationSideEffects(t *testing.T) {
	for _, state := range []protocol.GoalExecutionBindingState{
		protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict,
	} {
		t.Run(string(state), func(t *testing.T) {
			service, repo, _, _, ctx, item := newOwnerClaimService(t, protocol.GoalStatusActive, state)
			accountant := &fakeExternalMutationAccountant{service: service}
			service.SetExternalMutationAccountant(accountant)
			objective := "must not be applied"

			_, err := service.Update(ctx, item.ID, protocol.UpdateGoalRequest{
				Objective:   &objective,
				OwnerUserID: "owner-legacy",
			})
			if !errors.Is(err, ErrGoalForbidden) {
				t.Fatalf("Update() error = %v, want ErrGoalForbidden", err)
			}
			stored := repo.goals[item.ID]
			if stored.Version != item.Version || stored.Objective != item.Objective || len(stored.Metadata) != 0 {
				t.Fatalf("stored Goal = %#v, want unchanged %#v", stored, item)
			}
			if len(accountant.sessionKeys) != 0 || len(repo.events) != 0 {
				t.Fatalf("accounting=%#v events=%#v, want no mutation side effects", accountant.sessionKeys, repo.events)
			}
		})
	}
}

func TestLegacyConfirmedGoalClaimRequiresExactExecutionOwner(t *testing.T) {
	for _, ownerMatches := range []bool{false, true} {
		t.Run(map[bool]string{false: "mismatch", true: "match"}[ownerMatches], func(t *testing.T) {
			service, repo, _, readiness, ctx, item := newOwnerClaimService(
				t,
				protocol.GoalStatusActive,
				protocol.GoalExecutionBindingStateConfirmed,
			)
			readiness.ownerMatches = ownerMatches

			claimed, err := service.CurrentOptionalForOwner(ctx, item.SessionKey, "owner-legacy")
			if !ownerMatches {
				if !errors.Is(err, ErrGoalForbidden) {
					t.Fatalf("CurrentOptionalForOwner() error = %v, want ErrGoalForbidden", err)
				}
				if len(repo.goals[item.ID].Metadata) != 0 {
					t.Fatalf("owner mismatch persisted metadata = %#v", repo.goals[item.ID].Metadata)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if claimed == nil || readiness.ownerVerifyCalls != 1 ||
				readiness.verifiedExecutionID != "execution-legacy" ||
				readiness.verifiedGoalID != item.ID ||
				readiness.verifiedRevision != item.ObjectiveRevision() ||
				readiness.verifiedOwner != "owner-legacy" {
				t.Fatalf("confirmed proof = %#v claimed=%#v", readiness, claimed)
			}
		})
	}
}

func TestLegacyGoalClaimCoversOwnerControlPlaneMutations(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  protocol.GoalStatus
		mutate  func(context.Context, *Service, protocol.Goal) error
		deleted bool
	}{
		{
			name:   "REST update",
			status: protocol.GoalStatusActive,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				objective := "REST updated objective"
				_, err := service.Update(ctx, item.ID, protocol.UpdateGoalRequest{Objective: &objective, OwnerUserID: "owner-legacy"})
				return err
			},
		},
		{
			name:   "REST replace existing",
			status: protocol.GoalStatusActive,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.Create(ctx, protocol.CreateGoalRequest{
					SessionKey: item.SessionKey, Objective: "REST replacement", CreatedBy: "user",
					OwnerUserID: "owner-legacy", ReplaceExisting: true,
				})
				return err
			},
		},
		{
			name:   "REST pause",
			status: protocol.GoalStatusActive,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.Pause(ctx, item.ID)
				return err
			},
		},
		{
			name:   "REST resume",
			status: protocol.GoalStatusPaused,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.Resume(ctx, item.ID)
				return err
			},
		},
		{
			name:    "REST clear",
			status:  protocol.GoalStatusActive,
			deleted: true,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.Clear(ctx, item.ID)
				return err
			},
		},
		{
			name:   "app-server set",
			status: protocol.GoalStatusActive,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				objective := "app-server replacement"
				_, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
					ThreadID: item.SessionKey, Objective: &objective, OwnerUserID: "owner-legacy",
				})
				return err
			},
		},
		{
			name:    "app-server clear",
			status:  protocol.GoalStatusActive,
			deleted: true,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.ClearFromThreadGoalParams(ctx, goalappserver.ThreadGoalClearParams{
					ThreadID: item.SessionKey, OwnerUserID: "owner-legacy",
				})
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repo, verifier, readiness, ctx, item := newOwnerClaimService(
				t,
				test.status,
				protocol.GoalExecutionBindingStateStandalone,
			)
			if err := test.mutate(ctx, service, item); err != nil {
				t.Fatal(err)
			}
			if len(verifier.requests) == 0 || readiness.resolveCalls == 0 {
				t.Fatalf("session proofs=%d resolver calls=%d, want owner claim proof", len(verifier.requests), readiness.resolveCalls)
			}
			stored, exists := repo.goals[item.ID]
			if test.deleted {
				if exists {
					t.Fatalf("Goal still exists after clear: %#v", stored)
				}
				return
			}
			if !exists {
				t.Fatal("Goal missing after mutation")
			}
			if got := protocol.GoalMetadataString(stored.Metadata, protocol.GoalMetadataOwnerUserID); got != "owner-legacy" {
				t.Fatalf("persisted owner = %q, want owner-legacy", got)
			}
		})
	}
}

func TestForeignOwnerUpdateAndResumeAuthorizeBeforeSideEffects(t *testing.T) {
	const (
		goalOwner    = "owner-victim"
		foreignOwner = "owner-attacker"
	)
	for _, test := range []struct {
		name   string
		status protocol.GoalStatus
		mutate func(context.Context, *Service, protocol.Goal) error
	}{
		{
			name:   "REST Update",
			status: protocol.GoalStatusActive,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				objective := "attacker objective"
				_, err := service.Update(ctx, item.ID, protocol.UpdateGoalRequest{
					Objective:   &objective,
					OwnerUserID: foreignOwner,
				})
				return err
			},
		},
		{
			name:   "REST Resume",
			status: protocol.GoalStatusPaused,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.Resume(ctx, item.ID)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			item := protocol.Goal{
				ID:         "goal-victim",
				SessionKey: "agent:victim-agent:nexus:dm:victim-thread",
				Objective:  "victim objective",
				Status:     test.status,
				Version:    1,
				Metadata: map[string]any{
					protocol.GoalMetadataOwnerUserID: goalOwner,
				},
			}
			repo.goals[item.ID] = item
			service := NewService(config.Config{
				GoalEnabled:             true,
				GoalAutoContinueEnabled: true,
			}, repo)
			accountant := &fakeExternalMutationAccountant{
				service: service,
				usage:   protocol.GoalUsage{TotalTokens: 17},
				roundID: "round-attacker",
			}
			service.SetExternalMutationAccountant(accountant)
			dispatcher := &fakeContinuationDispatcher{}
			service.SetContinuationDispatcher(dispatcher)
			ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
				UserID: foreignOwner,
				Role:   authctx.RoleOwner,
			})

			err := test.mutate(ctx, service, item)
			if !errors.Is(err, ErrGoalForbidden) {
				t.Fatalf("mutation error = %v, want ErrGoalForbidden", err)
			}
			stored := repo.goals[item.ID]
			if len(accountant.sessionKeys) != 0 {
				t.Fatalf("accounting flushes = %#v, want none", accountant.sessionKeys)
			}
			if stored.Usage != (protocol.GoalUsage{}) || stored.Version != item.Version ||
				stored.Status != item.Status || stored.Objective != item.Objective {
				t.Fatalf("stored Goal = %#v, want unchanged %#v", stored, item)
			}
			if len(repo.events) != 0 {
				t.Fatalf("events = %#v, want none", repo.events)
			}
			if len(dispatcher.plans) != 0 || dispatcher.deferCalls != 0 {
				t.Fatalf("dispatcher plans=%d deferCalls=%d, want no calls", len(dispatcher.plans), dispatcher.deferCalls)
			}
		})
	}
}

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
