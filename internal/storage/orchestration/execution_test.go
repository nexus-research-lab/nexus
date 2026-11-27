// execution_test.go 覆盖编排执行状态迁移、完成审计、证据、Goal 确认与重定目标仓储测试。
package orchestration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
