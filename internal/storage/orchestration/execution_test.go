// execution_test.go 覆盖编排执行状态迁移、完成审计、证据、Goal 确认与重定目标仓储测试。
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
)

func TestRepositoryCreateWithPlanIsAtomic(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	create := createTestCommand("atomic-create")
	plan := testPlanCommand("atomic-create", 1, "atomic-create", "", 1)
	snapshot, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: create.Execution,
		Plan:      plan,
		Meta:      create.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.ID != create.Execution.ID ||
		snapshot.Plan == nil ||
		snapshot.Plan.ID != plan.Plan.ID ||
		len(snapshot.WorkItems) != len(plan.WorkItems) {
		t.Fatalf("atomic snapshot = %#v", snapshot)
	}
	assertEventSequence(t, repository.db, snapshot.Execution.ID, 2)

	invalid := createTestCommand("atomic-invalid")
	invalidPlan := testPlanCommand("atomic-invalid", 1, "atomic-invalid", "", 1)
	invalidPlan.Plan.ID = ""
	if _, err = repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: invalid.Execution,
		Plan:      invalidPlan,
		Meta:      invalid.Meta,
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("invalid CreateWithPlan error = %v, want ErrInvariant", err)
	}
	if stored, getErr := repository.Get(ctx, invalid.Execution.ID); getErr != nil || stored != nil {
		t.Fatalf("invalid atomic create persisted Execution = %#v, err=%v", stored, getErr)
	}
}

func TestRepositoryReplaceWithPlanTerminalizesOldGraphAndReplays(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	create := createTestCommand("replace-old")
	plan := testPlanCommand("replace-old", 1, "replace-old", "", 1)
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
			"replace-accepted",
			"agent-worker",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	old = startTestAttempt(
		t,
		ctx,
		repository,
		old,
		"assignment-replace-accepted",
		"attempt-replace-accepted",
	)
	old = finishTestAttempt(
		t,
		ctx,
		repository,
		old,
		"attempt-replace-accepted",
		protocol.WorkAttemptStatusSucceeded,
	)
	old = submitTestWork(
		t,
		ctx,
		repository,
		old,
		"assignment-replace-accepted",
		"attempt-replace-accepted",
		"submission-replace-accepted",
		"agent-worker",
	)
	old = reviewTestWork(
		t,
		ctx,
		repository,
		old,
		"assignment-replace-accepted",
		"submission-replace-accepted",
		"acceptance-replace-accepted",
		protocol.WorkAcceptanceAccepted,
	)
	liveAssignment := assignTestCommand(
		old,
		old.WorkItems[1].ID,
		old.WorkItemSpecs[1].ID,
		"replace-live",
		"agent-worker",
	)
	liveAssignment.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	liveAssignment.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-replace-live",
		DedupeKey:     "dispatch-replace-live",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver the second work item",
	}
	old, err = repository.Assign(
		ctx,
		liveAssignment,
	)
	if err != nil {
		t.Fatal(err)
	}
	liveOldAssignment := findAssignment(t, old, "assignment-replace-live")
	command := replacementTestCommand(old, "replace-new")
	successor, err := repository.ReplaceWithPlan(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if successor.Execution.Status != protocol.ExecutionStatusActive ||
		successor.Execution.ReplacesExecutionID != old.Execution.ID ||
		successor.Plan == nil ||
		successor.Plan.ExecutionID != successor.Execution.ID {
		t.Fatalf("successor = %#v", successor)
	}
	assertTerminalizedExecutionGraph(
		t,
		repository,
		old.Execution.ID,
		protocol.ExecutionStatusSuperseded,
		protocol.PlanRevisionStatusSuperseded,
		protocol.WorkItemStatusSuperseded,
		protocol.WorkAttemptStatusInterrupted,
	)
	var submissionCount, acceptanceCount int
	if err = repository.db.QueryRow(
		`SELECT COUNT(1) FROM execution_submissions WHERE execution_id = ?`,
		old.Execution.ID,
	).Scan(&submissionCount); err != nil {
		t.Fatal(err)
	}
	if err = repository.db.QueryRow(
		`SELECT COUNT(1) FROM execution_acceptances WHERE execution_id = ?`,
		old.Execution.ID,
	).Scan(&acceptanceCount); err != nil {
		t.Fatal(err)
	}
	if submissionCount != 1 || acceptanceCount != 1 {
		t.Fatalf(
			"replacement did not preserve immutable delivery history: submissions=%d acceptances=%d",
			submissionCount,
			acceptanceCount,
		)
	}
	var dispatchStatus string
	if err = repository.db.QueryRow(
		`SELECT status FROM execution_dispatches WHERE dispatch_id = ?`,
		"dispatch-replace-live",
	).Scan(&dispatchStatus); err != nil {
		t.Fatal(err)
	}
	if dispatchStatus != string(protocol.ExecutionDispatchStatusCancelled) {
		t.Fatalf("replacement Dispatch status = %q", dispatchStatus)
	}

	replayedCommand := replacementTestCommand(old, "ignored-retry-successor")
	replayedCommand.Meta = command.Meta
	replayedCommand.SuccessorMeta = command.SuccessorMeta
	replayedCommand.Plan.Meta = command.Plan.Meta
	replayed, err := repository.ReplaceWithPlan(ctx, replayedCommand)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.ID != successor.Execution.ID {
		t.Fatalf("replacement replay created %q, want %q", replayed.Execution.ID, successor.Execution.ID)
	}

	stalePlan := testPlanCommand("replace-old", old.Execution.Version+1, "late-old", plan.Plan.ID, 2)
	if _, err = repository.WritePlan(ctx, stalePlan); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("terminal old Plan mutation error = %v, want ErrVersionConflict", err)
	}
	terminalExecution, getErr := repository.Get(ctx, old.Execution.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if _, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  terminalExecution.Version,
		ExpectedAssignmentVersion: liveOldAssignment.Version + 1,
		Submission: protocol.WorkSubmission{
			ID:               "submission-late-terminal",
			ExecutionID:      liveOldAssignment.ExecutionID,
			PlanID:           liveOldAssignment.PlanID,
			WorkItemID:       liveOldAssignment.WorkItemID,
			SpecID:           liveOldAssignment.SpecID,
			AssignmentID:     liveOldAssignment.ID,
			AttemptID:        "attempt-replace-live",
			SubmitterAgentID: liveOldAssignment.OwnerAgentID,
			ResultSummary:    "late result",
		},
		Meta: testMeta("submit-late-terminal"),
	}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("terminal old Submission error = %v, want ErrVersionConflict", err)
	}
	if err = repository.db.QueryRow(
		`SELECT COUNT(1) FROM execution_submissions WHERE execution_id = ?`,
		old.Execution.ID,
	).Scan(&submissionCount); err != nil {
		t.Fatal(err)
	}
	if submissionCount != 1 {
		t.Fatalf("terminal old Execution accepted late Submission: count=%d", submissionCount)
	}
}

func TestRepositoryReplaceWithPlanRollsBackOldTerminalizationOnSuccessorFailure(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	create := createTestCommand("replace-rollback")
	plan := testPlanCommand("replace-rollback", 1, "replace-rollback", "", 1)
	old, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: create.Execution,
		Plan:      plan,
		Meta:      create.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	command := replacementTestCommand(old, "replace-rollback-new")
	command.Plan.Plan.ID = old.Plan.ID
	for index := range command.Plan.WorkItems {
		command.Plan.WorkItems[index].Item.PlanID = old.Plan.ID
		for claimIndex := range command.Plan.WorkItems[index].OutputClaims {
			command.Plan.WorkItems[index].OutputClaims[claimIndex].PlanID = old.Plan.ID
		}
	}
	for index := range command.Plan.Dependencies {
		command.Plan.Dependencies[index].PlanID = old.Plan.ID
	}
	if _, err = repository.ReplaceWithPlan(ctx, command); err == nil {
		t.Fatal("replacement with duplicate Plan id succeeded")
	}
	unchanged, getErr := repository.GetSnapshot(ctx, old.Execution.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if unchanged.Execution.Status != protocol.ExecutionStatusActive ||
		unchanged.Plan == nil ||
		unchanged.Plan.ID != old.Plan.ID {
		t.Fatalf("failed replacement changed old Execution = %#v", unchanged)
	}
	if stored, getErr := repository.Get(ctx, command.Successor.ID); getErr != nil || stored != nil {
		t.Fatalf("failed replacement persisted successor = %#v, err=%v", stored, getErr)
	}
}

func TestRepositoryAbandonCancelsTransientGraphWithoutSuccessor(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	create := createTestCommand("abandon")
	plan := testPlanCommand("abandon", 1, "abandon", "", 1)
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
			"abandon",
			"agent-worker",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	command := AbandonCommand{
		ExecutionID:              old.Execution.ID,
		ExpectedExecutionVersion: old.Execution.Version,
		Reason:                   "user stopped the objective",
		Meta:                     testMeta("abandon-current"),
	}
	cancelled, err := repository.Abandon(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Execution.Status != protocol.ExecutionStatusCancelled {
		t.Fatalf("abandoned status = %s", cancelled.Execution.Status)
	}
	assertTerminalizedExecutionGraph(
		t,
		repository,
		old.Execution.ID,
		protocol.ExecutionStatusCancelled,
		protocol.PlanRevisionStatusCancelled,
		protocol.WorkItemStatusCancelled,
		protocol.WorkAttemptStatusCancelled,
	)
	if current, findErr := repository.FindCurrent(
		ctx,
		old.Execution.OwnerUserID,
		old.Execution.SessionKey,
	); findErr != nil || current != nil {
		t.Fatalf("abandon left current Execution = %#v, err=%v", current, findErr)
	}
	replayed, err := repository.Abandon(ctx, command)
	if err != nil || replayed.Execution.Status != protocol.ExecutionStatusCancelled {
		t.Fatalf("abandon replay = %#v, err=%v", replayed, err)
	}
}

func replacementTestCommand(
	old *protocol.ExecutionSnapshot,
	successorSuffix string,
) ReplaceWithPlanCommand {
	successorID := "execution-" + successorSuffix
	successor := protocol.Execution{
		ID:                  successorID,
		OwnerUserID:         old.Execution.OwnerUserID,
		SessionKey:          old.Execution.SessionKey,
		ScopeKind:           old.Execution.ScopeKind,
		RoomID:              old.Execution.RoomID,
		ConversationID:      old.Execution.ConversationID,
		CoordinatorAgentID:  old.Execution.CoordinatorAgentID,
		Origin:              protocol.ExecutionOriginUserRequest,
		Objective:           "deliver " + successorSuffix,
		CompletionCriteria:  []string{"successor verified"},
		ReplacesExecutionID: old.Execution.ID,
		Status:              protocol.ExecutionStatusActive,
	}
	plan := testPlanCommand(successorSuffix, 1, successorSuffix, "", 1)
	return ReplaceWithPlanCommand{
		ExecutionID:              old.Execution.ID,
		ExpectedExecutionVersion: old.Execution.Version,
		Successor:                successor,
		Plan:                     plan,
		Reason:                   "user changed objective",
		Meta:                     testMeta("replace-" + successorSuffix),
		SuccessorMeta:            testMeta("create-" + successorSuffix),
	}
}

func assertTerminalizedExecutionGraph(
	t *testing.T,
	repository *Repository,
	executionID string,
	executionStatus protocol.ExecutionStatus,
	planStatus protocol.PlanRevisionStatus,
	workStatus protocol.WorkItemStatus,
	attemptStatus protocol.WorkAttemptStatus,
) {
	t.Helper()
	execution, err := repository.Get(t.Context(), executionID)
	if err != nil {
		t.Fatal(err)
	}
	if execution == nil || execution.Status != executionStatus {
		t.Fatalf("Execution status = %#v, want %s", execution, executionStatus)
	}
	var gotPlan string
	if err = repository.db.QueryRow(
		`SELECT status FROM execution_plan_revisions WHERE execution_id = ?`,
		executionID,
	).Scan(&gotPlan); err != nil {
		t.Fatal(err)
	}
	if gotPlan != string(planStatus) {
		t.Fatalf("Plan status = %q, want %s", gotPlan, planStatus)
	}
	var nonterminalStates int
	if err = repository.db.QueryRow(
		`SELECT COUNT(1) FROM execution_work_item_states WHERE execution_id = ? AND status <> ?`,
		executionID,
		workStatus,
	).Scan(&nonterminalStates); err != nil {
		t.Fatal(err)
	}
	if nonterminalStates != 0 {
		t.Fatalf("nonterminal Work Item states = %d", nonterminalStates)
	}
	var currentAssignments int
	if err = repository.db.QueryRow(
		`SELECT COUNT(1)
		 FROM execution_work_assignments
		 WHERE execution_id = ?
		   AND status IN ('assigned', 'active')`,
		executionID,
	).Scan(&currentAssignments); err != nil {
		t.Fatal(err)
	}
	if currentAssignments != 0 {
		t.Fatalf("current Assignments after terminalization = %d", currentAssignments)
	}
	var liveAttempts int
	if err = repository.db.QueryRow(
		`SELECT COUNT(1)
		 FROM execution_attempts
		 WHERE execution_id = ?
		   AND status IN ('pending', 'running')`,
		executionID,
	).Scan(&liveAttempts); err != nil {
		t.Fatal(err)
	}
	if liveAttempts != 0 {
		t.Fatalf("live Attempts after terminalization = %d", liveAttempts)
	}
	var expectedTerminalAttempts int
	if err = repository.db.QueryRow(
		`SELECT COUNT(1)
		 FROM execution_attempts
		 WHERE execution_id = ? AND status = ?`,
		executionID,
		attemptStatus,
	).Scan(&expectedTerminalAttempts); err != nil {
		t.Fatal(err)
	}
	if expectedTerminalAttempts == 0 {
		t.Fatalf("no Attempt terminalized as %s", attemptStatus)
	}
}

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

func TestRepositoryRecordEvidencePersistsFlagAndIsCommandIdempotent(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("evidence"))
	if err != nil {
		t.Fatal(err)
	}
	command := RecordEvidenceCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		MetadataKey:              "goal_promotion_context_boundary_evidence",
		Meta:                     testMeta("evidence-context-boundary"),
	}
	recorded, err := repository.RecordEvidence(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Execution.Version != 2 ||
		recorded.Execution.Metadata[command.MetadataKey] != true {
		t.Fatalf("recorded Execution = %#v", recorded.Execution)
	}
	replayed, err := repository.RecordEvidence(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != 2 ||
		replayed.Execution.Metadata[command.MetadataKey] != true {
		t.Fatalf("replayed Execution = %#v", replayed.Execution)
	}
	var eventType string
	var actorKind string
	var evidenceKey string
	if err = repository.db.QueryRow(`
SELECT event_type, actor_kind, json_extract(payload_json, '$.evidence_key')
FROM execution_events
WHERE execution_id = ? AND command_id = ?`,
		snapshot.Execution.ID,
		command.Meta.CommandID,
	).Scan(&eventType, &actorKind, &evidenceKey); err != nil {
		t.Fatal(err)
	}
	if eventType != string(protocol.ExecutionEventEvidenceRecorded) ||
		actorKind != string(protocol.ExecutionActorSystem) ||
		evidenceKey != command.MetadataKey {
		t.Fatalf("event = %q/%q/%q", eventType, actorKind, evidenceKey)
	}
}

func TestGoalConfirmationReceiptSurvivesRepositoryRestartAndConverges(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("goal-confirmation-restart"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status)
VALUES (?, ?, ?, 'active')`,
		"goal-confirmation-restart",
		snapshot.Execution.SessionKey,
		snapshot.Execution.Objective,
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.BindGoal(ctx, BindGoalCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Execution: protocol.Execution{
			ID:                    snapshot.Execution.ID,
			GoalID:                "goal-confirmation-restart",
			GoalObjectiveRevision: 3,
			GoalActivationOrigin:  protocol.GoalActivationOriginAdaptivePromoted,
			GoalActivationReason:  protocol.GoalActivationReasonObservedBoundary,
		},
		Meta: testMeta("bind-goal-confirmation-restart"),
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := repository.GetGoalConfirmationReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.State != GoalConfirmationPending ||
		receipt.GoalID != "goal-confirmation-restart" ||
		receipt.GoalObjectiveRevision != 3 ||
		len(receipt.CompletionCriteria) != 1 ||
		receipt.CompletionCriteria[0] != "verified" ||
		receipt.AttemptCount != 0 || receipt.NextAttemptAt == nil {
		t.Fatalf("pending receipt = %#v", receipt)
	}
	deadlines, err := repository.OrchestrationRecoveryDeadlines(ctx)
	if err != nil || deadlines.GoalConfirmation == nil ||
		!deadlines.GoalConfirmation.Equal(*receipt.NextAttemptAt) {
		t.Fatalf("Goal confirmation deadline = %+v, err=%v", deadlines, err)
	}

	// A fresh Repository value represents process restart: no request or Plan
	// proposal state is needed to rediscover the pending confirmation.
	restarted := NewSQLRepository("sqlite", repository.db)
	now := receipt.NextAttemptAt.Add(time.Minute)
	restarted.now = func() time.Time { return now }
	due, err := restarted.ListRecoverableGoalConfirmations(ctx, ListRecoverableGoalConfirmationsQuery{
		Now:   now,
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ExecutionID != snapshot.Execution.ID {
		t.Fatalf("restart recovery scan = %#v", due)
	}

	nextAttemptAt := now.Add(time.Minute)
	receipt, err = restarted.ScheduleGoalConfirmationRetry(ctx, MarkGoalConfirmationCommand{
		ExecutionID:           snapshot.Execution.ID,
		GoalID:                snapshot.Execution.GoalID,
		GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
		NextAttemptAt:         &nextAttemptAt,
		LastError:             "Goal service temporarily unavailable",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != GoalConfirmationPending || receipt.AttemptCount != 1 ||
		receipt.LastError == "" || receipt.NextAttemptAt == nil ||
		!receipt.NextAttemptAt.Equal(nextAttemptAt) {
		t.Fatalf("scheduled receipt = %#v", receipt)
	}
	deadlines, err = restarted.OrchestrationRecoveryDeadlines(ctx)
	if err != nil || deadlines.GoalConfirmation == nil ||
		!deadlines.GoalConfirmation.Equal(nextAttemptAt) {
		t.Fatalf("retry Goal confirmation deadline = %+v, err=%v", deadlines, err)
	}

	receipt, err = restarted.MarkGoalConfirmationConfirmed(ctx, MarkGoalConfirmationCommand{
		ExecutionID:           snapshot.Execution.ID,
		GoalID:                snapshot.Execution.GoalID,
		GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != GoalConfirmationConfirmed || receipt.AttemptCount != 2 ||
		receipt.LastError != "" || receipt.NextAttemptAt != nil || receipt.ConfirmedAt == nil {
		t.Fatalf("confirmed receipt = %#v", receipt)
	}
	deadlines, err = restarted.OrchestrationRecoveryDeadlines(ctx)
	if err != nil || deadlines.GoalConfirmation != nil {
		t.Fatalf("confirmed Goal confirmation deadline = %+v, err=%v", deadlines, err)
	}
	replayed, err := restarted.MarkGoalConfirmationConfirmed(ctx, MarkGoalConfirmationCommand{
		ExecutionID:           snapshot.Execution.ID,
		GoalID:                snapshot.Execution.GoalID,
		GoalObjectiveRevision: snapshot.Execution.GoalObjectiveRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != receipt.Version || replayed.AttemptCount != receipt.AttemptCount {
		t.Fatalf("idempotent confirmation changed receipt: first=%#v replay=%#v", receipt, replayed)
	}
	due, err = restarted.ListRecoverableGoalConfirmations(ctx, ListRecoverableGoalConfirmationsQuery{
		Now:   nextAttemptAt.Add(time.Minute),
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("confirmed receipt remained recoverable: %#v", due)
	}
}

func TestGoalConfirmationMigrationBackfillsOnlyExactPendingBinding(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "goal-confirmation-migration.db")
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	ensureGooseSQLiteDialect(t)
	if err = goose.UpTo(db, orchestrationMigrationDir(t, "sqlite"), 87); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		goalID              string
		executionID         string
		metadataExecutionID string
		metadataRevision    int
		state               string
	}{
		{
			goalID:              "goal-legacy-pending",
			executionID:         "execution-legacy-pending",
			metadataExecutionID: "execution-legacy-pending",
			metadataRevision:    2,
			state:               "pending",
		},
		{
			goalID:              "goal-legacy-confirmed",
			executionID:         "execution-legacy-confirmed",
			metadataExecutionID: "execution-legacy-confirmed",
			metadataRevision:    2,
			state:               "confirmed",
		},
		{
			goalID:              "goal-legacy-stale-execution",
			executionID:         "execution-legacy-stale-execution",
			metadataExecutionID: "execution-replaced",
			metadataRevision:    2,
			state:               "pending",
		},
		{
			goalID:              "goal-legacy-stale-revision",
			executionID:         "execution-legacy-stale-revision",
			metadataExecutionID: "execution-legacy-stale-revision",
			metadataRevision:    3,
			state:               "pending",
		},
	} {
		metadata := `{"objective_revision":` + fmt.Sprint(fixture.metadataRevision) +
			`,"execution_id":"` + fixture.metadataExecutionID +
			`","execution_binding_state":"` + fixture.state + `"}`
		if _, err = db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (?, ?, 'legacy objective', 'active', ?)`,
			fixture.goalID,
			"agent:nexus:workspace:dm:"+fixture.goalID,
			metadata,
		); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`
INSERT INTO executions (
    execution_id, owner_user_id, session_key, scope_kind,
    coordinator_agent_id, origin, objective, completion_criteria_json,
    goal_id, goal_objective_revision, goal_activation_origin,
    goal_activation_reason, status, version, metadata_json
) VALUES (?, 'owner-1', ?, 'dm', 'agent-lead', 'user_request',
          'legacy objective', '["verified"]', ?, 2, 'adaptive_promoted',
          'observed_boundary', 'active', 1, '{}')`,
			fixture.executionID,
			"agent:nexus:workspace:dm:"+fixture.goalID,
			fixture.goalID,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES ('goal-legacy-invalid-metadata', 'agent:nexus:workspace:dm:goal-legacy-invalid-metadata',
        'legacy objective', 'active', 'not-json')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO executions (
    execution_id, owner_user_id, session_key, scope_kind,
    coordinator_agent_id, origin, objective, completion_criteria_json,
    goal_id, goal_objective_revision, goal_activation_origin,
    goal_activation_reason, status, version, metadata_json
) VALUES ('execution-legacy-invalid-metadata', 'owner-1',
          'agent:nexus:workspace:dm:goal-legacy-invalid-metadata', 'dm',
          'agent-lead', 'user_request', 'legacy objective', '["verified"]',
          'goal-legacy-invalid-metadata', 2, 'adaptive_promoted',
          'observed_boundary', 'active', 1, '{}')`); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, orchestrationMigrationDir(t, "sqlite")); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLRepository("sqlite", db)
	pending, err := repository.GetGoalConfirmationReceipt(ctx, "execution-legacy-pending")
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := repository.GetGoalConfirmationReceipt(ctx, "execution-legacy-confirmed")
	if err != nil {
		t.Fatal(err)
	}
	staleExecution, err := repository.GetGoalConfirmationReceipt(
		ctx,
		"execution-legacy-stale-execution",
	)
	if err != nil {
		t.Fatal(err)
	}
	staleRevision, err := repository.GetGoalConfirmationReceipt(
		ctx,
		"execution-legacy-stale-revision",
	)
	if err != nil {
		t.Fatal(err)
	}
	invalidMetadata, err := repository.GetGoalConfirmationReceipt(
		ctx,
		"execution-legacy-invalid-metadata",
	)
	if err != nil {
		t.Fatal(err)
	}
	if pending == nil || pending.State != GoalConfirmationPending ||
		confirmed != nil || staleExecution != nil || staleRevision != nil ||
		invalidMetadata != nil {
		t.Fatalf(
			"migration receipts: pending=%#v confirmed=%#v stale_execution=%#v stale_revision=%#v invalid_metadata=%#v",
			pending,
			confirmed,
			staleExecution,
			staleRevision,
			invalidMetadata,
		)
	}

	foreignKeyRows, err := db.Query(`PRAGMA foreign_key_check('execution_goal_confirmations')`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = foreignKeyRows.Close() }()
	if foreignKeyRows.Next() {
		var table, parent string
		var rowID, foreignKeyID int64
		if err = foreignKeyRows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			t.Fatal(err)
		}
		t.Fatalf(
			"migration left foreign key violation: table=%s row_id=%d parent=%s fk_id=%d",
			table,
			rowID,
			parent,
			foreignKeyID,
		)
	}
	if err = foreignKeyRows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryGoalRevisionSupersedeAndSuccessorPlan(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	oldCreate := createTestCommand("goal-revision-old")
	if _, err := repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (?, ?, ?, 'active', ?)`,
		"goal-revision",
		oldCreate.Execution.SessionKey,
		oldCreate.Execution.Objective,
		`{"objective_revision":1,"activation_origin":"adaptive_promoted","activation_reason":"observed_boundary"}`,
	); err != nil {
		t.Fatal(err)
	}
	oldCreate.Execution.GoalID = "goal-revision"
	oldCreate.Execution.GoalObjectiveRevision = 1
	oldCreate.Execution.GoalActivationOrigin = protocol.GoalActivationOriginAdaptivePromoted
	oldCreate.Execution.GoalActivationReason = protocol.GoalActivationReasonObservedBoundary
	oldPlan := testPlanCommand("goal-revision-old", 1, "goal-revision-old", "", 1)
	old, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: oldCreate.Execution,
		Plan:      oldPlan,
		Meta:      oldCreate.Meta,
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
			"goal-revision-live",
			"agent-worker",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	command := SupersedeGoalRevisionCommand{
		ExecutionID:              old.Execution.ID,
		ExpectedExecutionVersion: old.Execution.Version,
		GoalID:                   old.Execution.GoalID,
		OldGoalObjectiveRevision: 1,
		NewGoalObjectiveRevision: 2,
		SuccessorExecutionID:     "execution-goal-revision-new",
		Reason:                   "user retargeted the Goal",
		Meta:                     testMeta("goal-revision-supersede"),
	}
	terminal, err := repository.SupersedeGoalRevision(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Execution.Status != protocol.ExecutionStatusSuperseded {
		t.Fatalf("terminal status = %s", terminal.Execution.Status)
	}
	assertTerminalizedExecutionGraph(
		t,
		repository,
		old.Execution.ID,
		protocol.ExecutionStatusSuperseded,
		protocol.PlanRevisionStatusSuperseded,
		protocol.WorkItemStatusSuperseded,
		protocol.WorkAttemptStatusInterrupted,
	)
	replayed, err := repository.SupersedeGoalRevision(ctx, command)
	if err != nil || replayed.Execution.Status != protocol.ExecutionStatusSuperseded {
		t.Fatalf("supersede replay = %#v, err=%v", replayed, err)
	}
	conflictingReplay := command
	conflictingReplay.SuccessorExecutionID = "execution-goal-revision-other"
	if _, err = repository.SupersedeGoalRevision(ctx, conflictingReplay); !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("conflicting supersede replay error = %v, want ErrCommandConflict", err)
	}
	differentCommand := command
	differentCommand.Meta = testMeta("goal-revision-supersede-other")
	if _, err = repository.SupersedeGoalRevision(ctx, differentCommand); !errors.Is(err, ErrInvariant) {
		t.Fatalf("terminal supersede without matching command error = %v, want ErrInvariant", err)
	}

	successorCreate := createTestCommand("goal-revision-new")
	successorCreate.Execution.OwnerUserID = old.Execution.OwnerUserID
	successorCreate.Execution.SessionKey = old.Execution.SessionKey
	successorCreate.Execution.ScopeKind = old.Execution.ScopeKind
	successorCreate.Execution.RoomID = old.Execution.RoomID
	successorCreate.Execution.ConversationID = old.Execution.ConversationID
	successorCreate.Execution.GoalID = old.Execution.GoalID
	successorCreate.Execution.GoalObjectiveRevision = 2
	successorCreate.Execution.GoalActivationOrigin = old.Execution.GoalActivationOrigin
	successorCreate.Execution.GoalActivationReason = old.Execution.GoalActivationReason
	successorCreate.Execution.ReplacesExecutionID = old.Execution.ID
	successorCreate.Execution.Objective = "retargeted Goal objective"
	successorPlan := testPlanCommand("goal-revision-new", 1, "goal-revision-new", "", 1)

	unreservedCreate := successorCreate
	unreservedCreate.Execution.ID = "execution-goal-revision-unreserved"
	unreservedPlan := testPlanCommand(
		"goal-revision-unreserved",
		1,
		"goal-revision-unreserved",
		"",
		1,
	)
	if _, err = repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: unreservedCreate.Execution,
		Plan:      unreservedPlan,
		Meta:      testMeta("create-goal-revision-unreserved"),
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("unreserved Goal successor error = %v, want ErrInvariant", err)
	}

	successor, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: successorCreate.Execution,
		Plan:      successorPlan,
		Meta:      successorCreate.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	if successor.Execution.GoalObjectiveRevision != 2 ||
		successor.Execution.ReplacesExecutionID != old.Execution.ID ||
		successor.Plan == nil {
		t.Fatalf("Goal revision successor = %#v", successor)
	}

	invalidCreate := createTestCommand("goal-revision-invalid")
	invalidCreate.Execution.OwnerUserID = old.Execution.OwnerUserID
	invalidCreate.Execution.SessionKey = old.Execution.SessionKey
	invalidCreate.Execution.ScopeKind = old.Execution.ScopeKind
	invalidCreate.Execution.RoomID = old.Execution.RoomID
	invalidCreate.Execution.ConversationID = old.Execution.ConversationID
	invalidCreate.Execution.GoalID = old.Execution.GoalID
	invalidCreate.Execution.GoalObjectiveRevision = 4
	invalidCreate.Execution.GoalActivationOrigin = old.Execution.GoalActivationOrigin
	invalidCreate.Execution.GoalActivationReason = old.Execution.GoalActivationReason
	invalidCreate.Execution.ReplacesExecutionID = old.Execution.ID
	invalidPlan := testPlanCommand("goal-revision-invalid", 1, "goal-revision-invalid", "", 1)
	if _, err = repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: invalidCreate.Execution,
		Plan:      invalidPlan,
		Meta:      invalidCreate.Meta,
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("invalid Goal successor error = %v, want ErrInvariant", err)
	}
}

func TestRepositoryGoalRevisionTerminalReservationIsConcurrentAndIdempotent(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	oldCreate := createTestCommand("goal-revision-terminal-race")
	if _, err := repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (?, ?, ?, 'active', ?)`,
		"goal-revision-terminal-race",
		oldCreate.Execution.SessionKey,
		oldCreate.Execution.Objective,
		`{"objective_revision":1,"activation_origin":"adaptive_promoted","activation_reason":"observed_boundary"}`,
	); err != nil {
		t.Fatal(err)
	}
	oldCreate.Execution.GoalID = "goal-revision-terminal-race"
	oldCreate.Execution.GoalObjectiveRevision = 1
	oldCreate.Execution.GoalActivationOrigin = protocol.GoalActivationOriginAdaptivePromoted
	oldCreate.Execution.GoalActivationReason = protocol.GoalActivationReasonObservedBoundary
	old, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: oldCreate.Execution,
		Plan:      testPlanCommand("goal-revision-terminal-race", 1, "goal-revision-terminal-race", "", 1),
		Meta:      oldCreate.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.db.Exec(`
UPDATE executions SET status = 'completed', completed_at = CURRENT_TIMESTAMP
WHERE execution_id = ?`, old.Execution.ID); err != nil {
		t.Fatal(err)
	}
	completed, err := repository.GetSnapshot(ctx, old.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	command := SupersedeGoalRevisionCommand{
		ExecutionID:              completed.Execution.ID,
		ExpectedExecutionVersion: completed.Execution.Version,
		GoalID:                   completed.Execution.GoalID,
		OldGoalObjectiveRevision: 1,
		NewGoalObjectiveRevision: 2,
		SuccessorExecutionID:     "execution-goal-revision-terminal-race-new",
		Reason:                   "concurrent terminal retarget",
		Meta:                     testMeta("goal-revision-terminal-race-supersede"),
	}
	start := make(chan struct{})
	errorsByCall := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, callErr := repository.SupersedeGoalRevision(ctx, command)
			errorsByCall <- callErr
		}()
	}
	close(start)
	wait.Wait()
	close(errorsByCall)
	for callErr := range errorsByCall {
		if callErr != nil {
			t.Fatalf("concurrent terminal retarget error = %v", callErr)
		}
	}
	var reservations int
	if err = repository.db.QueryRow(`
SELECT COUNT(*) FROM execution_events
WHERE execution_id = ? AND command_id = ? AND event_type = ?`,
		command.ExecutionID,
		command.Meta.CommandID,
		protocol.ExecutionEventSuperseded,
	).Scan(&reservations); err != nil {
		t.Fatal(err)
	}
	if reservations != 1 {
		t.Fatalf("terminal successor reservation events = %d, want 1", reservations)
	}
	stored, err := repository.GetSnapshot(ctx, command.ExecutionID)
	if err != nil || stored.Execution.Status != protocol.ExecutionStatusCompleted ||
		stored.Execution.Version != completed.Execution.Version+1 {
		t.Fatalf("terminal predecessor after race = %+v err=%v", stored, err)
	}
}

func TestRepositoryFencesOrphanGoalExecutionReservationBeforeDelayedCreate(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	orphanCreate := createTestCommand("goal-orphan-old")
	if _, err := repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (?, ?, ?, 'active', ?)`,
		"goal-orphan",
		orphanCreate.Execution.SessionKey,
		orphanCreate.Execution.Objective,
		`{"objective_revision":1,"activation_origin":"adaptive_initial","activation_reason":"context_boundary"}`,
	); err != nil {
		t.Fatal(err)
	}
	orphanCreate.Execution.GoalID = "goal-orphan"
	orphanCreate.Execution.GoalObjectiveRevision = 1
	orphanCreate.Execution.GoalActivationOrigin = protocol.GoalActivationOriginAdaptiveInitial
	orphanCreate.Execution.GoalActivationReason = protocol.GoalActivationReasonContextBoundary
	fence := FenceGoalExecutionIdentityCommand{
		ExecutionID:           orphanCreate.Execution.ID,
		ExpectedOwnerUserID:   orphanCreate.Execution.OwnerUserID,
		GoalID:                orphanCreate.Execution.GoalID,
		GoalObjectiveRevision: 1,
		SuccessorExecutionID:  "execution-goal-orphan-new",
		Meta:                  testMeta("goal-orphan-fence"),
	}
	fenced, err := repository.FenceGoalExecutionIdentity(ctx, fence)
	if err != nil || !fenced {
		t.Fatalf("FenceGoalExecutionIdentity() fenced=%t err=%v", fenced, err)
	}
	replayed, err := repository.FenceGoalExecutionIdentity(ctx, fence)
	if err != nil || !replayed {
		t.Fatalf("fence replay fenced=%t err=%v", replayed, err)
	}

	orphanPlan := testPlanCommand("goal-orphan-old", 1, "goal-orphan-old", "", 1)
	if _, err = repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: orphanCreate.Execution,
		Plan:      orphanPlan,
		Meta:      orphanCreate.Meta,
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("delayed orphan materialization error = %v, want ErrInvariant", err)
	}

	successorCreate := createTestCommand("goal-orphan-new")
	successorCreate.Execution.ID = fence.SuccessorExecutionID
	successorCreate.Execution.OwnerUserID = orphanCreate.Execution.OwnerUserID
	successorCreate.Execution.SessionKey = orphanCreate.Execution.SessionKey
	successorCreate.Execution.GoalID = orphanCreate.Execution.GoalID
	successorCreate.Execution.GoalObjectiveRevision = 2
	successorCreate.Execution.GoalActivationOrigin = orphanCreate.Execution.GoalActivationOrigin
	successorCreate.Execution.GoalActivationReason = orphanCreate.Execution.GoalActivationReason
	successorCreate.Execution.Objective = "new objective after fenced reservation"
	successorPlan := testPlanCommand("goal-orphan-new", 1, "goal-orphan-new", "", 1)
	successor, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: successorCreate.Execution,
		Plan:      successorPlan,
		Meta:      successorCreate.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	if successor.Execution.ID != fence.SuccessorExecutionID ||
		successor.Execution.ReplacesExecutionID != "" ||
		successor.Execution.GoalObjectiveRevision != 2 {
		t.Fatalf("fenced successor = %#v", successor.Execution)
	}
}
