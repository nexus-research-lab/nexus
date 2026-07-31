package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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
