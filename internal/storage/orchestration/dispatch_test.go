// dispatch_test.go 覆盖编排派发、批次指派、取消、评审派发与子代理对账仓储测试。
package orchestration

import (
	"context"
	"errors"
	"fmt"
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

func TestAssignmentBatchAtomicRollbackAndDurableReplay(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := repository.Create(ctx, createTestCommand("batch")); err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.WritePlan(ctx, testPlanCommand("batch", 1, "batch", "", 1))
	if err != nil {
		t.Fatal(err)
	}
	batch := AssignBatchCommand{
		ExecutionID: snapshot.Execution.ID, ExpectedExecutionVersion: snapshot.Execution.Version,
		RequestDigest: "exact-intent", Meta: testMeta("batch"),
	}
	for i := 1; i <= 2; i++ {
		suffix := fmt.Sprint(i)
		item := assignTestCommand(snapshot, "work-batch-"+suffix, "spec-batch-"+suffix, "batch-"+suffix, "agent-worker-"+suffix)
		item.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
		item.Dispatch = &protocol.ExecutionDispatch{
			ID: "dispatch-" + suffix, DedupeKey: "batch-dispatch-" + suffix, TargetAgentID: item.Assignment.OwnerAgentID,
			Kind: protocol.ExecutionDispatchRoomDirected, Status: protocol.ExecutionDispatchStatusPending,
			Instruction: "deliver evidence",
		}
		batch.Assignments = append(batch.Assignments, item)
	}
	assertUnchanged := func() {
		t.Helper()
		current, err := repository.GetSnapshot(ctx, batch.ExecutionID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Execution.Version != snapshot.Execution.Version || len(current.Assignments) != 0 || len(current.Attempts) != 0 || len(current.Dispatches) != 0 {
			t.Fatalf("failed batch left writes: %+v", current)
		}
		assertEventSequence(t, repository.db, batch.ExecutionID, 2)
	}
	invalid := batch
	invalid.Assignments = append([]AssignCommand(nil), batch.Assignments...)
	invalid.Assignments[1].Assignment.SpecID = "wrong-spec"
	if _, err := repository.AssignBatch(ctx, invalid); err == nil {
		t.Fatal("second invalid assignment committed")
	}
	assertUnchanged()
	duplicate := batch
	duplicate.Assignments = []AssignCommand{batch.Assignments[0], batch.Assignments[0]}
	if _, err := repository.AssignBatch(ctx, duplicate); !errors.Is(err, ErrInvariant) {
		t.Fatalf("duplicate work error = %v", err)
	}
	assertUnchanged()
	stale := batch
	stale.ExpectedExecutionVersion--
	if _, err := repository.AssignBatch(ctx, stale); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale batch error = %v", err)
	}
	assertUnchanged()
	for _, command := range []AssignBatchCommand{batch, batch} {
		committed, err := repository.AssignBatch(ctx, command)
		if err != nil {
			t.Fatal(err)
		}
		if committed.Execution.Version != snapshot.Execution.Version+1 || len(committed.Assignments) != 2 || len(committed.Attempts) != 2 || len(committed.Dispatches) != 2 {
			t.Fatalf("batch commit/replay = %+v", committed)
		}
		assertEventSequence(t, repository.db, batch.ExecutionID, 5)
	}
	changedIntent := batch
	changedIntent.RequestDigest = "different-intent"
	if _, err := repository.AssignBatch(ctx, changedIntent); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed request intent error = %v", err)
	}
	changedActor := batch
	changedActor.Meta.ActorID = "another-actor"
	if _, err := repository.AssignBatch(ctx, changedActor); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("changed actor error = %v", err)
	}
	assertEventSequence(t, repository.db, batch.ExecutionID, 5)
}

func TestRepositoryReplacementEnqueuesAndRecoversExactCancellation(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	create := createTestCommand("cancel-outbox-old")
	plan := testPlanCommand("cancel-outbox-old", 1, "cancel-outbox-old", "", 1)
	old, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: create.Execution,
		Plan:      plan,
		Meta:      create.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	old, err = repository.Assign(
		ctx,
		assignTestCommand(
			old,
			old.WorkItems[0].ID,
			old.WorkItemSpecs[0].ID,
			"cancel-outbox",
			"agent-worker",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	assignment := findAssignment(t, old, "assignment-cancel-outbox")
	attempt := findAttempt(t, old, "attempt-cancel-outbox")
	attempt.RuntimeSessionKey = "runtime-session-old"
	attempt.RuntimeRoundID = "runtime-round-old"
	attempt.RootRoundID = "root-round-old"
	attempt.AgentRoundID = "agent-round-old"
	old, err = repository.StartAttempt(ctx, StartAttemptCommand{
		ExpectedExecutionVersion:  old.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		ExpectedAttemptVersion:    attempt.Version,
		Attempt:                   attempt,
		Meta:                      testMeta("start-cancel-outbox"),
	})
	if err != nil {
		t.Fatal(err)
	}
	command := replacementTestCommand(old, "cancel-outbox-successor")
	successor, err := repository.ReplaceWithPlan(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if len(successor.CancellationDispatches) != 0 {
		t.Fatalf(
			"successor inherited predecessor cancellation rows: %+v",
			successor.CancellationDispatches,
		)
	}
	terminal, err := repository.GetSnapshot(ctx, old.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(terminal.CancellationDispatches) != 1 {
		t.Fatalf(
			"terminal cancellation rows = %+v",
			terminal.CancellationDispatches,
		)
	}
	captured := terminal.CancellationDispatches[0]
	if captured.AttemptID != attempt.ID ||
		captured.RuntimeAttemptID != attempt.ID ||
		captured.TargetKind != protocol.ExecutionCancellationTargetRuntimeRound ||
		captured.RuntimeSessionKey != "runtime-session-old" ||
		captured.RuntimeRoundID != "runtime-round-old" ||
		captured.CommandID != command.Meta.CommandID {
		t.Fatalf("captured cancellation = %+v", captured)
	}

	available, err := repository.ListAvailableCancellationDispatches(ctx, 10)
	if err != nil || len(available) != 1 {
		t.Fatalf("available cancellation rows = %+v, err=%v", available, err)
	}
	claimed, err := repository.ClaimCancellationDispatch(
		ctx,
		available[0].ID,
		available[0].Version,
		"worker-a",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	retryAt := time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC)
	pending, err := repository.RetryCancellationDispatch(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-a",
		retryAt,
		"runtime temporarily unavailable",
	)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != protocol.ExecutionCancellationDispatchPending ||
		pending.LastError == "" {
		t.Fatalf("retried cancellation = %+v", pending)
	}
	repository.now = func() time.Time { return retryAt.Add(time.Second) }
	claimed, err = repository.ClaimCancellationDispatch(
		ctx,
		pending.ID,
		pending.Version,
		"worker-b",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := repository.ResolveCancellationDispatch(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-b",
		protocol.ExecutionCancellationDispatchDelivered,
		protocol.ExecutionCancellationOutcomeLocalRoundCancelled,
		"provider_interrupt_unsafe_shared_session",
		"exact old local round cancelled",
	)
	if err != nil {
		t.Fatal(err)
	}
	if delivered.Status != protocol.ExecutionCancellationDispatchDelivered ||
		delivered.Outcome != protocol.ExecutionCancellationOutcomeLocalRoundCancelled ||
		delivered.LimitationCode != "provider_interrupt_unsafe_shared_session" ||
		delivered.Receipt == "" {
		t.Fatalf("delivered cancellation = %+v", delivered)
	}
	if _, err = repository.ClaimCancellationDispatch(
		ctx,
		delivered.ID,
		delivered.Version,
		"worker-c",
		time.Minute,
	); !errors.Is(err, ErrDispatchLease) {
		t.Fatalf("terminal cancellation was reclaimed: %v", err)
	}

	if _, err = repository.ReplaceWithPlan(ctx, command); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = repository.db.QueryRow(
		`SELECT COUNT(1)
		 FROM execution_cancellation_dispatches
		 WHERE attempt_id = ?`,
		attempt.ID,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("replacement replay created %d cancellation rows", count)
	}
}

func TestBuildCancellationDispatchUsesParentRuntimeForRoomSubagent(t *testing.T) {
	now := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)
	execution := protocol.Execution{
		ID:             "execution-room",
		SessionKey:     "room:room-1:group:conversation-1",
		ScopeKind:      protocol.ExecutionScopeRoom,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
	}
	assignment := protocol.WorkAssignment{
		ID:           "assignment-1",
		OwnerAgentID: "agent-worker",
	}
	parent := protocol.WorkAttempt{
		ID:                "attempt-parent",
		ExecutionID:       execution.ID,
		PlanID:            "plan-1",
		WorkItemID:        "work-1",
		SpecID:            "spec-1",
		AssignmentID:      assignment.ID,
		DispatchID:        "dispatch-1",
		ExecutorKind:      protocol.AttemptExecutorAgent,
		RuntimeSessionKey: "agent:agent-worker:ws:group:conversation-1",
		RuntimeRoundID:    "agent-round-1",
		AgentRoundID:      "agent-round-1",
		Status:            protocol.WorkAttemptStatusRunning,
	}
	child := parent
	child.ID = "attempt-child"
	child.DispatchID = ""
	child.ParentAttemptID = parent.ID
	child.ExecutorKind = protocol.AttemptExecutorSubagent
	child.ToolUseID = "tool-child"
	dispatch := buildCancellationDispatch(
		execution,
		assignment,
		child,
		parent,
		"command-1",
		"plan superseded",
		now,
	)
	if dispatch.TargetKind != protocol.ExecutionCancellationTargetRoomSlot ||
		dispatch.AttemptID != child.ID ||
		dispatch.RuntimeAttemptID != parent.ID ||
		dispatch.DispatchID != parent.DispatchID ||
		dispatch.ToolUseID != child.ToolUseID ||
		dispatch.AgentRoundID != parent.AgentRoundID {
		t.Fatalf("subagent cancellation target = %+v", dispatch)
	}
}

func TestRepositoryRoomSubmitAtomicallyCreatesReviewDispatch(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("review-return"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand(
			"review-return",
			snapshot.Execution.Version,
			"review-return",
			"",
			1,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(
		snapshot,
		"work-review-return-1",
		"spec-review-return-1",
		"review-return",
		"agent-worker",
	)
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Assignment.ReturnToAgentID = "agent-lead"
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-review-return",
		DedupeKey:     "assignment:review-return",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver result",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.Assignment.ID,
		assign.RootAttempt.ID,
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.RootAttempt.ID,
		protocol.WorkAttemptStatusSucceeded,
	)
	assignment := findAssignment(t, snapshot, assign.Assignment.ID)
	submission := protocol.WorkSubmission{
		ID:               "submission-review-return",
		ExecutionID:      assignment.ExecutionID,
		PlanID:           assignment.PlanID,
		WorkItemID:       assignment.WorkItemID,
		SpecID:           assignment.SpecID,
		AssignmentID:     assignment.ID,
		AttemptID:        assign.RootAttempt.ID,
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "completed review return",
		Evidence:         []string{"verified"},
	}
	versionBeforeFailedSubmit := snapshot.Execution.Version
	if _, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submission,
		Meta:                      testMeta("submit-without-review-return"),
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("Room Submit without review Dispatch error = %v, want ErrInvariant", err)
	}
	unchanged, err := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Execution.Version != versionBeforeFailedSubmit ||
		len(unchanged.Submissions) != 0 ||
		len(unchanged.ReviewDispatches) != 0 {
		t.Fatalf("failed Room Submit was not atomic: %+v", unchanged)
	}

	assignment = findAssignment(t, unchanged, assign.Assignment.ID)
	snapshot, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  unchanged.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submission,
		ReviewDispatch: &protocol.ExecutionReviewDispatch{
			ID:            "review-dispatch-1",
			DedupeKey:     "review-return:" + submission.ID,
			TargetAgentID: "agent-lead",
			Status:        protocol.ExecutionReviewDispatchStatusPending,
			Instruction:   "review the submitted result",
		},
		Meta: testMeta("submit-with-review-return"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Submissions) != 1 ||
		len(snapshot.ReviewDispatches) != 1 {
		t.Fatalf(
			"Submission/review Dispatch snapshot = submissions=%+v dispatches=%+v",
			snapshot.Submissions,
			snapshot.ReviewDispatches,
		)
	}
	reviewDispatch := snapshot.ReviewDispatches[0]
	if reviewDispatch.SubmissionID != submission.ID ||
		reviewDispatch.AssignmentID != assignment.ID ||
		reviewDispatch.TargetAgentID != assignment.ReturnToAgentID ||
		reviewDispatch.Status != protocol.ExecutionReviewDispatchStatusPending {
		t.Fatalf("review Dispatch = %+v", reviewDispatch)
	}
	candidates, err := repository.ListAvailableReviewDispatches(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != reviewDispatch.ID {
		t.Fatalf("available review Dispatches = %+v", candidates)
	}
	claimed, err := repository.ClaimReviewDispatch(
		ctx,
		reviewDispatch.ID,
		reviewDispatch.Version,
		"review-worker",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != protocol.ExecutionReviewDispatchStatusClaimed ||
		claimed.DeliveryAttempts != 1 {
		t.Fatalf("claimed review Dispatch = %+v", claimed)
	}

	snapshot, err = repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = reviewTestWork(
		t,
		ctx,
		repository,
		snapshot,
		assignment.ID,
		submission.ID,
		"acceptance-review-return",
		protocol.WorkAcceptanceAccepted,
	)
	if len(snapshot.ReviewDispatches) != 1 ||
		snapshot.ReviewDispatches[0].Status !=
			protocol.ExecutionReviewDispatchStatusCancelled {
		t.Fatalf(
			"review did not cancel undelivered return: %+v",
			snapshot.ReviewDispatches,
		)
	}
}

func TestNormalizeReviewDispatchAllowsSelfAssignmentWithIndependentReviewer(t *testing.T) {
	repository := &Repository{}
	submission := protocol.WorkSubmission{
		ID:           "submission-independent-review",
		ExecutionID:  "execution-independent-review",
		PlanID:       "plan-independent-review",
		WorkItemID:   "work-independent-review",
		SpecID:       "spec-independent-review",
		AssignmentID: "assignment-independent-review",
	}
	assignment := protocol.WorkAssignment{
		ID:              submission.AssignmentID,
		OwnerAgentID:    "agent-coordinator",
		ReturnToAgentID: "agent-reviewer",
		Strategy:        protocol.AssignmentStrategySelf,
	}
	dispatch, err := repository.normalizeReviewDispatch(
		&protocol.ExecutionReviewDispatch{
			ID:            "review-dispatch-independent-review",
			TargetAgentID: assignment.ReturnToAgentID,
			Status:        protocol.ExecutionReviewDispatchStatusPending,
			Instruction:   "review the coordinator deliverable",
		},
		submission,
		assignment,
		testMeta("submit-independent-review"),
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if dispatch == nil || dispatch.AssignmentID != assignment.ID ||
		dispatch.TargetAgentID != assignment.ReturnToAgentID {
		t.Fatalf("independent review Dispatch = %+v", dispatch)
	}
}

func TestRepositoryTerminalizerCancelsClaimedReviewDispatch(t *testing.T) {
	repository, snapshot, reviewDispatch := setupClaimableReviewDispatch(t, "terminal-review")
	ctx := context.Background()
	claimed, err := repository.ClaimReviewDispatch(
		ctx,
		reviewDispatch.ID,
		reviewDispatch.Version,
		"review-worker",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.Abandon(ctx, AbandonCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Reason:                   "user abandoned objective",
		Meta:                     testMeta("abandon-terminal-review"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusCancelled {
		t.Fatalf("Execution status = %s", snapshot.Execution.Status)
	}
	current, err := repository.getReviewDispatch(
		ctx,
		repository.db,
		claimed.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil ||
		current.Status != protocol.ExecutionReviewDispatchStatusCancelled ||
		current.LeaseOwner != "" {
		t.Fatalf("terminal review Dispatch = %+v", current)
	}
}

func setupClaimableReviewDispatch(
	t *testing.T,
	suffix string,
) (*Repository, *protocol.ExecutionSnapshot, protocol.ExecutionReviewDispatch) {
	t.Helper()
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand(suffix))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand(suffix, snapshot.Execution.Version, suffix, "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(
		snapshot,
		"work-"+suffix+"-1",
		"spec-"+suffix+"-1",
		suffix,
		"agent-worker",
	)
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Assignment.ReturnToAgentID = "agent-lead"
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-" + suffix,
		DedupeKey:     "assignment:" + suffix,
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver result",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.Assignment.ID,
		assign.RootAttempt.ID,
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.RootAttempt.ID,
		protocol.WorkAttemptStatusSucceeded,
	)
	assignment := findAssignment(t, snapshot, assign.Assignment.ID)
	submission := protocol.WorkSubmission{
		ID:               "submission-" + suffix,
		ExecutionID:      assignment.ExecutionID,
		PlanID:           assignment.PlanID,
		WorkItemID:       assignment.WorkItemID,
		SpecID:           assignment.SpecID,
		AssignmentID:     assignment.ID,
		AttemptID:        assign.RootAttempt.ID,
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "completed " + suffix,
	}
	reviewDispatch := protocol.ExecutionReviewDispatch{
		ID:            "review-dispatch-" + suffix,
		DedupeKey:     "review-return:" + submission.ID,
		TargetAgentID: "agent-lead",
		Status:        protocol.ExecutionReviewDispatchStatusPending,
		Instruction:   "review result",
	}
	snapshot, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submission,
		ReviewDispatch:            &reviewDispatch,
		Meta:                      testMeta("submit-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository, snapshot, snapshot.ReviewDispatches[0]
}

func TestRepositorySubagentReconciliationDeadlineSurvivesRepositoryRestart(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("reconcile"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("reconcile", snapshot.Execution.Version, "reconcile", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.Assign(
		ctx,
		assignTestCommand(
			snapshot,
			"work-reconcile-1",
			"spec-reconcile-1",
			"reconcile",
			"agent-worker",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		"assignment-reconcile",
		"attempt-reconcile",
	)
	assignment := findAssignment(t, snapshot, "assignment-reconcile")
	child := protocol.WorkAttempt{
		ID:                "attempt-reconcile-child",
		ExecutionID:       snapshot.Execution.ID,
		PlanID:            snapshot.Plan.ID,
		WorkItemID:        assignment.WorkItemID,
		SpecID:            assignment.SpecID,
		AssignmentID:      assignment.ID,
		ParentAttemptID:   "attempt-reconcile",
		ExecutorKind:      protocol.AttemptExecutorSubagent,
		ParentAgentID:     "agent-worker",
		RuntimeSessionKey: "runtime-session",
		SDKSessionID:      "sdk-session",
		ToolUseID:         "tool-reconcile",
		Status:            protocol.WorkAttemptStatusRunning,
	}
	snapshot, err = repository.StartAttempt(ctx, StartAttemptCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		ExpectedAttemptVersion:    0,
		Attempt:                   child,
		Meta:                      testMeta("start-reconcile-child"),
	})
	if err != nil {
		t.Fatal(err)
	}
	child = findAttempt(t, snapshot, child.ID)
	nextDeadline, err := repository.NextSubagentReconciliationDeadline(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if nextDeadline != nil {
		t.Fatalf("unscheduled child deadline = %s", nextDeadline)
	}
	beforeRestart, err := repository.ListOrphanedSubagentAttempts(
		ctx,
		child.CreatedAt,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeRestart) != 0 {
		t.Fatalf("same-process child classified as orphan: %#v", beforeRestart)
	}
	afterRestart, err := repository.ListOrphanedSubagentAttempts(
		ctx,
		child.CreatedAt.Add(time.Second),
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRestart) != 1 || afterRestart[0].ID != child.ID {
		t.Fatalf("pre-restart unscheduled child Attempts = %#v", afterRestart)
	}
	exitedAt := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	reconcileAfter := exitedAt.Add(protocol.SubagentReconciliationGrace)
	if _, err = repository.ScheduleSubagentReconciliation(
		ctx,
		ScheduleSubagentReconciliationCommand{
			ExecutionID:              snapshot.Execution.ID,
			ExpectedExecutionVersion: snapshot.Execution.Version,
			ExpectedAttemptVersion:   child.Version,
			AttemptID:                child.ID,
			ParentRoundExitedAt:      exitedAt,
			ReconcileAfter:           reconcileAfter.Add(time.Nanosecond),
			Meta:                     testMeta("reject-non-exact-reconcile-child"),
		},
	); err == nil {
		t.Fatal("non-exact reconciliation deadline was accepted")
	}
	snapshot, err = repository.ScheduleSubagentReconciliation(
		ctx,
		ScheduleSubagentReconciliationCommand{
			ExecutionID:              snapshot.Execution.ID,
			ExpectedExecutionVersion: snapshot.Execution.Version,
			ExpectedAttemptVersion:   child.Version,
			AttemptID:                child.ID,
			ParentRoundExitedAt:      exitedAt,
			ReconcileAfter:           reconcileAfter,
			Meta:                     testMeta("schedule-reconcile-child"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	scheduled := findAttempt(t, snapshot, child.ID)
	if scheduled.ParentRoundExitedAt == nil ||
		!scheduled.ParentRoundExitedAt.Equal(exitedAt) ||
		scheduled.ReconcileAfter == nil ||
		!scheduled.ReconcileAfter.Equal(reconcileAfter) {
		t.Fatalf("scheduled child Attempt = %#v", scheduled)
	}
	deadline, err := repository.NextSubagentReconciliationAt(ctx)
	if err != nil || deadline == nil || !deadline.Equal(reconcileAfter) {
		t.Fatalf("subagent reconciliation deadline = %v, err=%v", deadline, err)
	}
	orphansAfterSchedule, err := repository.ListOrphanedSubagentAttempts(
		ctx,
		child.CreatedAt.Add(time.Second),
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphansAfterSchedule) != 0 {
		t.Fatalf("durably scheduled child remained orphaned: %#v", orphansAfterSchedule)
	}
	restarted := NewSQLRepository("sqlite", repository.db)
	nextDeadline, err = restarted.NextSubagentReconciliationDeadline(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if nextDeadline == nil || !nextDeadline.Equal(reconcileAfter) {
		t.Fatalf("next reconciliation deadline = %v, want %s", nextDeadline, reconcileAfter)
	}
	before, err := restarted.ListExpiredSubagentAttempts(
		ctx,
		reconcileAfter.Add(-time.Nanosecond),
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("child became due before deadline: %#v", before)
	}
	expired, err := restarted.ListExpiredSubagentAttempts(
		ctx,
		reconcileAfter,
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0].ID != child.ID ||
		expired[0].ReconcileAfter == nil ||
		!expired[0].ReconcileAfter.Equal(reconcileAfter) {
		t.Fatalf("expired child Attempts = %#v", expired)
	}
}
