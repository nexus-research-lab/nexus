package orchestration

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	"testing"
)

type recoveryConsumerFunc func(context.Context, protocol.Execution, string) error

func (f recoveryConsumerFunc) QueueExecutionRecovery(ctx context.Context, e protocol.Execution, key string) error {
	return f(ctx, e, key)
}

func TestRootRecoveryCASExhaustionIsNotSuccess(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Attempts = []protocol.WorkAttempt{{ID: "root", Status: protocol.WorkAttemptStatusRunning}}
	calls := 0
	service := testService(&fakeRepository{snapshot: snapshot, finishAttempt: func(context.Context, store.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
		calls++
		return nil, store.ErrVersionConflict
	}})
	_, err := service.finishRootAttempts(context.Background(), coordinatorActor(), snapshot, "", protocol.WorkAttemptStatusInterrupted, "host_restart")
	if !errors.Is(err, store.ErrVersionConflict) || calls != 3 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRootRecoveryLeavesPendingOutboxAndAcceptedWork(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Attempts = []protocol.WorkAttempt{
		{ID: "running", Status: protocol.WorkAttemptStatusRunning},
		{ID: "undelivered", Status: protocol.WorkAttemptStatusPending, DispatchID: "pending"},
		{ID: "delivered", Status: protocol.WorkAttemptStatusPending, DispatchID: "delivered"},
		{ID: "accepted", Status: protocol.WorkAttemptStatusSucceeded},
		{ID: "child", ParentAttemptID: "running", Status: protocol.WorkAttemptStatusRunning},
	}
	snapshot.Dispatches = []protocol.ExecutionDispatch{{ID: "pending", Status: protocol.ExecutionDispatchStatusPending}, {ID: "delivered", Status: protocol.ExecutionDispatchStatusDelivered}}
	var settled []string
	service := testService(&fakeRepository{snapshot: snapshot, finishAttempt: func(_ context.Context, c store.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
		settled = append(settled, c.Attempt.ID)
		return snapshot, nil
	}})
	if _, err := service.finishRootAttempts(context.Background(), coordinatorActor(), snapshot, "", protocol.WorkAttemptStatusInterrupted, "host_restart"); err != nil {
		t.Fatal(err)
	}
	if len(settled) != 2 || settled[0] != "running" || settled[1] != "delivered" {
		t.Fatalf("settled=%v", settled)
	}
}

func TestRootRecoveryDefersGoalBoundExecutionToGoalAuthority(t *testing.T) {
	service := testService(&fakeRepository{})
	calls := 0
	service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(context.Context, protocol.Execution, string) error { calls++; return nil }))
	snapshot := executionSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusActive
	snapshot.Execution.GoalID = "goal"
	if err := service.queueRootRecovery(context.Background(), snapshot, "recovery"); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("Goal recovery duplicated")
	}
	snapshot.Execution.GoalID = ""
	if err := service.queueRootRecovery(context.Background(), snapshot, "recovery"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("non-Goal recovery missing")
	}
}

func TestStartupDoesNotWakeStoppedCurrentAssignment(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Assignments = []protocol.WorkAssignment{{ID: "assignment", Status: protocol.WorkAssignmentStatusActive}}
	snapshot.Attempts = []protocol.WorkAttempt{{ID: "root", AssignmentID: "assignment", Status: protocol.WorkAttemptStatusInterrupted, FailureReason: "__nexus_interrupt_without_message__"}}
	if !rootRecoveryStoppedByUser(snapshot) {
		t.Fatal("restart would revive user stop")
	}
	snapshot.Attempts[0].FailureReason = "host_shutdown"
	if rootRecoveryStoppedByUser(snapshot) {
		t.Fatal("host shutdown treated as user stop")
	}
}

type recoveryRepository struct{ *fakeRepository }

func (r *recoveryRepository) ListRecoveryExecutions(_ context.Context, after string, _ int) ([]protocol.Execution, error) {
	if after != "" {
		return nil, nil
	}
	return []protocol.Execution{r.snapshot.Execution}, nil
}

func TestRootRecoveryRetriesEnqueueFromTerminalReceipt(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusActive
	snapshot.Assignments = []protocol.WorkAssignment{{ID: "assignment", OwnerAgentID: "member", Status: protocol.WorkAssignmentStatusActive}}
	snapshot.Attempts = []protocol.WorkAttempt{{ID: "failed-root", AssignmentID: "assignment", Status: protocol.WorkAttemptStatusFailed, Metadata: map[string]any{"host_recovery_required": true}}}
	service := NewService(&recoveryRepository{&fakeRepository{snapshot: snapshot}})
	calls := 0
	var keys []string
	service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(_ context.Context, _ protocol.Execution, key string) error {
		calls++
		keys = append(keys, key)
		if calls == 1 {
			return errors.New("queue write failed")
		}
		return nil
	}))
	cursor := ""
	if err := service.reconcileRootRecoveryInputs(context.Background(), &cursor, 64); err == nil {
		t.Fatal("lost queue failure")
	}
	if err := service.reconcileRootRecoveryInputs(context.Background(), &cursor, 64); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || keys[0] != keys[1] || keys[0] != "failed:failed-root" {
		t.Fatalf("unstable retry identity: %v", keys)
	}
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusRunning
	if err := service.reconcileRootRecoveryInputs(context.Background(), &cursor, 64); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("live root was replayed")
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
			service := testService(&fakeRepository{snapshot: snapshot, finishAttempt: func(_ context.Context, c store.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
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

func TestStartupRootRecoveryIsolatesEachExecution(t *testing.T) {
	repository := &multiRecoveryRepository{fakeRepository: &fakeRepository{}, snapshots: map[string]*protocol.ExecutionSnapshot{}, failing: map[string]error{}}
	for _, id := range []string{"execution-unreadable", "execution-deleted-room", "execution-healthy"} {
		snapshot := hostInterruptedSnapshot(id, hostShutdownAttemptReason)
		repository.executions = append(repository.executions, snapshot.Execution)
		repository.snapshots[id] = snapshot
	}
	unreadable := errors.New("snapshot read failed")
	repository.failing["execution-unreadable"] = unreadable
	service := NewService(repository)
	var queued []string
	service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(_ context.Context, e protocol.Execution, _ string) error {
		if e.ID == "execution-deleted-room" {
			return fmt.Errorf("%w: Room conversation is gone", ErrExecutionRecoveryTargetUnavailable)
		}
		queued = append(queued, e.ID)
		return nil
	}))

	err := service.RecoverRootAttempts(context.Background())

	if len(queued) != 1 || queued[0] != "execution-healthy" {
		t.Fatalf("queued = %v, want the healthy Execution recovered despite earlier failures", queued)
	}
	if !errors.Is(err, unreadable) {
		t.Fatalf("err = %v, want the unreadable Execution reported", err)
	}
	if errors.Is(err, ErrExecutionRecoveryTargetUnavailable) {
		t.Fatalf("err = %v, want an unavailable recovery target skipped rather than reported", err)
	}
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

// Startup wakes coordinators only for work the host left in flight, never for
// graphs that have been idle or abandoned since long before this start.
func TestStartupRootRecoveryWakesOnlyHostInterruptedWork(t *testing.T) {
	repository := &multiRecoveryRepository{fakeRepository: &fakeRepository{}, snapshots: map[string]*protocol.ExecutionSnapshot{}, failing: map[string]error{}}
	add := func(snapshot *protocol.ExecutionSnapshot) {
		repository.executions = append(repository.executions, snapshot.Execution)
		repository.snapshots[snapshot.Execution.ID] = snapshot
	}
	add(hostInterruptedSnapshot("execution-shutdown", hostShutdownAttemptReason))
	add(hostInterruptedSnapshot("execution-user-stopped", message.InterruptWithoutMessage))
	add(hostInterruptedSnapshot("execution-host-control", "host_control_stop"))
	stale := hostInterruptedSnapshot("execution-stale-failure", "Room slot failed")
	stale.Attempts[0].Status = protocol.WorkAttemptStatusFailed
	add(stale)
	marked := hostInterruptedSnapshot("execution-marked-failure", "provider error")
	marked.Attempts[0].Status = protocol.WorkAttemptStatusFailed
	marked.Attempts[0].Metadata = map[string]any{"host_recovery_required": true}
	add(marked)
	idle := executionSnapshot()
	idle.Execution.ID = "execution-idle"
	idle.Execution.Status = protocol.ExecutionStatusActive
	add(idle)
	orphan := hostInterruptedSnapshot("execution-orphan", "")
	orphan.Attempts[0].Status = protocol.WorkAttemptStatusRunning
	add(orphan)
	repository.finishAttempt = func(_ context.Context, command store.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
		current := repository.snapshots[command.Attempt.ExecutionID]
		if current == nil {
			current = repository.snapshots["execution-orphan"]
		}
		next := *current
		next.Execution.Version++
		next.Attempts = []protocol.WorkAttempt{command.Attempt}
		repository.snapshots[next.Execution.ID] = &next
		return &next, nil
	}
	service := NewService(repository)
	var queued []string
	service.SetExecutionRecoveryConsumer(recoveryConsumerFunc(func(_ context.Context, e protocol.Execution, _ string) error {
		queued = append(queued, e.ID)
		return nil
	}))

	if err := service.RecoverRootAttempts(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"execution-shutdown", "execution-marked-failure", "execution-orphan"}
	if !slices.Equal(queued, want) {
		t.Fatalf("queued = %v, want %v", queued, want)
	}
}
