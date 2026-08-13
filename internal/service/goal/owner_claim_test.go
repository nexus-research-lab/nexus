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

func TestOwnerScopedCurrentClaimsLegacyStandaloneGoalOnce(t *testing.T) {
	service, repo, verifier, readiness, ctx, item := newOwnerClaimService(
		t,
		protocol.GoalStatusActive,
		protocol.GoalExecutionBindingStateStandalone,
	)

	claimed, err := service.CurrentOptionalForOwner(ctx, item.SessionKey, "owner-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalMetadataString(claimed.Metadata, protocol.GoalMetadataOwnerUserID); got != "owner-legacy" {
		t.Fatalf("claimed owner = %q, want owner-legacy", got)
	}
	if claimed.Version != item.Version+1 {
		t.Fatalf("claimed version = %d, want %d", claimed.Version, item.Version+1)
	}
	if len(verifier.requests) != 1 || readiness.resolveCalls != 1 {
		t.Fatalf("session proofs=%d resolver calls=%d, want 1/1", len(verifier.requests), readiness.resolveCalls)
	}

	again, err := service.CurrentOptionalForOwner(ctx, item.SessionKey, "owner-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if again.Version != claimed.Version || len(verifier.requests) != 1 || readiness.resolveCalls != 1 {
		t.Fatalf("second read=%#v proofs=%d resolver calls=%d, want idempotent claim", again, len(verifier.requests), readiness.resolveCalls)
	}
	if got := protocol.GoalMetadataString(repo.goals[item.ID].Metadata, protocol.GoalMetadataOwnerUserID); got != "owner-legacy" {
		t.Fatalf("persisted owner = %q, want owner-legacy", got)
	}
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
