package orchestration

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestAllBulkAttemptInterruptionPathsCaptureCancellationFirst(t *testing.T) {
	files := []string{
		"state.go",
		"plan.go",
		"assignment.go",
		"execution_transition.go",
	}
	checked := 0
	for _, name := range files {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(content)
		offset := 0
		for {
			index := strings.Index(source[offset:], "UPDATE execution_attempts")
			if index < 0 {
				break
			}
			index += offset
			afterEnd := min(len(source), index+600)
			after := source[index:afterEnd]
			if strings.Contains(after, "status IN ('pending', 'running')") {
				beforeStart := max(0, index-1400)
				before := source[beforeStart:index]
				if !strings.Contains(before, "enqueueAttemptCancellations(") {
					t.Fatalf(
						"%s bulk terminalization at byte %d has no preceding cancellation capture",
						name,
						index,
					)
				}
				checked++
			}
			offset = index + len("UPDATE execution_attempts")
		}
	}
	if checked != 4 {
		t.Fatalf("checked %d bulk interruption paths, want 4", checked)
	}
}
