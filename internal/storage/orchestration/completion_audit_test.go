package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
)

func TestAcceptedReviewAndCompletionAuditReceiptCommitAtomically(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot := prepareCompletionAuditSubmission(t, ctx, repository, "atomic")
	assignment := findAssignment(t, snapshot, "assignment-completion-atomic")
	submission := findSubmission(t, snapshot, "submission-completion-atomic")

	snapshot, err := repository.Review(ctx, completionAuditReviewCommand(
		snapshot,
		assignment,
		submission,
		"acceptance-completion-atomic",
	))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusActive ||
		len(snapshot.CompletionBlockers) != 0 {
		t.Fatalf("accepted review snapshot = %#v", snapshot)
	}
	receipt, err := repository.GetCompletionAuditReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.State != CompletionAuditPending ||
		receipt.TriggerAcceptanceID != "acceptance-completion-atomic" ||
		receipt.Version != 1 || receipt.NextAttemptAt == nil ||
		receipt.SettledAt != nil {
		t.Fatalf("accepted review receipt = %#v", receipt)
	}
	deadlines, err := repository.OrchestrationRecoveryDeadlines(ctx)
	if err != nil || deadlines.CompletionAudit == nil ||
		!deadlines.CompletionAudit.Equal(*receipt.NextAttemptAt) {
		t.Fatalf("completion audit deadline = %+v, err=%v", deadlines, err)
	}

	snapshot, err = repository.Complete(ctx, CompleteCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Meta:                     testMeta("complete-completion-atomic"),
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = repository.GetCompletionAuditReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusCompleted ||
		receipt == nil || receipt.State != CompletionAuditCompleted ||
		receipt.NextAttemptAt != nil || receipt.LastError != "" ||
		receipt.SettledAt == nil {
		t.Fatalf("completed snapshot=%#v receipt=%#v", snapshot.Execution, receipt)
	}
	deadlines, err = repository.OrchestrationRecoveryDeadlines(ctx)
	if err != nil || deadlines.CompletionAudit != nil {
		t.Fatalf("settled completion audit deadline = %+v, err=%v", deadlines, err)
	}
}

func TestReviewVersionConflictLeavesNoAcceptanceOrCompletionAudit(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	stale := prepareCompletionAuditSubmission(t, ctx, repository, "cas")
	assignment := findAssignment(t, stale, "assignment-completion-cas")
	submission := findSubmission(t, stale, "submission-completion-cas")

	current, err := repository.RecordEvidence(ctx, RecordEvidenceCommand{
		ExecutionID:              stale.Execution.ID,
		ExpectedExecutionVersion: stale.Execution.Version,
		MetadataKey:              "completion_review_race",
		Meta:                     testMeta("completion-review-race"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if current.Execution.Version == stale.Execution.Version {
		t.Fatal("race mutation did not advance Execution version")
	}

	_, err = repository.Review(ctx, completionAuditReviewCommand(
		stale,
		assignment,
		submission,
		"acceptance-completion-cas",
	))
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale Review error = %v, want ErrVersionConflict", err)
	}
	current, err = repository.GetSnapshot(ctx, stale.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Acceptances) != 0 ||
		findAssignment(t, current, assignment.ID).Status != protocol.WorkAssignmentStatusActive {
		t.Fatalf("stale Review partially committed: %#v", current)
	}
	receipt, err := repository.GetCompletionAuditReceipt(ctx, stale.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt != nil {
		t.Fatalf("stale Review created completion audit receipt: %#v", receipt)
	}
}

func TestCompletionAuditMigrationBackfillsLegacyAcceptedReview(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot := prepareCompletionAuditSubmission(t, ctx, repository, "migration")
	assignment := findAssignment(t, snapshot, "assignment-completion-migration")
	submission := findSubmission(t, snapshot, "submission-completion-migration")
	snapshot, err := repository.Review(ctx, completionAuditReviewCommand(
		snapshot,
		assignment,
		submission,
		"acceptance-completion-migration",
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.CompletionBlockers) != 0 {
		t.Fatalf("legacy fixture retained blockers: %#v", snapshot.CompletionBlockers)
	}

	// Roll only the new receipt migration back to reproduce the exact legacy
	// state: accepted active WorkGraph, no durable completion-audit table.
	ensureGooseSQLiteDialect(t)
	if err = goose.DownTo(
		repository.db,
		orchestrationMigrationDir(t, "sqlite"),
		102,
	); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(
		repository.db,
		orchestrationMigrationDir(t, "sqlite"),
		103,
	); err != nil {
		t.Fatal(err)
	}

	restarted := NewSQLRepository("sqlite", repository.db)
	receipt, err := restarted.GetCompletionAuditReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.State != CompletionAuditPending ||
		receipt.TriggerAcceptanceID != "acceptance-completion-migration" ||
		receipt.NextAttemptAt == nil ||
		receipt.LastError != "legacy accepted review recovered during migration" {
		t.Fatalf("migration backfill receipt = %#v", receipt)
	}
}

func prepareCompletionAuditSubmission(
	t *testing.T,
	ctx context.Context,
	repository *Repository,
	suffix string,
) *protocol.ExecutionSnapshot {
	t.Helper()
	executionSuffix := "completion-" + suffix
	snapshot, err := repository.Create(ctx, createTestCommand(executionSuffix))
	if err != nil {
		t.Fatal(err)
	}
	plan := testPlanCommand(
		executionSuffix,
		snapshot.Execution.Version,
		executionSuffix,
		"",
		1,
	)
	plan.WorkItems = plan.WorkItems[:1]
	plan.WorkItems[0].Item.Terminal = true
	snapshot, err = repository.WritePlan(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	work := plan.WorkItems[0]
	assignmentSuffix := "completion-" + suffix
	snapshot, err = repository.Assign(ctx, assignTestCommand(
		snapshot,
		work.WorkItem.ID,
		work.Spec.ID,
		assignmentSuffix,
		"agent-worker",
	))
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		"assignment-"+assignmentSuffix,
		"attempt-"+assignmentSuffix,
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		"attempt-"+assignmentSuffix,
		protocol.WorkAttemptStatusSucceeded,
	)
	return submitTestWork(
		t,
		ctx,
		repository,
		snapshot,
		"assignment-"+assignmentSuffix,
		"attempt-"+assignmentSuffix,
		"submission-"+assignmentSuffix,
		"agent-worker",
	)
}

func completionAuditReviewCommand(
	snapshot *protocol.ExecutionSnapshot,
	assignment protocol.WorkAssignment,
	submission protocol.WorkSubmission,
	acceptanceID string,
) ReviewCommand {
	return ReviewCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Acceptance: protocol.WorkAcceptance{
			ID:           acceptanceID,
			ExecutionID:  submission.ExecutionID,
			PlanID:       submission.PlanID,
			WorkItemID:   submission.WorkItemID,
			SpecID:       submission.SpecID,
			AssignmentID: submission.AssignmentID,
			SubmissionID: submission.ID,
			Decision:     protocol.WorkAcceptanceAccepted,
			ReviewerKind: protocol.WorkReviewerAgent,
			ReviewerID:   "agent-lead",
			CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
				Criterion: "verified",
				Passed:    true,
			}},
		},
		Meta: testMeta("review-" + acceptanceID),
	}
}
