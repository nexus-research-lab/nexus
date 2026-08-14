package orchestration

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestReconcileCompletionAuditsDefersBlockersAndCompletesAfterFreshWake(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Plan = &protocol.ExecutionPlanRevision{
		ID:          "plan-completion-recovery",
		ExecutionID: snapshot.Execution.ID,
		Revision:    1,
		Status:      protocol.PlanRevisionStatusActive,
	}
	snapshot.CompletionBlockers = []string{"work_item:work-2:required_not_accepted"}
	base := &fakeRepository{snapshot: snapshot}
	store := &completionAuditRepositoryFake{
		fakeRepository: base,
		receipt: orchestrationstore.CompletionAuditReceipt{
			ExecutionID:         snapshot.Execution.ID,
			TriggerAcceptanceID: "acceptance-work-1",
			State:               orchestrationstore.CompletionAuditPending,
			Version:             1,
			NextAttemptAt:       timePointer(time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)),
		},
	}
	service := testService(store)
	service.now = func() time.Time {
		return time.Date(2026, 8, 14, 8, 1, 0, 0, time.UTC)
	}
	result, err := service.ReconcileCompletionAudits(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Deferred != 1 || result.Completed != 0 ||
		store.receipt.AttemptCount != 1 || store.receipt.LastError == "" ||
		store.receipt.NextAttemptAt == nil {
		t.Fatalf("blocked recovery result=%#v receipt=%#v", result, store.receipt)
	}

	// A later accepted review atomically reawakens the same Execution receipt
	// with a fresh acceptance/version fence after the blockers have disappeared.
	snapshot.CompletionBlockers = nil
	store.receipt.TriggerAcceptanceID = "acceptance-work-2"
	store.receipt.Version++
	store.receipt.AttemptCount = 0
	store.receipt.NextAttemptAt = timePointer(service.currentTime())
	store.receipt.LastError = ""
	base.complete = func(
		_ context.Context,
		command orchestrationstore.CompleteCommand,
	) (*protocol.ExecutionSnapshot, error) {
		if command.ExpectedExecutionVersion != snapshot.Execution.Version ||
			command.Meta.ActorKind != protocol.ExecutionActorSystem ||
			command.Meta.CommandID != "completion-audit:acceptance-work-2" {
			t.Fatalf("completion recovery command = %#v", command)
		}
		completed := cloneExecutionSnapshot(snapshot)
		completed.Execution.Version++
		completed.Execution.Status = protocol.ExecutionStatusCompleted
		base.snapshot = completed
		store.receipt.State = orchestrationstore.CompletionAuditCompleted
		store.receipt.Version++
		store.receipt.NextAttemptAt = nil
		store.receipt.SettledAt = timePointer(service.currentTime())
		return completed, nil
	}
	result, err = service.ReconcileCompletionAudits(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Completed != 1 || result.Deferred != 0 ||
		base.snapshot.Execution.Status != protocol.ExecutionStatusCompleted ||
		store.receipt.State != orchestrationstore.CompletionAuditCompleted {
		t.Fatalf("ready recovery result=%#v receipt=%#v snapshot=%#v", result, store.receipt, base.snapshot)
	}
}

func TestCompletionAuditRetryBackoffIsBounded(t *testing.T) {
	if got := completionAuditRetryBackoff(0); got != 15*time.Second {
		t.Fatalf("first retry delay = %s", got)
	}
	if got := completionAuditRetryBackoff(3); got != 2*time.Minute {
		t.Fatalf("fourth retry delay = %s", got)
	}
	if got := completionAuditRetryBackoff(100); got != completionAuditRetryMaxDelay {
		t.Fatalf("bounded retry delay = %s", got)
	}
}

type completionAuditRepositoryFake struct {
	*fakeRepository
	receipt orchestrationstore.CompletionAuditReceipt
}

func (f *completionAuditRepositoryFake) ListRecoverableCompletionAudits(
	context.Context,
	orchestrationstore.ListRecoverableCompletionAuditsQuery,
) ([]orchestrationstore.CompletionAuditReceipt, error) {
	if f.receipt.State != orchestrationstore.CompletionAuditPending {
		return nil, nil
	}
	return []orchestrationstore.CompletionAuditReceipt{f.receipt}, nil
}

func (f *completionAuditRepositoryFake) ScheduleCompletionAuditRetry(
	_ context.Context,
	command orchestrationstore.TransitionCompletionAuditCommand,
) (*orchestrationstore.CompletionAuditReceipt, error) {
	if command.TriggerAcceptanceID != f.receipt.TriggerAcceptanceID ||
		command.ExpectedVersion != f.receipt.Version {
		return nil, orchestrationstore.ErrVersionConflict
	}
	f.receipt.Version++
	f.receipt.AttemptCount++
	f.receipt.NextAttemptAt = command.NextAttemptAt
	f.receipt.LastError = command.LastError
	item := f.receipt
	return &item, nil
}

func (f *completionAuditRepositoryFake) MarkCompletionAuditCompleted(
	_ context.Context,
	command orchestrationstore.TransitionCompletionAuditCommand,
) (*orchestrationstore.CompletionAuditReceipt, error) {
	if command.TriggerAcceptanceID != f.receipt.TriggerAcceptanceID ||
		command.ExpectedVersion != f.receipt.Version {
		return nil, orchestrationstore.ErrVersionConflict
	}
	f.receipt.State = orchestrationstore.CompletionAuditCompleted
	f.receipt.Version++
	f.receipt.NextAttemptAt = nil
	f.receipt.LastError = ""
	f.receipt.SettledAt = timePointer(time.Now().UTC())
	item := f.receipt
	return &item, nil
}

func (f *completionAuditRepositoryFake) MarkCompletionAuditDiscarded(
	_ context.Context,
	command orchestrationstore.TransitionCompletionAuditCommand,
) (*orchestrationstore.CompletionAuditReceipt, error) {
	if command.TriggerAcceptanceID != f.receipt.TriggerAcceptanceID ||
		command.ExpectedVersion != f.receipt.Version {
		return nil, orchestrationstore.ErrVersionConflict
	}
	f.receipt.State = orchestrationstore.CompletionAuditDiscarded
	f.receipt.Version++
	f.receipt.NextAttemptAt = nil
	f.receipt.LastError = command.LastError
	f.receipt.SettledAt = timePointer(time.Now().UTC())
	item := f.receipt
	return &item, nil
}
