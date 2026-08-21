// INPUT: sealed proposal fixtures, durable active binding, access fences, stale versions and simulated restart deadlines.
// OUTPUT: deterministic bind/create, exact CAS receipts, supersede isolation and recovery behavior proofs.
// POS: persistent ExecutionPlanProposal aggregate and host-owned selection behavior tests independent of model input.
package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestPlanProposalBindingSupersedesSealedProposalWithoutAllowingLateReplay(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	first := transientPlanProposal("binding-first")
	createdFirst, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: first},
	)
	if err != nil {
		t.Fatal(err)
	}
	access := planProposalBindingAccessFor(*createdFirst)
	bound, err := repository.GetBoundPlanProposal(
		ctx,
		GetBoundPlanProposalQuery{Access: access},
	)
	if err != nil || bound == nil || bound.ID != createdFirst.ID {
		t.Fatalf("initial binding = %#v err=%v", bound, err)
	}
	sameRound := transientPlanProposal("binding-same-round")
	sameRound.SessionKey = first.SessionKey
	sameRound.RootRoundID = first.RootRoundID
	sameRound.RuntimeRoundID = first.RuntimeRoundID
	sameRound.AgentRoundID = first.AgentRoundID
	if _, err = repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: sameRound},
	); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("same-round second proposal error = %v, want ErrCommandConflict", err)
	}
	bound, err = repository.GetBoundPlanProposal(
		ctx,
		GetBoundPlanProposalQuery{Access: access},
	)
	if err != nil || bound == nil || bound.ID != createdFirst.ID {
		t.Fatalf("same-round conflict changed binding = %#v err=%v", bound, err)
	}

	second := transientPlanProposal("binding-second")
	second.SessionKey = first.SessionKey
	createdSecond, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: second},
	)
	if err != nil {
		t.Fatal(err)
	}
	bound, err = repository.GetBoundPlanProposal(
		ctx,
		GetBoundPlanProposalQuery{Access: access},
	)
	if err != nil || bound == nil || bound.ID != createdSecond.ID {
		t.Fatalf("superseding binding = %#v err=%v", bound, err)
	}
	discarded, err := repository.GetPlanProposal(
		ctx,
		GetPlanProposalQuery{Access: planProposalAccessFor(*createdFirst)},
	)
	if err != nil || discarded == nil ||
		discarded.Status != protocol.ExecutionPlanProposalStatusDiscarded ||
		discarded.Version != createdFirst.Version+1 {
		t.Fatalf("superseded proposal = %#v err=%v", discarded, err)
	}
	if _, err = repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: first},
	); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("late replay error = %v, want ErrCommandConflict", err)
	}
	bound, err = repository.GetBoundPlanProposal(
		ctx,
		GetBoundPlanProposalQuery{Access: access},
	)
	if err != nil || bound == nil || bound.ID != createdSecond.ID {
		t.Fatalf("late replay changed binding = %#v err=%v", bound, err)
	}
}

func TestPlanProposalBindingCannotSupersedeMaterializingProposal(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	created, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: transientPlanProposal("binding-materializing")},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.MarkPlanProposalMaterializing(
		ctx,
		MarkPlanProposalMaterializingCommand{
			Access:                   planProposalAccessFor(*created),
			ExpectedVersion:          created.Version,
			ReservedExecutionID:      "execution-binding-materializing",
			MaterializationCommandID: "materialize-binding-materializing",
			NextAttemptAt:            proposalTestFutureTime(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	tooLate := transientPlanProposal("binding-too-late")
	tooLate.SessionKey = created.SessionKey
	if _, err = repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: tooLate},
	); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("materializing supersede error = %v, want ErrCommandConflict", err)
	}
	bound, err := repository.GetBoundPlanProposal(
		ctx,
		GetBoundPlanProposalQuery{Access: planProposalBindingAccessFor(*created)},
	)
	if err != nil || bound == nil || bound.ID != created.ID ||
		bound.Status != protocol.ExecutionPlanProposalStatusMaterializing {
		t.Fatalf("materializing binding = %#v err=%v", bound, err)
	}
}

func TestPlanProposalLifecycleRecoveryAndExactReplay(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	proposal := goalPlanProposal("lifecycle")
	created, err := repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{Proposal: proposal})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != protocol.ExecutionPlanProposalStatusSealed || created.Version != 1 ||
		created.ContentDigest == "" {
		t.Fatalf("created proposal = %#v", created)
	}
	if created.Document.Items[1].DependsOn[0].Kind != protocol.WorkDependencyHard {
		t.Fatalf("default dependency kind = %q, want hard", created.Document.Items[1].DependsOn[0].Kind)
	}
	digest := created.ContentDigest
	replayed, err := repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{Proposal: proposal})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != created.ID || replayed.Version != 1 || replayed.ContentDigest != digest {
		t.Fatalf("create replay = %#v", replayed)
	}

	wrong := planProposalAccessFor(*created)
	wrong.CoordinatorAgentID = "agent-intruder"
	if _, err = repository.GetPlanProposal(ctx, GetPlanProposalQuery{Access: wrong}); !errors.Is(err, ErrPlanProposalAccess) {
		t.Fatalf("wrong access error = %v, want ErrPlanProposalAccess", err)
	}

	materializationRetry := time.Date(2026, 7, 30, 13, 0, 0, 0, time.UTC)
	materializingCommand := MarkPlanProposalMaterializingCommand{
		Access:                   planProposalAccessFor(*created),
		ExpectedVersion:          1,
		ReservedExecutionID:      "execution-lifecycle",
		MaterializationCommandID: "materialize-lifecycle",
		GoalID:                   created.GoalID,
		GoalObjectiveRevision:    created.GoalObjectiveRevision,
		GoalActivationOrigin:     protocol.GoalActivationOriginUserExplicit,
		GoalActivationReason:     protocol.GoalActivationReasonPersistenceRequested,
		NextAttemptAt:            &materializationRetry,
	}
	materializing, err := repository.MarkPlanProposalMaterializing(ctx, materializingCommand)
	if err != nil {
		t.Fatal(err)
	}
	if materializing.Status != protocol.ExecutionPlanProposalStatusMaterializing ||
		materializing.Version != 2 || materializing.AttemptCount != 1 ||
		materializing.ConfirmationState != protocol.ExecutionPlanProposalConfirmationPending ||
		materializing.ContentDigest != digest {
		t.Fatalf("materializing proposal = %#v", materializing)
	}
	if _, err = repository.MarkPlanProposalMaterializing(
		ctx,
		materializingCommand,
	); !errors.Is(err, ErrPlanProposalNotDue) {
		t.Fatalf("materialization replay error = %v, want ErrPlanProposalNotDue", err)
	}
	stale := materializingCommand
	stale.MaterializationCommandID = "materialize-other"
	if _, err = repository.MarkPlanProposalMaterializing(ctx, stale); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale materialization error = %v, want ErrVersionConflict", err)
	}

	recoverable, err := repository.ListRecoverablePlanProposals(ctx, ListRecoverablePlanProposalsQuery{
		Now:   materializationRetry,
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(recoverable) != 1 || recoverable[0].ID != created.ID {
		t.Fatalf("recoverable materialization = %#v", recoverable)
	}

	confirmationRetry := materializationRetry.Add(time.Hour)
	materializedCommand := MarkPlanProposalMaterializedCommand{
		Access:                  planProposalAccessFor(*created),
		ExpectedVersion:         2,
		MaterializedExecutionID: "execution-lifecycle",
		MaterializedPlanID:      "plan-lifecycle",
		NextAttemptAt:           &confirmationRetry,
	}
	materialized, err := repository.MarkPlanProposalMaterialized(ctx, materializedCommand)
	if err != nil {
		t.Fatal(err)
	}
	if materialized.Status != protocol.ExecutionPlanProposalStatusMaterialized ||
		materialized.Version != 3 || materialized.MaterializedAt == nil ||
		materialized.ContentDigest != digest {
		t.Fatalf("materialized proposal = %#v", materialized)
	}
	recoverable, err = repository.ListRecoverablePlanProposals(ctx, ListRecoverablePlanProposalsQuery{
		Now:   confirmationRetry.Add(-time.Second),
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(recoverable) != 0 {
		t.Fatalf("early confirmation recovery = %#v", recoverable)
	}

	secondRetry := confirmationRetry.Add(time.Hour)
	pendingCommand := MarkPlanProposalConfirmationCommand{
		Access:            planProposalAccessFor(*created),
		ExpectedVersion:   3,
		ConfirmationState: protocol.ExecutionPlanProposalConfirmationPending,
		LastError:         "goal store temporarily unavailable",
		NextAttemptAt:     &secondRetry,
	}
	pending, err := repository.MarkPlanProposalConfirmation(ctx, pendingCommand)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Version != 4 || pending.AttemptCount != 2 || pending.LastError == "" {
		t.Fatalf("pending confirmation = %#v", pending)
	}
	recoverable, err = repository.ListRecoverablePlanProposals(ctx, ListRecoverablePlanProposalsQuery{
		Now:   secondRetry,
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(recoverable) != 1 || recoverable[0].ID != created.ID {
		t.Fatalf("recoverable confirmation = %#v", recoverable)
	}

	confirmedCommand := MarkPlanProposalConfirmationCommand{
		Access:            planProposalAccessFor(*created),
		ExpectedVersion:   4,
		ConfirmationState: protocol.ExecutionPlanProposalConfirmationConfirmed,
	}
	confirmed, err := repository.MarkPlanProposalConfirmation(ctx, confirmedCommand)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Version != 5 || confirmed.AttemptCount != 3 ||
		confirmed.ConfirmationState != protocol.ExecutionPlanProposalConfirmationConfirmed ||
		confirmed.NextAttemptAt != nil || confirmed.LastError != "" {
		t.Fatalf("confirmed proposal = %#v", confirmed)
	}
	replayed, err = repository.MarkPlanProposalConfirmation(ctx, confirmedCommand)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != confirmed.Version || replayed.AttemptCount != confirmed.AttemptCount {
		t.Fatalf("confirmation replay = %#v", replayed)
	}
	recoverable, err = repository.ListRecoverablePlanProposals(ctx, ListRecoverablePlanProposalsQuery{
		Now:   secondRetry.Add(time.Hour),
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(recoverable) != 0 {
		t.Fatalf("confirmed proposal remained recoverable = %#v", recoverable)
	}
}

func TestPlanProposalAllowsHardOrderedOutputScopeHandoff(t *testing.T) {
	proposal := transientPlanProposal("scope-handoff")
	proposal.Document.Items[0].OutputScopes = []protocol.WorkOutputScope{{
		Scope: "file:output/workgraph-demo.md",
	}}
	proposal.Document.Items[1].OutputScopes = []protocol.WorkOutputScope{{
		Scope: "file:output/workgraph-demo.md",
	}}
	if _, _, err := normalizeAndValidatePlanProposal(
		proposal,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("hard-ordered proposal handoff rejected: %v", err)
	}

	proposal.Document.Items[1].DependsOn[0].Kind = protocol.WorkDependencySoft
	if _, _, err := normalizeAndValidatePlanProposal(
		proposal,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
	); !errors.Is(err, ErrInvariant) {
		t.Fatalf("soft-only proposal overlap error = %v, want ErrInvariant", err)
	}
}

func TestPlanProposalGoalReservedExecutionIsImmutableAndEnforced(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	proposal := goalPlanProposal("goal-reserved")
	proposal.GoalReservedExecutionID = "execution-goal-reserved"
	proposal.ReplacesExecutionID = "execution-goal-predecessor"
	created, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: proposal},
	)
	if err != nil {
		t.Fatal(err)
	}
	if created.GoalReservedExecutionID != proposal.GoalReservedExecutionID {
		t.Fatalf("persisted Goal reservation = %q", created.GoalReservedExecutionID)
	}
	command := MarkPlanProposalMaterializingCommand{
		Access:                   planProposalAccessFor(*created),
		ExpectedVersion:          created.Version,
		ReservedExecutionID:      "execution-other",
		MaterializationCommandID: "materialize-goal-reserved",
		GoalID:                   created.GoalID,
		GoalObjectiveRevision:    created.GoalObjectiveRevision,
		GoalActivationOrigin:     created.GoalActivationOrigin,
		GoalActivationReason:     created.GoalActivationReason,
		ReplacesExecutionID:      created.ReplacesExecutionID,
		NextAttemptAt:            proposalTestFutureTime(),
	}
	if _, err = repository.MarkPlanProposalMaterializing(ctx, command); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("mismatched Goal reservation error = %v, want ErrCommandConflict", err)
	}
	command.ReservedExecutionID = proposal.GoalReservedExecutionID
	materializing, err := repository.MarkPlanProposalMaterializing(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if materializing.ReservedExecutionID != proposal.GoalReservedExecutionID {
		t.Fatalf("materializing reservation = %q", materializing.ReservedExecutionID)
	}
}

func TestPlanProposalMaterializationLeaseClaimsOnlyAfterDeadline(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	created, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: transientPlanProposal("lease")},
	)
	if err != nil {
		t.Fatal(err)
	}
	deadline := proposalTestFutureTime()
	materializing, err := repository.MarkPlanProposalMaterializing(
		ctx,
		MarkPlanProposalMaterializingCommand{
			Access:                   planProposalAccessFor(*created),
			ExpectedVersion:          created.Version,
			ReservedExecutionID:      "execution-lease",
			MaterializationCommandID: "materialize-lease",
			NextAttemptAt:            deadline,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	claim := ClaimPlanProposalMaterializingCommand{
		Access:          planProposalAccessFor(*created),
		ExpectedVersion: materializing.Version,
		ClaimAt:         deadline.Add(-time.Second),
		LeaseUntil:      deadline.Add(time.Minute),
	}
	if _, err = repository.ClaimPlanProposalMaterializing(ctx, claim); !errors.Is(err, ErrPlanProposalNotDue) {
		t.Fatalf("early claim error = %v, want ErrPlanProposalNotDue", err)
	}
	claim.ClaimAt = *deadline
	claimed, err := repository.ClaimPlanProposalMaterializing(ctx, claim)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Version != materializing.Version+1 || claimed.AttemptCount != 2 ||
		claimed.NextAttemptAt == nil || !claimed.NextAttemptAt.Equal(claim.LeaseUntil) {
		t.Fatalf("claimed proposal = %#v", claimed)
	}
}

func TestPlanProposalCreateConflictBlockAndDigestTamper(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	proposal := transientPlanProposal("block")
	created, err := repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{Proposal: proposal})
	if err != nil {
		t.Fatal(err)
	}
	conflict := proposal
	conflict.OwnerUserID = "owner-other"
	if _, err = repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{Proposal: conflict}); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("deterministic ID conflict error = %v, want ErrCommandConflict", err)
	}
	missingRound := transientPlanProposal("missing-round")
	missingRound.RootRoundID = ""
	if _, err = repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: missingRound},
	); !errors.Is(err, ErrInvariant) {
		t.Fatalf("missing root round error = %v, want ErrInvariant", err)
	}
	blockedCommand := MarkPlanProposalBlockedCommand{
		Access:          planProposalAccessFor(*created),
		ExpectedVersion: 1,
		LastError:       "target version fence is stale",
	}
	blocked, err := repository.MarkPlanProposalBlocked(ctx, blockedCommand)
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != protocol.ExecutionPlanProposalStatusBlocked || blocked.Version != 2 {
		t.Fatalf("blocked proposal = %#v", blocked)
	}
	replayed, err := repository.MarkPlanProposalBlocked(ctx, blockedCommand)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != blocked.Version {
		t.Fatalf("blocked replay = %#v", replayed)
	}

	optionalAcceptance := transientPlanProposal("optional-acceptance")
	optionalAcceptance.Document.Items[0].AcceptanceCriteria = nil
	optionalCreated, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: optionalAcceptance},
	)
	if err != nil {
		t.Fatalf("service-valid optional acceptance criteria: %v", err)
	}
	if len(optionalCreated.Document.Items[0].AcceptanceCriteria) != 0 {
		t.Fatalf("optional acceptance criteria = %#v", optionalCreated.Document.Items[0].AcceptanceCriteria)
	}

	tampered := transientPlanProposal("tamper")
	tamperedCreated, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: tampered},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.db.ExecContext(
		ctx,
		`UPDATE execution_plan_proposals SET owner_user_id = ? WHERE proposal_id = ?`,
		"owner-tampered",
		tamperedCreated.ID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.GetPlanProposal(ctx, GetPlanProposalQuery{
		Access: planProposalAccessFor(*tamperedCreated),
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("tampered immutable fence error = %v, want ErrInvariant", err)
	}

	unknownField := transientPlanProposal("unknown-field")
	unknownCreated, err := repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: unknownField},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.db.ExecContext(
		ctx,
		`UPDATE execution_plan_proposals
SET document_json = json_set(document_json, '$.unexpected_field', 'tampered')
WHERE proposal_id = ?`,
		unknownCreated.ID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.GetPlanProposal(ctx, GetPlanProposalQuery{
		Access: planProposalAccessFor(*unknownCreated),
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("unknown document field error = %v, want ErrInvariant", err)
	}
}

func TestPlanProposalScheduleMaterializationRetryCAS(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	created, err := repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{
		Proposal: transientPlanProposal("retry"),
	})
	if err != nil {
		t.Fatal(err)
	}
	initialRetry := time.Date(2026, 7, 30, 13, 0, 0, 0, time.UTC)
	materializingCommand := MarkPlanProposalMaterializingCommand{
		Access:                   planProposalAccessFor(*created),
		ExpectedVersion:          1,
		ReservedExecutionID:      "execution-retry",
		MaterializationCommandID: "materialize-retry",
		NextAttemptAt:            &initialRetry,
	}
	materializing, err := repository.MarkPlanProposalMaterializing(
		ctx,
		materializingCommand,
	)
	if err != nil {
		t.Fatal(err)
	}
	nextRetry := initialRetry.Add(time.Hour + 123456789*time.Nanosecond)
	normalizedNextRetry := nextRetry.UTC().Truncate(time.Microsecond)
	retryCommand := SchedulePlanProposalRetryCommand{
		Access:          planProposalAccessFor(*created),
		ExpectedVersion: materializing.Version,
		LastError:       "authoritative store temporarily unavailable",
		NextAttemptAt:   &nextRetry,
	}
	scheduled, err := repository.SchedulePlanProposalRetry(ctx, retryCommand)
	if err != nil {
		t.Fatal(err)
	}
	if scheduled.Status != protocol.ExecutionPlanProposalStatusMaterializing ||
		scheduled.Version != 3 || scheduled.AttemptCount != 2 ||
		scheduled.LastError != retryCommand.LastError ||
		!equalTimePointers(scheduled.NextAttemptAt, &normalizedNextRetry) {
		t.Fatalf("scheduled retry = %#v", scheduled)
	}
	replayed, err := repository.SchedulePlanProposalRetry(ctx, retryCommand)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != scheduled.Version || replayed.AttemptCount != scheduled.AttemptCount {
		t.Fatalf("retry replay = %#v", replayed)
	}
	stale := retryCommand
	stale.LastError = "different failure"
	if _, err = repository.SchedulePlanProposalRetry(ctx, stale); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale retry error = %v, want ErrVersionConflict", err)
	}
	for _, test := range []struct {
		name  string
		now   time.Time
		count int
	}{
		{name: "before backoff", now: normalizedNextRetry.Add(-time.Second), count: 0},
		{name: "at backoff", now: normalizedNextRetry, count: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			recoverable, listErr := repository.ListRecoverablePlanProposals(
				ctx,
				ListRecoverablePlanProposalsQuery{Now: test.now, Limit: 10},
			)
			if listErr != nil {
				t.Fatal(listErr)
			}
			if len(recoverable) != test.count {
				t.Fatalf("recoverable = %#v, want count %d", recoverable, test.count)
			}
		})
	}
	blocked, err := repository.MarkPlanProposalBlocked(ctx, MarkPlanProposalBlockedCommand{
		Access:          planProposalAccessFor(*created),
		ExpectedVersion: scheduled.Version,
		LastError:       "sealed target fence became stale",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.MarkPlanProposalMaterializing(
		ctx,
		materializingCommand,
	); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("blocked materialization replay error = %v, want ErrVersionConflict", err)
	}
	if blocked.Status != protocol.ExecutionPlanProposalStatusBlocked {
		t.Fatalf("blocked proposal = %#v", blocked)
	}
}

func TestBlockedPlanProposalConvergesOnlyThroughExactCommandReceipt(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	const suffix = "blocked-receipt"
	const commandID = "materialize-blocked-receipt"

	created, err := repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{
		Proposal: transientPlanProposal(suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	materializing, err := repository.MarkPlanProposalMaterializing(
		ctx,
		MarkPlanProposalMaterializingCommand{
			Access:                   planProposalAccessFor(*created),
			ExpectedVersion:          created.Version,
			ReservedExecutionID:      "execution-" + suffix,
			MaterializationCommandID: commandID,
			NextAttemptAt:            proposalTestFutureTime(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := repository.MarkPlanProposalBlocked(ctx, MarkPlanProposalBlockedCommand{
		Access:          planProposalAccessFor(*created),
		ExpectedVersion: materializing.Version,
		LastError:       "concurrent target check observed a stale fence",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.MarkPlanProposalMaterialized(ctx, MarkPlanProposalMaterializedCommand{
		Access:                  planProposalAccessFor(*created),
		ExpectedVersion:         blocked.Version,
		MaterializedExecutionID: "execution-" + suffix,
		MaterializedPlanID:      "plan-" + suffix,
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("blocked proposal without receipt error = %v, want ErrInvariant", err)
	}

	create := createTestCommand(suffix)
	create.Meta.CommandID = commandID
	create.Meta.EventID = "event-" + commandID
	plan := testPlanCommand(suffix, 1, suffix, "", 1)
	plan.Meta.CommandID = commandID + ":plan"
	plan.Meta.EventID = "event-" + commandID + "-plan"
	if _, err = repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: create.Execution,
		Plan:      plan,
		Meta:      create.Meta,
	}); err != nil {
		t.Fatal(err)
	}

	recoverable, err := repository.ListRecoverablePlanProposals(
		ctx,
		ListRecoverablePlanProposalsQuery{Now: time.Now().UTC(), Limit: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(recoverable) != 1 || recoverable[0].ID != created.ID ||
		recoverable[0].Status != protocol.ExecutionPlanProposalStatusBlocked {
		t.Fatalf("receipt-proven blocked recovery = %#v", recoverable)
	}
	materialized, err := repository.MarkPlanProposalMaterialized(
		ctx,
		MarkPlanProposalMaterializedCommand{
			Access:                  planProposalAccessFor(*created),
			ExpectedVersion:         blocked.Version,
			MaterializedExecutionID: "execution-" + suffix,
			MaterializedPlanID:      "plan-" + suffix,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if materialized.Status != protocol.ExecutionPlanProposalStatusMaterialized ||
		materialized.MaterializedExecutionID != "execution-"+suffix ||
		materialized.MaterializedPlanID != "plan-"+suffix {
		t.Fatalf("converged proposal = %#v", materialized)
	}
}

func TestPlanProposalReplanAndReplaceExactFences(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()

	replan := transientPlanProposal("replan")
	replan.TargetExecutionID = "execution-current"
	replan.TargetExecutionVersion = 7
	replan.BasePlanID = "plan-current"
	replan.Document.Operation = protocol.ExecutionPlanProposalReplan
	replan.Document.RevisionReason = "add verification evidence"
	replan.Document.Items[0].ExistingWorkItemID = "work-produce"
	replan.Document.Items[1].ExistingWorkItemID = "work-verify"
	createdReplan, err := repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{Proposal: replan})
	if err != nil {
		t.Fatal(err)
	}
	materializingReplan, err := repository.MarkPlanProposalMaterializing(
		ctx,
		MarkPlanProposalMaterializingCommand{
			Access:                   planProposalAccessFor(*createdReplan),
			ExpectedVersion:          createdReplan.Version,
			ReservedExecutionID:      replan.TargetExecutionID,
			MaterializationCommandID: "materialize-replan",
			NextAttemptAt:            proposalTestFutureTime(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.MarkPlanProposalMaterialized(
		ctx,
		MarkPlanProposalMaterializedCommand{
			Access:                  planProposalAccessFor(*createdReplan),
			ExpectedVersion:         materializingReplan.Version,
			MaterializedExecutionID: replan.TargetExecutionID,
			MaterializedPlanID:      "plan-replanned",
		},
	); err != nil {
		t.Fatal(err)
	}

	firstPlan := replan
	firstPlan.Document.Items = append([]protocol.ExecutionPlanProposalItem(nil), replan.Document.Items...)
	firstPlan.ID = "proposal-replan-first-plan"
	firstPlan.BasePlanID = ""
	firstPlan.Document.Items[0].ExistingWorkItemID = ""
	firstPlan.Document.Items[1].ExistingWorkItemID = ""
	if _, err = repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: firstPlan},
	); err != nil {
		t.Fatalf("first Plan for existing Execution: %v", err)
	}
	duplicateExisting := replan
	duplicateExisting.ID = "proposal-replan-duplicate-existing"
	duplicateExisting.Document.Items[1].ExistingWorkItemID = "work-produce"
	if _, err = repository.CreateOrGetPlanProposal(
		ctx,
		CreateOrGetPlanProposalCommand{Proposal: duplicateExisting},
	); !errors.Is(err, ErrInvariant) {
		t.Fatalf("duplicate existing Work Item error = %v, want ErrInvariant", err)
	}

	replace := transientPlanProposal("replace")
	replace.TargetExecutionID = "execution-old"
	replace.TargetExecutionVersion = 9
	replace.BasePlanID = "plan-old"
	replace.ReplacesExecutionID = replace.TargetExecutionID
	replace.Document.Operation = protocol.ExecutionPlanProposalReplace
	replace.Document.ReplacementReason = "objective boundary changed"
	createdReplace, err := repository.CreateOrGetPlanProposal(ctx, CreateOrGetPlanProposalCommand{Proposal: replace})
	if err != nil {
		t.Fatal(err)
	}
	materializingReplace, err := repository.MarkPlanProposalMaterializing(
		ctx,
		MarkPlanProposalMaterializingCommand{
			Access:                   planProposalAccessFor(*createdReplace),
			ExpectedVersion:          createdReplace.Version,
			ReservedExecutionID:      "execution-successor",
			MaterializationCommandID: "materialize-replace",
			ReplacesExecutionID:      replace.TargetExecutionID,
			NextAttemptAt:            proposalTestFutureTime(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	materializedReplace, err := repository.MarkPlanProposalMaterialized(
		ctx,
		MarkPlanProposalMaterializedCommand{
			Access:                  planProposalAccessFor(*createdReplace),
			ExpectedVersion:         materializingReplace.Version,
			MaterializedExecutionID: "execution-successor",
			MaterializedPlanID:      "plan-successor",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if materializedReplace.ReplacesExecutionID != replace.TargetExecutionID ||
		materializedReplace.ContentDigest != createdReplace.ContentDigest {
		t.Fatalf("replace receipt = %#v", materializedReplace)
	}
}

func goalPlanProposal(suffix string) protocol.ExecutionPlanProposal {
	item := transientPlanProposal(suffix)
	item.GoalID = "goal-" + suffix
	item.GoalObjectiveRevision = 3
	item.GoalActivationOrigin = protocol.GoalActivationOriginUserExplicit
	item.GoalActivationReason = protocol.GoalActivationReasonPersistenceRequested
	return item
}

func transientPlanProposal(suffix string) protocol.ExecutionPlanProposal {
	return protocol.ExecutionPlanProposal{
		ID:                 "proposal-" + suffix,
		OwnerUserID:        "owner-1",
		SessionKey:         "agent:nexus:workspace:dm:" + suffix,
		ScopeKind:          protocol.ExecutionScopeDM,
		CoordinatorAgentID: "agent-lead",
		RootRoundID:        "round-" + suffix,
		Document: protocol.ExecutionPlanProposalDocument{
			Version:            protocol.ExecutionPlanProposalDocumentVersion,
			Operation:          protocol.ExecutionPlanProposalCreate,
			Objective:          "produce and verify " + suffix,
			CompletionCriteria: []string{"report exists", "report is verified"},
			Items: []protocol.ExecutionPlanProposalItem{
				{
					LogicalKey:         "produce",
					Kind:               protocol.WorkItemKindProduce,
					Subject:            "Produce report",
					Objective:          "Write a small report",
					Deliverable:        "report.md",
					AcceptanceCriteria: []string{"report has a summary"},
					Required:           true,
					OutputScopes: []protocol.WorkOutputScope{
						{Scope: "file:report.md", Mode: protocol.WorkOutputScopeShared},
					},
				},
				{
					LogicalKey:         "verify",
					Kind:               protocol.WorkItemKindVerify,
					Subject:            "Verify report",
					Objective:          "Check the report",
					Deliverable:        "verification result",
					AcceptanceCriteria: []string{"all criteria pass"},
					Required:           true,
					Terminal:           true,
					DependsOn: []protocol.ExecutionPlanProposalDependency{
						{LogicalKey: "produce"},
					},
					InputRefs: []string{"report.md"},
				},
			},
		},
	}
}

func proposalTestFutureTime() *time.Time {
	value := time.Date(2026, 7, 30, 18, 0, 0, 0, time.UTC)
	return &value
}
