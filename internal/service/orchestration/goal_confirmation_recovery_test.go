package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestReconcileGoalConfirmationsSurvivesServiceRestartAndIsIdempotent(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.GoalID = "goal-recovery"
	snapshot.Execution.GoalObjectiveRevision = 4
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	store := &goalConfirmationRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		receipt: orchestrationstore.GoalConfirmationReceipt{
			ExecutionID:           snapshot.Execution.ID,
			GoalID:                snapshot.Execution.GoalID,
			GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
			CompletionCriteria:    []string{"verified"},
			State:                 orchestrationstore.GoalConfirmationPending,
			Version:               1,
			NextAttemptAt:         timePointer(time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)),
		},
	}
	firstGateway := &confirmingGoalBindingGateway{
		confirmErrOnce: errors.New("Goal repository temporarily unavailable"),
	}
	first := testService(store)
	first.now = func() time.Time {
		return time.Date(2026, 8, 11, 8, 1, 0, 0, time.UTC)
	}
	first.SetExplicitGoalBindingGateway(firstGateway)
	result, err := first.ReconcileGoalConfirmations(context.Background(), 10)
	if err == nil {
		t.Fatal("first reconciliation hid the pending confirmation error")
	}
	if result.Scanned != 1 || result.Pending != 1 || result.Confirmed != 0 ||
		store.receipt.State != orchestrationstore.GoalConfirmationPending ||
		store.receipt.AttemptCount != 1 || store.receipt.LastError == "" {
		t.Fatalf("first recovery: result=%#v receipt=%#v", result, store.receipt)
	}

	// A new Service value has no request or proposal memory. The durable store
	// alone is sufficient to finish the exact confirmation.
	secondGateway := &confirmingGoalBindingGateway{}
	second := testService(store)
	second.now = func() time.Time {
		return time.Date(2026, 8, 11, 8, 2, 0, 0, time.UTC)
	}
	second.SetExplicitGoalBindingGateway(secondGateway)
	sink := &capturingExecutionInvalidationSink{}
	second.SetExecutionInvalidationSink(sink)
	result, err = second.ReconcileGoalConfirmations(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Confirmed != 1 || result.Pending != 0 ||
		store.receipt.State != orchestrationstore.GoalConfirmationConfirmed ||
		store.receipt.ConfirmedAt == nil || secondGateway.confirmCalls != 1 ||
		len(sink.invalidations) != 1 ||
		sink.invalidations[0].ExecutionID != snapshot.Execution.ID {
		t.Fatalf("restart recovery: result=%#v receipt=%#v gateway=%#v", result, store.receipt, secondGateway)
	}

	result, err = second.ReconcileGoalConfirmations(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 0 || secondGateway.confirmCalls != 1 {
		t.Fatalf("confirmed receipt replayed: result=%#v calls=%d", result, secondGateway.confirmCalls)
	}
}

func TestPromoteExecutionReturnsAppliedPendingAfterDurableBind(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	var store *goalConfirmationRepositoryFake
	base := &fakeRepository{snapshot: snapshot}
	store = &goalConfirmationRepositoryFake{fakeRepository: base}
	base.bindGoal = func(
		_ context.Context,
		command orchestrationstore.BindGoalCommand,
	) (*protocol.ExecutionSnapshot, error) {
		updated := cloneExecutionSnapshot(snapshot)
		updated.Execution.Version++
		updated.Execution.GoalID = command.Execution.GoalID
		updated.Execution.GoalObjectiveRevision = command.Execution.GoalObjectiveRevision
		updated.Execution.GoalActivationOrigin = command.Execution.GoalActivationOrigin
		updated.Execution.GoalActivationReason = command.Execution.GoalActivationReason
		base.snapshot = updated
		store.receipt = orchestrationstore.GoalConfirmationReceipt{
			ExecutionID:           updated.Execution.ID,
			GoalID:                updated.Execution.GoalID,
			GoalObjectiveRevision: updated.Execution.GoalObjectiveRevision,
			CompletionCriteria:    append([]string(nil), updated.Execution.CompletionCriteria...),
			State:                 orchestrationstore.GoalConfirmationPending,
			Version:               1,
			NextAttemptAt:         timePointer(time.Now().UTC()),
		}
		return updated, nil
	}
	service := testService(store)
	service.SetGoalPromotionGateway(goalPromotionGatewayFunc(func(
		context.Context,
		GoalPromotionRequest,
	) (GoalPromotionBinding, error) {
		return GoalPromotionBinding{
			GoalID:                "goal-pending",
			GoalObjectiveRevision: 1,
			ActivationOrigin:      protocol.GoalActivationOriginAdaptivePromoted,
			ActivationReason:      protocol.GoalActivationReasonObservedBoundary,
		}, nil
	}))
	service.SetExplicitGoalBindingGateway(&confirmingGoalBindingGateway{
		confirmErrOnce: errors.New("confirmation transport interrupted"),
	})
	result, err := service.PromoteExecutionToGoal(
		context.Background(),
		coordinatorActor(),
		PromoteExecutionToGoalInput{
			ExecutionID:       snapshot.Execution.ID,
			SnapshotRevision:  snapshot.Execution.Version,
			CommandID:         "promotion-pending",
			ObjectiveProposal: snapshot.Execution.Objective,
			ActivationReason:  protocol.GoalActivationReasonObservedBoundary,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		result.GoalConfirmation != GoalConfirmationPending ||
		result.GoalAuthority != nil || result.Snapshot == nil ||
		result.Snapshot.Execution.GoalID != "goal-pending" ||
		len(result.NextActions) == 0 ||
		result.NextActions[len(result.NextActions)-1].Tool != "promote_execution_to_goal" {
		t.Fatalf("durable pending promotion result = %#v", result)
	}
}

type goalConfirmationRepositoryFake struct {
	*fakeRepository
	receipt orchestrationstore.GoalConfirmationReceipt
}

func (f *goalConfirmationRepositoryFake) EnsureGoalConfirmationReceipt(
	context.Context,
	string,
) (*orchestrationstore.GoalConfirmationReceipt, error) {
	item := f.receipt
	return &item, nil
}

func (f *goalConfirmationRepositoryFake) GetGoalConfirmationReceipt(
	context.Context,
	string,
) (*orchestrationstore.GoalConfirmationReceipt, error) {
	item := f.receipt
	return &item, nil
}

func (f *goalConfirmationRepositoryFake) ListRecoverableGoalConfirmations(
	context.Context,
	orchestrationstore.ListRecoverableGoalConfirmationsQuery,
) ([]orchestrationstore.GoalConfirmationReceipt, error) {
	if f.receipt.State != orchestrationstore.GoalConfirmationPending {
		return nil, nil
	}
	return []orchestrationstore.GoalConfirmationReceipt{f.receipt}, nil
}

func (f *goalConfirmationRepositoryFake) MarkGoalConfirmationConfirmed(
	_ context.Context,
	_ orchestrationstore.MarkGoalConfirmationCommand,
) (*orchestrationstore.GoalConfirmationReceipt, error) {
	if f.receipt.State == orchestrationstore.GoalConfirmationConfirmed {
		item := f.receipt
		return &item, nil
	}
	f.receipt.State = orchestrationstore.GoalConfirmationConfirmed
	f.receipt.Version++
	f.receipt.AttemptCount++
	f.receipt.NextAttemptAt = nil
	f.receipt.LastError = ""
	f.receipt.ConfirmedAt = timePointer(time.Now().UTC())
	item := f.receipt
	return &item, nil
}

func (f *goalConfirmationRepositoryFake) ScheduleGoalConfirmationRetry(
	_ context.Context,
	command orchestrationstore.MarkGoalConfirmationCommand,
) (*orchestrationstore.GoalConfirmationReceipt, error) {
	if f.receipt.State == orchestrationstore.GoalConfirmationConfirmed {
		item := f.receipt
		return &item, nil
	}
	f.receipt.Version++
	f.receipt.AttemptCount++
	f.receipt.NextAttemptAt = command.NextAttemptAt
	f.receipt.LastError = command.LastError
	item := f.receipt
	return &item, nil
}

func timePointer(value time.Time) *time.Time {
	return &value
}
