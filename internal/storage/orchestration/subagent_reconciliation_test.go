package orchestration

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
