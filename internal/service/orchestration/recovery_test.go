// recovery_test.go 覆盖编排根恢复、命令恢复、完成审计与 Goal 确认恢复测试。
package orchestration

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationrepo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type recoveryConsumerFunc func(context.Context, protocol.Execution, string) error

func (f recoveryConsumerFunc) QueueExecutionRecovery(ctx context.Context, e protocol.Execution, key string) error {
	return f(ctx, e, key)
}

func TestRootRecoveryCASExhaustionIsNotSuccess(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Attempts = []protocol.WorkAttempt{{ID: "root", Status: protocol.WorkAttemptStatusRunning}}
	calls := 0
	service := testService(&fakeRepository{snapshot: snapshot, finishAttempt: func(context.Context, orchestrationrepo.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
		calls++
		return nil, orchestrationrepo.ErrVersionConflict
	}})
	_, err := service.finishRootAttempts(context.Background(), coordinatorActor(), snapshot, "", protocol.WorkAttemptStatusInterrupted, "host_restart", time.Time{})
	if !errors.Is(err, orchestrationrepo.ErrVersionConflict) || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestResumeMemberWorkWakesOnlyInterruptedOwner(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusInterrupted
	service := testService(&fakeRepository{snapshot: snapshot})
	calls := 0
	service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(context.Context, protocol.Execution, string) error { calls++; return nil }))
	for _, member := range []string{"other", "agent-worker"} {
		if err := service.ResumeMemberWork(context.Background(), "owner-1", "session-1", member); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("resume wake count = %d", calls)
	}
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
	if err := service.ResumeMemberWork(context.Background(), "owner-1", "session-1", "agent-worker"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("accepted member work restarted")
	}
}

func TestRoomTerminalRecoveryDistinguishesFailureAndExplicitStop(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  protocol.WorkAttemptStatus
		recover bool
		want    int
	}{
		{"failed", protocol.WorkAttemptStatusFailed, false, 1},
		{"unexpected interruption", protocol.WorkAttemptStatusInterrupted, true, 1},
		{"user stop", protocol.WorkAttemptStatusInterrupted, false, 0},
		{"finished", protocol.WorkAttemptStatusSucceeded, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := assignedExecutionSnapshot()
			snapshot.Execution.ScopeKind = protocol.ExecutionScopeRoom
			snapshot.Assignments[0].Strategy = protocol.AssignmentStrategySelf
			snapshot.Attempts[0].Status = protocol.WorkAttemptStatusRunning
			service := testService(&fakeRepository{snapshot: snapshot, finishAttempt: func(_ context.Context, c orchestrationrepo.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
				snapshot.Attempts[0] = c.Attempt
				return snapshot, nil
			}})
			calls := 0
			service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(context.Context, protocol.Execution, string) error { calls++; return nil }))
			actor := coordinatorActor()
			actor.AgentID = "agent-worker"
			actor.ScopeKind = protocol.ExecutionScopeRoom
			err := service.FinishRoomAttempt(context.Background(), actor, RoomAttemptTerminalInput{
				Binding: protocol.ExecutionWorkBinding{ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1", SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1"},
				Status:  tc.status, RecoverInterrupted: tc.recover,
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls != tc.want {
				t.Fatalf("wake count=%d want=%d", calls, tc.want)
			}
			marked, _ := snapshot.Attempts[0].Metadata["host_recovery_required"].(bool)
			if marked != (tc.want == 1) {
				t.Fatalf("durable wake marker=%v", marked)
			}
		})
	}
}

func TestRecoveredTerminalWorkClaimCannotBlockCoordinatorQueue(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	service := testService(&fakeRepository{snapshot: snapshot})
	binding := protocol.ExecutionWorkBinding{ExecutionID: snapshot.Execution.ID, AttemptID: snapshot.Attempts[0].ID}
	for _, status := range []protocol.WorkAttemptStatus{protocol.WorkAttemptStatusPending, protocol.WorkAttemptStatusRunning, protocol.WorkAttemptStatusInterrupted, protocol.WorkAttemptStatusSucceeded} {
		snapshot.Attempts[0].Status = status
		current, err := service.RecoveryWorkInputCurrent(context.Background(), "owner-1", "session-1", binding)
		if err != nil {
			t.Fatal(err)
		}
		if current != (status == protocol.WorkAttemptStatusPending || status == protocol.WorkAttemptStatusRunning) {
			t.Fatalf("status=%s current=%v", status, current)
		}
	}
}

// multiRecoveryRepository serves several Executions so startup recovery can be
// checked for per-Execution isolation.
type multiRecoveryRepository struct {
	*fakeRepository
	executions []protocol.Execution
	snapshots  map[string]*protocol.ExecutionSnapshot
	failing    map[string]error
}

func (r *multiRecoveryRepository) ListRecoveryExecutions(_ context.Context, after string, _ int) ([]protocol.Execution, error) {
	if after != "" {
		return nil, nil
	}
	return r.executions, nil
}

func (r *multiRecoveryRepository) GetSnapshot(_ context.Context, id string) (*protocol.ExecutionSnapshot, error) {
	if err := r.failing[id]; err != nil {
		return nil, err
	}
	return r.snapshots[id], nil
}

func TestRootRecoveryAuditAdvancesPastFailingExecution(t *testing.T) {
	repository := &multiRecoveryRepository{fakeRepository: &fakeRepository{}, snapshots: map[string]*protocol.ExecutionSnapshot{}, failing: map[string]error{}}
	for _, id := range []string{"execution-unreadable", "execution-failed-member"} {
		snapshot := executionSnapshot()
		snapshot.Execution.ID = id
		snapshot.Execution.Status = protocol.ExecutionStatusActive
		snapshot.Assignments = []protocol.WorkAssignment{{ID: "assignment", OwnerAgentID: "member", Status: protocol.WorkAssignmentStatusActive}}
		snapshot.Attempts = []protocol.WorkAttempt{{ID: "root-" + id, AssignmentID: "assignment", Status: protocol.WorkAttemptStatusFailed, Metadata: map[string]any{"host_recovery_required": true}}}
		repository.executions = append(repository.executions, snapshot.Execution)
		repository.snapshots[id] = snapshot
	}
	repository.failing["execution-unreadable"] = errors.New("snapshot read failed")
	service := NewService(repository)
	var queued []string
	service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(_ context.Context, e protocol.Execution, _ string) error {
		queued = append(queued, e.ID)
		return nil
	}))
	cursor := ""

	if err := service.reconcileRootRecoveryInputs(context.Background(), &cursor, 64); err == nil {
		t.Fatal("unreadable Execution was not reported")
	}
	if len(queued) != 1 || queued[0] != "execution-failed-member" {
		t.Fatalf("queued = %v, want the audit to reach Executions after a failing one", queued)
	}
}

func hostInterruptedSnapshot(id string, reason string) *protocol.ExecutionSnapshot {
	snapshot := executionSnapshot()
	snapshot.Execution.ID = id
	snapshot.Execution.Status = protocol.ExecutionStatusActive
	snapshot.Assignments = []protocol.WorkAssignment{{ID: "assignment-" + id, OwnerAgentID: "member", Status: protocol.WorkAssignmentStatusActive}}
	snapshot.Attempts = []protocol.WorkAttempt{{ID: "root-" + id, AssignmentID: "assignment-" + id, Status: protocol.WorkAttemptStatusInterrupted, FailureReason: reason}}
	return snapshot
}

func (r *multiRecoveryRepository) Get(_ context.Context, id string) (*protocol.Execution, error) {
	snapshot := r.snapshots[id]
	if snapshot == nil {
		return nil, nil
	}
	execution := snapshot.Execution
	return &execution, nil
}

// A transient startup failure is retried by the background coordinator, and the
// retry never settles a root that a live round created after this start.
func TestStartupRootRecoveryRetriesTransientFailureWithinCutoff(t *testing.T) {
	startedAt := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	repository := &multiRecoveryRepository{fakeRepository: &fakeRepository{}, snapshots: map[string]*protocol.ExecutionSnapshot{}, failing: map[string]error{}}
	snapshot := hostInterruptedSnapshot("execution-transient", hostShutdownAttemptReason)
	snapshot.Assignments = append(snapshot.Assignments, protocol.WorkAssignment{ID: "assignment-live", OwnerAgentID: "member-2", Status: protocol.WorkAssignmentStatusActive})
	snapshot.Attempts = append(snapshot.Attempts, protocol.WorkAttempt{
		ID: "root-live", AssignmentID: "assignment-live", Status: protocol.WorkAttemptStatusRunning,
		CreatedAt: startedAt.Add(time.Minute),
	})
	repository.executions = []protocol.Execution{snapshot.Execution}
	repository.snapshots[snapshot.Execution.ID] = snapshot
	repository.failing[snapshot.Execution.ID] = errors.New("database is locked")
	var settled []string
	repository.finishAttempt = func(_ context.Context, command orchestrationrepo.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
		settled = append(settled, command.Attempt.ID)
		return snapshot, nil
	}
	service := NewService(repository)
	service.now = func() time.Time { return startedAt }
	var queued []string
	service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(_ context.Context, e protocol.Execution, _ string) error {
		queued = append(queued, e.ID)
		return nil
	}))

	if err := service.RecoverRootAttempts(context.Background()); err == nil {
		t.Fatal("transient startup failure was not reported")
	}
	if len(queued) != 0 {
		t.Fatalf("queued = %v before the retry", queued)
	}

	delete(repository.failing, snapshot.Execution.ID)
	if err := service.retryStartupRootRecovery(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(queued, []string{"execution-transient"}) {
		t.Fatalf("queued = %v, want the retried Execution recovered", queued)
	}
	if len(settled) != 0 {
		t.Fatalf("settled = %v, want the live root created after startup left alone", settled)
	}
	if err := service.retryStartupRootRecovery(context.Background()); err != nil || len(queued) != 1 {
		t.Fatalf("retry after success queued again: %v %v", queued, err)
	}
}

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
		receipt: orchestrationrepo.CompletionAuditReceipt{
			ExecutionID:         snapshot.Execution.ID,
			TriggerAcceptanceID: "acceptance-work-1",
			State:               orchestrationrepo.CompletionAuditPending,
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
		command orchestrationrepo.CompleteCommand,
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
		store.receipt.State = orchestrationrepo.CompletionAuditCompleted
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
		store.receipt.State != orchestrationrepo.CompletionAuditCompleted {
		t.Fatalf("ready recovery result=%#v receipt=%#v snapshot=%#v", result, store.receipt, base.snapshot)
	}
}

type completionAuditRepositoryFake struct {
	*fakeRepository
	receipt orchestrationrepo.CompletionAuditReceipt
}

func (f *completionAuditRepositoryFake) ListRecoverableCompletionAudits(
	context.Context,
	orchestrationrepo.ListRecoverableCompletionAuditsQuery,
) ([]orchestrationrepo.CompletionAuditReceipt, error) {
	if f.receipt.State != orchestrationrepo.CompletionAuditPending {
		return nil, nil
	}
	return []orchestrationrepo.CompletionAuditReceipt{f.receipt}, nil
}

func (f *completionAuditRepositoryFake) ScheduleCompletionAuditRetry(
	_ context.Context,
	command orchestrationrepo.TransitionCompletionAuditCommand,
) (*orchestrationrepo.CompletionAuditReceipt, error) {
	if command.TriggerAcceptanceID != f.receipt.TriggerAcceptanceID ||
		command.ExpectedVersion != f.receipt.Version {
		return nil, orchestrationrepo.ErrVersionConflict
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
	command orchestrationrepo.TransitionCompletionAuditCommand,
) (*orchestrationrepo.CompletionAuditReceipt, error) {
	if command.TriggerAcceptanceID != f.receipt.TriggerAcceptanceID ||
		command.ExpectedVersion != f.receipt.Version {
		return nil, orchestrationrepo.ErrVersionConflict
	}
	f.receipt.State = orchestrationrepo.CompletionAuditCompleted
	f.receipt.Version++
	f.receipt.NextAttemptAt = nil
	f.receipt.LastError = ""
	f.receipt.SettledAt = timePointer(time.Now().UTC())
	item := f.receipt
	return &item, nil
}

func (f *completionAuditRepositoryFake) MarkCompletionAuditDiscarded(
	_ context.Context,
	command orchestrationrepo.TransitionCompletionAuditCommand,
) (*orchestrationrepo.CompletionAuditReceipt, error) {
	if command.TriggerAcceptanceID != f.receipt.TriggerAcceptanceID ||
		command.ExpectedVersion != f.receipt.Version {
		return nil, orchestrationrepo.ErrVersionConflict
	}
	f.receipt.State = orchestrationrepo.CompletionAuditDiscarded
	f.receipt.Version++
	f.receipt.NextAttemptAt = nil
	f.receipt.LastError = command.LastError
	f.receipt.SettledAt = timePointer(time.Now().UTC())
	item := f.receipt
	return &item, nil
}

type capturingExecutionInvalidationSink struct {
	invalidations []ExecutionInvalidation
}

func (s *capturingExecutionInvalidationSink) InvalidateExecution(
	_ context.Context,
	invalidation ExecutionInvalidation,
) {
	s.invalidations = append(s.invalidations, invalidation)
}

func TestReconcileGoalConfirmationsSurvivesServiceRestartAndIsIdempotent(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.GoalID = "goal-recovery"
	snapshot.Execution.GoalObjectiveRevision = 4
	snapshot.Execution.CompletionCriteria = []string{"verified"}
	store := &goalConfirmationRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		receipt: orchestrationrepo.GoalConfirmationReceipt{
			ExecutionID:           snapshot.Execution.ID,
			GoalID:                snapshot.Execution.GoalID,
			GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
			CompletionCriteria:    []string{"verified"},
			State:                 orchestrationrepo.GoalConfirmationPending,
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
		store.receipt.State != orchestrationrepo.GoalConfirmationPending ||
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
		store.receipt.State != orchestrationrepo.GoalConfirmationConfirmed ||
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

type goalConfirmationRepositoryFake struct {
	*fakeRepository
	receipt orchestrationrepo.GoalConfirmationReceipt
}

func (f *goalConfirmationRepositoryFake) EnsureGoalConfirmationReceipt(
	context.Context,
	string,
) (*orchestrationrepo.GoalConfirmationReceipt, error) {
	item := f.receipt
	return &item, nil
}

func (f *goalConfirmationRepositoryFake) GetGoalConfirmationReceipt(
	context.Context,
	string,
) (*orchestrationrepo.GoalConfirmationReceipt, error) {
	item := f.receipt
	return &item, nil
}

func (f *goalConfirmationRepositoryFake) ListRecoverableGoalConfirmations(
	context.Context,
	orchestrationrepo.ListRecoverableGoalConfirmationsQuery,
) ([]orchestrationrepo.GoalConfirmationReceipt, error) {
	if f.receipt.State != orchestrationrepo.GoalConfirmationPending {
		return nil, nil
	}
	return []orchestrationrepo.GoalConfirmationReceipt{f.receipt}, nil
}

func (f *goalConfirmationRepositoryFake) MarkGoalConfirmationConfirmed(
	_ context.Context,
	_ orchestrationrepo.MarkGoalConfirmationCommand,
) (*orchestrationrepo.GoalConfirmationReceipt, error) {
	if f.receipt.State == orchestrationrepo.GoalConfirmationConfirmed {
		item := f.receipt
		return &item, nil
	}
	f.receipt.State = orchestrationrepo.GoalConfirmationConfirmed
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
	command orchestrationrepo.MarkGoalConfirmationCommand,
) (*orchestrationrepo.GoalConfirmationReceipt, error) {
	if f.receipt.State == orchestrationrepo.GoalConfirmationConfirmed {
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
