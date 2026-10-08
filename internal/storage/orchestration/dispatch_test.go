package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryDispatchLeaseRetryAndIdempotentDelivery(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("dispatch"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("dispatch", snapshot.Execution.Version, "dispatch", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	command := assignTestCommand(
		snapshot,
		"work-dispatch-1",
		"spec-dispatch-1",
		"dispatch",
		"agent-worker",
	)
	command.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	command.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-1",
		DedupeKey:     "dispatch:work-1:agent-worker",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver evidence",
	}
	snapshot, err = repository.Assign(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	executionVersion := snapshot.Execution.Version

	candidates, err := repository.ListAvailableRoomDispatches(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != "dispatch-1" {
		t.Fatalf("available Dispatches = %+v", candidates)
	}
	deadlines, err := repository.ExecutionDispatchDeadlines(ctx)
	if err != nil || deadlines.Room == nil ||
		!deadlines.Room.Equal(candidates[0].AvailableAt) {
		t.Fatalf("pending dispatch deadline = %+v, err=%v", deadlines, err)
	}
	if _, err = repository.ClaimDispatch(ctx, "dispatch-1", 99, "worker-a", time.Minute); !errors.Is(err, ErrDispatchLease) {
		t.Fatalf("stale claim error = %v, want ErrDispatchLease", err)
	}
	claimed, err := repository.ClaimDispatch(
		ctx,
		candidates[0].ID,
		candidates[0].Version,
		"worker-a",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != protocol.ExecutionDispatchStatusClaimed ||
		claimed.DeliveryAttempts != 1 ||
		claimed.LeaseOwner != "worker-a" {
		t.Fatalf("claimed Dispatch = %+v", claimed)
	}
	deadlines, err = repository.ExecutionDispatchDeadlines(ctx)
	if err != nil || deadlines.Room == nil || claimed.LeaseExpiresAt == nil ||
		!deadlines.Room.Equal(*claimed.LeaseExpiresAt) {
		t.Fatalf("claimed dispatch deadline = %+v, dispatch=%+v err=%v", deadlines, claimed, err)
	}
	if _, err = repository.ClaimDispatch(
		ctx,
		claimed.ID,
		candidates[0].Version,
		"worker-b",
		time.Minute,
	); !errors.Is(err, ErrDispatchLease) {
		t.Fatalf("double claim error = %v, want ErrDispatchLease", err)
	}

	retryAt := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	pending, err := repository.RetryDispatch(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-a",
		retryAt,
		"Room temporarily unavailable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != protocol.ExecutionDispatchStatusPending ||
		pending.LeaseOwner != "" ||
		pending.LastError == "" {
		t.Fatalf("retried Dispatch = %+v", pending)
	}
	deadlines, err = repository.ExecutionDispatchDeadlines(ctx)
	if err != nil || deadlines.Room == nil || !deadlines.Room.Equal(retryAt) {
		t.Fatalf("retry dispatch deadline = %+v, err=%v", deadlines, err)
	}
	if available, listErr := repository.ListAvailableRoomDispatches(ctx, 10); listErr != nil {
		t.Fatal(listErr)
	} else if len(available) != 0 {
		t.Fatalf("future retry should not be claimable: %+v", available)
	}

	future := retryAt.Add(time.Second)
	repository.now = func() time.Time { return future }
	claimed, err = repository.ClaimDispatch(
		ctx,
		pending.ID,
		pending.Version,
		"worker-b",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := repository.MarkDispatchDelivered(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-b",
		"handoff-dispatch-1",
		"queue-dispatch-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if delivered.Status != protocol.ExecutionDispatchStatusDelivered ||
		delivered.HandoffID != "handoff-dispatch-1" ||
		delivered.QueueItemID != "queue-dispatch-1" {
		t.Fatalf("delivered Dispatch = %+v", delivered)
	}
	replayed, err := repository.MarkDispatchDelivered(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-b",
		"handoff-dispatch-1",
		"queue-dispatch-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != delivered.Version {
		t.Fatalf("duplicate ACK changed version: got %d want %d", replayed.Version, delivered.Version)
	}
	current, err := repository.Get(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != executionVersion {
		t.Fatalf("outbox lifecycle changed Execution version: got %d want %d", current.Version, executionVersion)
	}
}

func TestRepositoryCancelClaimedDispatchIsTerminalAndIdempotent(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("dispatch-cancel"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand(
			"dispatch-cancel",
			snapshot.Execution.Version,
			"dispatch-cancel",
			"",
			1,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	command := assignTestCommand(
		snapshot,
		"work-dispatch-cancel-1",
		"spec-dispatch-cancel-1",
		"dispatch-cancel",
		"agent-worker",
	)
	command.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	command.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-cancel-1",
		DedupeKey:     "dispatch:cancel:agent-worker",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver evidence",
	}
	snapshot, err = repository.Assign(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := repository.ListAvailableRoomDispatches(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("available Dispatches = %+v", candidates)
	}
	claimed, err := repository.ClaimDispatch(
		ctx,
		candidates[0].ID,
		candidates[0].Version,
		"worker-a",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := repository.CancelDispatch(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-a",
		"current WorkContract no longer exists",
	)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != protocol.ExecutionDispatchStatusCancelled ||
		cancelled.LeaseOwner != "" ||
		cancelled.LastError != "current WorkContract no longer exists" {
		t.Fatalf("cancelled Dispatch = %+v", cancelled)
	}
	released, err := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if released.Execution.Version != snapshot.Execution.Version+1 {
		t.Fatalf(
			"responsibility release version = %d, want %d",
			released.Execution.Version,
			snapshot.Execution.Version+1,
		)
	}
	if len(released.Assignments) != 1 ||
		released.Assignments[0].Status != protocol.WorkAssignmentStatusReleased {
		t.Fatalf("permanent failure did not release Assignment: %+v", released.Assignments)
	}
	if len(released.Attempts) != 1 ||
		released.Attempts[0].Status != protocol.WorkAttemptStatusCancelled ||
		!strings.Contains(released.Attempts[0].FailureReason, "permanent dispatch failure") {
		t.Fatalf("permanent failure did not terminalize Attempt: %+v", released.Attempts)
	}
	if !contains(released.ReadyWorkItemIDs, command.Assignment.WorkItemID) {
		t.Fatalf("released Work Item is not ready again: %+v", released.ReadyWorkItemIDs)
	}
	assertEventSequence(t, repository.db, snapshot.Execution.ID, 4)
	replayed, err := repository.CancelDispatch(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-a",
		"current WorkContract no longer exists",
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != cancelled.Version {
		t.Fatalf(
			"idempotent cancel changed version: got %d want %d",
			replayed.Version,
			cancelled.Version,
		)
	}
	available, err := repository.ListAvailableRoomDispatches(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(available) != 0 {
		t.Fatalf("cancelled Dispatch remained claimable: %+v", available)
	}
}
