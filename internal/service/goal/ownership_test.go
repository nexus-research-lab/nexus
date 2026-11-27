// ownership_test.go 覆盖 Goal 拥有者认领、授权副作用与会话归属测试。
package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
