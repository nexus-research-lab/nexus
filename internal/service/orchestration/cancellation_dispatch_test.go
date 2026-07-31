package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type cancellationOutboxFake struct {
	*fakeRepository
	item protocol.ExecutionCancellationDispatch
}

func (f *cancellationOutboxFake) ListAvailableCancellationDispatches(
	context.Context,
	int,
) ([]protocol.ExecutionCancellationDispatch, error) {
	if f.item.Status != protocol.ExecutionCancellationDispatchPending {
		return nil, nil
	}
	return []protocol.ExecutionCancellationDispatch{f.item}, nil
}

func (f *cancellationOutboxFake) ClaimCancellationDispatch(
	_ context.Context,
	id string,
	version int64,
	owner string,
	_ time.Duration,
) (*protocol.ExecutionCancellationDispatch, error) {
	if f.item.ID != id ||
		f.item.Version != version ||
		f.item.Status != protocol.ExecutionCancellationDispatchPending {
		return nil, errors.New("lease conflict")
	}
	f.item.Status = protocol.ExecutionCancellationDispatchClaimed
	f.item.Version++
	f.item.DeliveryAttempts++
	f.item.LeaseOwner = owner
	item := f.item
	return &item, nil
}

func (f *cancellationOutboxFake) ResolveCancellationDispatch(
	_ context.Context,
	id string,
	version int64,
	owner string,
	status protocol.ExecutionCancellationDispatchStatus,
	outcome protocol.ExecutionCancellationOutcome,
	limitationCode string,
	receipt string,
) (*protocol.ExecutionCancellationDispatch, error) {
	if f.item.ID != id ||
		f.item.Version != version ||
		f.item.LeaseOwner != owner ||
		f.item.Status != protocol.ExecutionCancellationDispatchClaimed {
		return nil, errors.New("lease conflict")
	}
	f.item.Status = status
	f.item.Outcome = outcome
	f.item.LimitationCode = limitationCode
	f.item.Receipt = receipt
	f.item.Version++
	f.item.LeaseOwner = ""
	item := f.item
	return &item, nil
}

func (f *cancellationOutboxFake) RetryCancellationDispatch(
	_ context.Context,
	id string,
	version int64,
	owner string,
	_ time.Time,
	cause string,
) (*protocol.ExecutionCancellationDispatch, error) {
	if f.item.ID != id ||
		f.item.Version != version ||
		f.item.LeaseOwner != owner ||
		f.item.Status != protocol.ExecutionCancellationDispatchClaimed {
		return nil, errors.New("lease conflict")
	}
	f.item.Status = protocol.ExecutionCancellationDispatchPending
	f.item.LastError = cause
	f.item.Version++
	f.item.LeaseOwner = ""
	item := f.item
	return &item, nil
}

type cancellationConsumerFunc func(
	context.Context,
	ExecutionCancellationDelivery,
) (ExecutionCancellationReceipt, error)

func (f cancellationConsumerFunc) DeliverExecutionCancellation(
	ctx context.Context,
	delivery ExecutionCancellationDelivery,
) (ExecutionCancellationReceipt, error) {
	return f(ctx, delivery)
}

func TestDispatchPendingCancellationsRetriesThenDeliversExactBinding(t *testing.T) {
	repository := &cancellationOutboxFake{
		fakeRepository: &fakeRepository{},
		item: protocol.ExecutionCancellationDispatch{
			ID:                "cancel-1",
			ExecutionID:       "execution-old",
			PlanID:            "plan-old",
			WorkItemID:        "work-old",
			SpecID:            "spec-old",
			AssignmentID:      "assignment-old",
			AttemptID:         "attempt-child",
			RuntimeAttemptID:  "attempt-root",
			DispatchID:        "dispatch-old",
			ExecutorKind:      protocol.AttemptExecutorSubagent,
			TargetKind:        protocol.ExecutionCancellationTargetRuntimeRound,
			TargetAgentID:     "agent-worker",
			ScopeSessionKey:   "agent:worker:ws:dm:scope",
			RuntimeSessionKey: "agent:worker:ws:dm:runtime",
			RuntimeRoundID:    "round-old",
			Status:            protocol.ExecutionCancellationDispatchPending,
			Reason:            "execution superseded",
			Version:           1,
		},
	}
	service := NewService(repository)
	calls := 0
	var captured ExecutionCancellationDelivery
	service.SetExecutionCancellationConsumer(cancellationConsumerFunc(
		func(
			_ context.Context,
			delivery ExecutionCancellationDelivery,
		) (ExecutionCancellationReceipt, error) {
			calls++
			captured = delivery
			if calls == 1 {
				return ExecutionCancellationReceipt{}, errors.New(
					"runtime temporarily unavailable",
				)
			}
			return ExecutionCancellationReceipt{
				Outcome:        protocol.ExecutionCancellationOutcomeLocalRoundCancelled,
				LimitationCode: "provider_interrupt_unsafe_shared_session",
				Detail:         "old local round cancelled",
			}, nil
		},
	))
	first, err := service.DispatchPendingCancellations(
		context.Background(),
		"worker-1",
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Claimed != 1 || first.Retried != 1 ||
		repository.item.Status != protocol.ExecutionCancellationDispatchPending ||
		repository.item.LastError == "" {
		t.Fatalf("first drain = %+v item=%+v", first, repository.item)
	}
	second, err := service.DispatchPendingCancellations(
		context.Background(),
		"worker-2",
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if second.Claimed != 1 || second.Delivered != 1 ||
		repository.item.Status != protocol.ExecutionCancellationDispatchDelivered ||
		repository.item.Outcome != protocol.ExecutionCancellationOutcomeLocalRoundCancelled ||
		repository.item.LimitationCode != "provider_interrupt_unsafe_shared_session" {
		t.Fatalf("second drain = %+v item=%+v", second, repository.item)
	}
	if captured.Binding.AttemptID != "attempt-child" ||
		captured.Binding.RuntimeAttemptID != "attempt-root" ||
		captured.Binding.RuntimeSessionKey != "agent:worker:ws:dm:runtime" ||
		captured.Binding.RuntimeRoundID != "round-old" ||
		captured.ExecutorKind != protocol.AttemptExecutorSubagent {
		t.Fatalf("captured delivery = %+v", captured)
	}
}

func TestDispatchPendingCancellationsRecordsExplicitNotStartedLimitation(t *testing.T) {
	repository := &cancellationOutboxFake{
		fakeRepository: &fakeRepository{},
		item: protocol.ExecutionCancellationDispatch{
			ID:             "cancel-pending",
			TargetKind:     protocol.ExecutionCancellationTargetNotStarted,
			Status:         protocol.ExecutionCancellationDispatchPending,
			LimitationCode: "attempt_not_started",
			Version:        1,
		},
	}
	service := NewService(repository)
	result, err := service.DispatchPendingCancellations(
		context.Background(),
		"worker-1",
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.NotRequired != 1 ||
		repository.item.Status != protocol.ExecutionCancellationDispatchNotRequired ||
		repository.item.Outcome != protocol.ExecutionCancellationOutcomeNotStarted ||
		repository.item.LimitationCode != "attempt_not_started" ||
		repository.item.Receipt != "Attempt never acquired a physical runtime target" {
		t.Fatalf("result=%+v item=%+v", result, repository.item)
	}
}

func TestDispatchPendingCancellationsPersistsRuntimeUnsupportedLimitation(t *testing.T) {
	repository := &cancellationOutboxFake{
		fakeRepository: &fakeRepository{},
		item: protocol.ExecutionCancellationDispatch{
			ID:                "cancel-unsupported",
			TargetKind:        protocol.ExecutionCancellationTargetRuntimeRound,
			RuntimeSessionKey: "agent:worker:ws:dm:runtime",
			RuntimeRoundID:    "round-old",
			Status:            protocol.ExecutionCancellationDispatchPending,
			Version:           1,
		},
	}
	service := NewService(repository)
	service.SetExecutionCancellationConsumer(cancellationConsumerFunc(
		func(
			context.Context,
			ExecutionCancellationDelivery,
		) (ExecutionCancellationReceipt, error) {
			return ExecutionCancellationReceipt{
				Outcome:        protocol.ExecutionCancellationOutcomeUnsupported,
				LimitationCode: "exact_local_cancel_unavailable",
				Detail:         "shared provider session has no exact local cancel",
			}, nil
		},
	))
	result, err := service.DispatchPendingCancellations(
		context.Background(),
		"worker-1",
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Unsupported != 1 ||
		repository.item.Status != protocol.ExecutionCancellationDispatchUnsupported ||
		repository.item.Outcome != protocol.ExecutionCancellationOutcomeUnsupported ||
		repository.item.LimitationCode != "exact_local_cancel_unavailable" {
		t.Fatalf("result=%+v item=%+v", result, repository.item)
	}
}
