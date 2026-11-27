// repository_test.go 覆盖编排仓储 CRUD、方言初始化、恢复扫描与投影限额测试。
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// goose.SetDialect writes an unsynchronized package-level global in goose v3,
// which data-races when t.Parallel() tests run migrations concurrently.
// Set it once per test package instead of in every test helper.
var (
	gooseDialectOnce sync.Once
	gooseDialectErr  error
)

func ensureGooseSQLiteDialect(t *testing.T) {
	t.Helper()
	gooseDialectOnce.Do(func() {
		gooseDialectErr = goose.SetDialect("sqlite3")
	})
	if gooseDialectErr != nil {
		t.Fatal(gooseDialectErr)
	}
}

func TestRepositoryPlanDAGClaimsAndCommandIdempotency(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("dag"))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Version != 1 {
		t.Fatalf("created version = %d, want 1", snapshot.Execution.Version)
	}
	replayed, err := repository.Create(ctx, createTestCommand("dag"))
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != 1 {
		t.Fatalf("replayed create version = %d, want 1", replayed.Execution.Version)
	}

	cyclic := testPlanCommand("dag", 1, "cycle", "", 1)
	cyclic.Dependencies = []protocol.ExecutionPlanDependency{
		{WorkItemID: "work-cycle-1", DependsOnWorkItemID: "work-cycle-2", Kind: protocol.WorkDependencyHard},
		{WorkItemID: "work-cycle-2", DependsOnWorkItemID: "work-cycle-1", Kind: protocol.WorkDependencyHard},
	}
	if _, err = repository.WritePlan(ctx, cyclic); !errors.Is(err, ErrInvariant) {
		t.Fatalf("cyclic Plan error = %v, want ErrInvariant", err)
	}
	current, err := repository.Get(ctx, "execution-dag")
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != 1 {
		t.Fatalf("failed command changed Execution version to %d", current.Version)
	}

	valid := testPlanCommand("dag", 1, "main", "", 1)
	valid.Dependencies = []protocol.ExecutionPlanDependency{{
		WorkItemID:          "work-main-2",
		DependsOnWorkItemID: "work-main-1",
		Kind:                protocol.WorkDependencyHard,
	}}
	snapshot, err = repository.WritePlan(ctx, valid)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Version != 2 || snapshot.Plan == nil || snapshot.Plan.ID != "plan-main" {
		t.Fatalf("valid Plan snapshot = %#v", snapshot)
	}
	if len(snapshot.ReadyWorkItemIDs) != 1 || snapshot.ReadyWorkItemIDs[0] != "work-main-1" {
		t.Fatalf("ready = %#v, want work-main-1", snapshot.ReadyWorkItemIDs)
	}
	replayed, err = repository.WritePlan(ctx, valid)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != 2 {
		t.Fatalf("replayed Plan version = %d, want 2", replayed.Execution.Version)
	}

	conflict := testPlanCommand("dag", 2, "claim", "plan-main", 2)
	conflict.WorkItems[0].OutputClaims[0].Scope = "dir:shared/output"
	conflict.WorkItems[1].OutputClaims[0].Scope = "file:shared/output/result.md"
	conflict.WorkItems[0].OutputClaims[0].Mode = protocol.WorkOutputScopeExclusive
	conflict.WorkItems[1].OutputClaims[0].Mode = protocol.WorkOutputScopeShared
	if _, err = repository.WritePlan(ctx, conflict); !errors.Is(err, ErrInvariant) {
		t.Fatalf("conflicting output error = %v, want ErrInvariant", err)
	}
	current, err = repository.Get(ctx, "execution-dag")
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != 2 {
		t.Fatalf("claim conflict changed Execution version to %d", current.Version)
	}
	assertEventSequence(t, repository.db, "execution-dag", 2)
}

func TestRepositoryBindGoalAndBlockWork(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("binding"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status)
VALUES (?, ?, ?, 'active')`,
		"goal-binding", snapshot.Execution.SessionKey, "persist binding",
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.BindGoal(ctx, BindGoalCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Execution: protocol.Execution{
			ID:                    snapshot.Execution.ID,
			GoalID:                "goal-binding",
			GoalObjectiveRevision: 1,
			GoalActivationOrigin:  protocol.GoalActivationOriginAdaptivePromoted,
			GoalActivationReason:  protocol.GoalActivationReasonContextBoundary,
		},
		Meta: testMeta("bind-goal"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.GoalID != "goal-binding" ||
		snapshot.Execution.GoalActivationReason != protocol.GoalActivationReasonContextBoundary {
		t.Fatalf("Goal binding = %#v", snapshot.Execution)
	}
	goalExecution, err := repository.FindCurrentByGoal(ctx, "goal-binding", 1)
	if err != nil {
		t.Fatal(err)
	}
	if goalExecution == nil || goalExecution.ID != snapshot.Execution.ID {
		t.Fatalf("Goal Execution = %#v, want %s", goalExecution, snapshot.Execution.ID)
	}
	other, err := repository.Create(ctx, createTestCommand("binding-conflict"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.BindGoal(ctx, BindGoalCommand{
		ExpectedExecutionVersion: other.Execution.Version,
		Execution: protocol.Execution{
			ID:                    other.Execution.ID,
			GoalID:                "goal-binding",
			GoalObjectiveRevision: 1,
			GoalActivationOrigin:  protocol.GoalActivationOriginAdaptivePromoted,
			GoalActivationReason:  protocol.GoalActivationReasonContextBoundary,
		},
		Meta: testMeta("bind-goal-conflict"),
	})
	if !errors.Is(err, ErrCommandConflict) {
		t.Fatalf("second Goal revision binding error = %v, want command conflict", err)
	}
	plan := testPlanCommand("binding", snapshot.Execution.Version, "block", "", 1)
	plan.Dependencies = []protocol.ExecutionPlanDependency{{
		WorkItemID:          "work-block-2",
		DependsOnWorkItemID: "work-block-1",
		Kind:                protocol.WorkDependencyHard,
	}}
	snapshot, err = repository.WritePlan(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	block := BlockCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     1,
		State: protocol.WorkItemState{
			WorkItemID:    "work-block-1",
			ExecutionID:   "execution-binding",
			CurrentSpecID: "spec-block-1",
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   "external approval",
			NeededInput:   "approval",
		},
		Meta: testMeta("block-work"),
	}
	snapshot, err = repository.Block(ctx, block)
	if err != nil {
		t.Fatal(err)
	}
	if contains(snapshot.ReadyWorkItemIDs, "work-block-1") {
		t.Fatalf("blocked Work Item remains ready: %#v", snapshot.ReadyWorkItemIDs)
	}
	replayed, err := repository.Block(ctx, block)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != snapshot.Execution.Version {
		t.Fatalf("replayed block version = %d, want %d", replayed.Execution.Version, snapshot.Execution.Version)
	}
}

func TestRepositoryPlanRevisionRequiresOptInAndReleasesActiveWorkAtomically(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("revision"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("revision", snapshot.Execution.Version, "revision-old", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(
		snapshot,
		"work-revision-old-1",
		"spec-revision-old-1",
		"revision-old",
		"agent-worker",
	)
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-revision-old",
		DedupeKey:     "dispatch-revision-old",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "old plan",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.ClaimDispatch(
		ctx,
		"dispatch-revision-old",
		1,
		"worker-revision",
		time.Minute,
	); err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		"assignment-revision-old",
		"attempt-revision-old",
	)
	versionBefore := snapshot.Execution.Version
	replacement := testPlanCommand(
		"revision",
		versionBefore,
		"revision-new",
		"plan-revision-old",
		2,
	)
	replacement.Plan.RevisionReason = "scope changed"
	reused := &replacement.WorkItems[0]
	reused.WorkItem = protocol.WorkItem{
		ID:          "work-revision-old-1",
		ExecutionID: "execution-revision",
		LogicalKey:  "work-revision-old-1",
		Kind:        protocol.WorkItemKindProduce,
	}
	reused.Spec.WorkItemID = reused.WorkItem.ID
	reused.Spec.Version = 2
	reused.State.WorkItemID = reused.WorkItem.ID
	reused.State.CurrentSpecID = reused.Spec.ID
	reused.Item.WorkItemID = reused.WorkItem.ID
	reused.Item.SpecID = reused.Spec.ID
	reused.ExpectedStateVersion = findState(t, snapshot, reused.WorkItem.ID).Version
	for index := range reused.OutputClaims {
		reused.OutputClaims[index].WorkItemID = reused.WorkItem.ID
		reused.OutputClaims[index].SpecID = reused.Spec.ID
	}
	if _, err = repository.WritePlan(ctx, replacement); !errors.Is(err, ErrInvariant) {
		t.Fatalf("replacement without opt-in error = %v, want ErrInvariant", err)
	}
	unchanged, err := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Execution.Version != versionBefore ||
		unchanged.Plan == nil ||
		unchanged.Plan.ID != "plan-revision-old" ||
		findAssignment(t, unchanged, "assignment-revision-old").Status != protocol.WorkAssignmentStatusActive ||
		findAttempt(t, unchanged, "attempt-revision-old").Status != protocol.WorkAttemptStatusRunning ||
		findDispatch(t, unchanged, "dispatch-revision-old").Status != protocol.ExecutionDispatchStatusClaimed {
		t.Fatalf("failed replacement was not atomic = %#v", unchanged)
	}

	replacement.SupersedeActiveWork = true
	replacement.Meta = testMeta("plan-revision-new-opt-in")
	updated, err := repository.WritePlan(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Plan == nil || updated.Plan.ID != "plan-revision-new" ||
		updated.Plan.BasePlanID != "plan-revision-old" {
		t.Fatalf("active replacement Plan = %#v", updated.Plan)
	}
	var (
		oldPlanStatus   string
		assignmentState string
		releasedAt      sql.NullTime
		attemptState    string
		failureReason   string
		dispatchState   string
		leaseOwner      sql.NullString
		currentSpecID   string
	)
	if err = repository.db.QueryRow(
		`SELECT status FROM execution_plan_revisions WHERE plan_id = ?`,
		"plan-revision-old",
	).Scan(&oldPlanStatus); err != nil {
		t.Fatal(err)
	}
	if err = repository.db.QueryRow(
		`SELECT status, released_at FROM execution_work_assignments WHERE assignment_id = ?`,
		"assignment-revision-old",
	).Scan(&assignmentState, &releasedAt); err != nil {
		t.Fatal(err)
	}
	if err = repository.db.QueryRow(
		`SELECT status, failure_reason FROM execution_attempts WHERE attempt_id = ?`,
		"attempt-revision-old",
	).Scan(&attemptState, &failureReason); err != nil {
		t.Fatal(err)
	}
	if err = repository.db.QueryRow(
		`SELECT status, lease_owner FROM execution_dispatches WHERE dispatch_id = ?`,
		"dispatch-revision-old",
	).Scan(&dispatchState, &leaseOwner); err != nil {
		t.Fatal(err)
	}
	if err = repository.db.QueryRow(
		`SELECT current_spec_id FROM execution_work_item_states WHERE work_item_id = ?`,
		"work-revision-old-1",
	).Scan(&currentSpecID); err != nil {
		t.Fatal(err)
	}
	if oldPlanStatus != string(protocol.PlanRevisionStatusSuperseded) ||
		assignmentState != string(protocol.WorkAssignmentStatusReleased) ||
		!releasedAt.Valid ||
		attemptState != string(protocol.WorkAttemptStatusInterrupted) ||
		failureReason != "plan superseded: scope changed" ||
		dispatchState != string(protocol.ExecutionDispatchStatusCancelled) ||
		leaseOwner.Valid ||
		currentSpecID != "spec-revision-new-1" {
		t.Fatalf(
			"replacement closure plan=%s assignment=%s released=%#v attempt=%s reason=%q dispatch=%s lease=%#v spec=%s",
			oldPlanStatus,
			assignmentState,
			releasedAt,
			attemptState,
			failureReason,
			dispatchState,
			leaseOwner,
			currentSpecID,
		)
	}
	replayed, err := repository.WritePlan(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != updated.Execution.Version ||
		replayed.Plan == nil ||
		replayed.Plan.ID != updated.Plan.ID {
		t.Fatalf("replayed replacement = %#v", replayed)
	}
}

func newRepositoryTestStore(t *testing.T) *Repository {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "repository.db")
	migrationDB, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	ensureGooseSQLiteDialect(t)
	if err = goose.Up(migrationDB, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if err = migrationDB.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	foreignKeyRows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if foreignKeyRows.Next() {
		var table, parent string
		var rowID, foreignKeyID int64
		if err = foreignKeyRows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			_ = foreignKeyRows.Close()
			t.Fatal(err)
		}
		_ = foreignKeyRows.Close()
		t.Fatalf(
			"orchestration migrations left broken foreign key: table=%s row=%d parent=%s fk=%d",
			table,
			rowID,
			parent,
			foreignKeyID,
		)
	}
	if err = foreignKeyRows.Close(); err != nil {
		t.Fatal(err)
	}
	repository := NewSQLRepository("sqlite", db)
	current := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	repository.now = func() time.Time {
		current = current.Add(time.Second)
		return current
	}
	return repository
}

func createTestCommand(suffix string) CreateCommand {
	return CreateCommand{
		Execution: protocol.Execution{
			ID:                 "execution-" + suffix,
			OwnerUserID:        "owner-1",
			SessionKey:         "agent:nexus:workspace:dm:" + suffix,
			ScopeKind:          protocol.ExecutionScopeDM,
			CoordinatorAgentID: "agent-lead",
			Origin:             protocol.ExecutionOriginUserRequest,
			Objective:          "deliver " + suffix,
			CompletionCriteria: []string{"verified"},
			Status:             protocol.ExecutionStatusActive,
		},
		Meta: testMeta("create-" + suffix),
	}
}

func testPlanCommand(executionSuffix string, expected int64, planSuffix string, base string, revision int64) WritePlanCommand {
	executionID := "execution-" + executionSuffix
	planID := "plan-" + planSuffix
	return WritePlanCommand{
		ExecutionID:              executionID,
		ExpectedExecutionVersion: expected,
		Plan: protocol.ExecutionPlanRevision{
			ID:               planID,
			ExecutionID:      executionID,
			Revision:         revision,
			Status:           protocol.PlanRevisionStatusActive,
			BasePlanID:       base,
			CreatedByAgentID: "agent-lead",
			RevisionReason:   "test",
		},
		WorkItems: []PlanWorkItem{
			testPlanWork(executionID, planID, "work-"+planSuffix+"-1", "spec-"+planSuffix+"-1", 0),
			testPlanWork(executionID, planID, "work-"+planSuffix+"-2", "spec-"+planSuffix+"-2", 1),
		},
		Meta: testMeta("plan-" + planSuffix),
	}
}

func testPlanWork(executionID, planID, workID, specID string, position int) PlanWorkItem {
	return PlanWorkItem{
		WorkItem: protocol.WorkItem{
			ID:          workID,
			ExecutionID: executionID,
			LogicalKey:  workID,
			Kind:        protocol.WorkItemKindProduce,
		},
		Spec: protocol.WorkItemSpec{
			ID:                 specID,
			WorkItemID:         workID,
			ExecutionID:        executionID,
			Version:            1,
			Subject:            "Deliver " + workID,
			Objective:          "Complete " + workID,
			Deliverable:        "Verified " + workID,
			AcceptanceCriteria: []string{"verified"},
			SpecHash:           "hash-" + specID,
			CreatedByAgentID:   "agent-lead",
		},
		State: protocol.WorkItemState{
			WorkItemID:    workID,
			ExecutionID:   executionID,
			CurrentSpecID: specID,
			Status:        protocol.WorkItemStatusOpen,
			Version:       1,
		},
		Item: protocol.ExecutionPlanItem{
			PlanID:      planID,
			ExecutionID: executionID,
			WorkItemID:  workID,
			SpecID:      specID,
			Required:    true,
			Position:    position,
		},
		OutputClaims: []protocol.ExecutionPlanOutputClaim{{
			PlanID:      planID,
			ExecutionID: executionID,
			WorkItemID:  workID,
			SpecID:      specID,
			Scope:       "dir:output/" + workID,
			Mode:        protocol.WorkOutputScopeExclusive,
		}},
	}
}

func assignTestCommand(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	specID string,
	suffix string,
	ownerAgentID string,
) AssignCommand {
	assignmentID := "assignment-" + suffix
	attemptID := "attempt-" + suffix
	return AssignCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Assignment: testAssignment(
			snapshot.Execution.ID, snapshot.Plan.ID, workItemID, specID, assignmentID, ownerAgentID,
		),
		RootAttempt: ptrAttempt(testRootAttempt(
			snapshot.Execution.ID, snapshot.Plan.ID, workItemID, specID,
			assignmentID, attemptID, ownerAgentID,
		)),
		Meta: testMeta("assign-" + suffix),
	}
}

func testAssignment(
	executionID string,
	planID string,
	workItemID string,
	specID string,
	assignmentID string,
	ownerAgentID string,
) protocol.WorkAssignment {
	return protocol.WorkAssignment{
		ID:                assignmentID,
		ExecutionID:       executionID,
		PlanID:            planID,
		WorkItemID:        workItemID,
		SpecID:            specID,
		OwnerAgentID:      ownerAgentID,
		AssignedByAgentID: "agent-lead",
		ReturnToAgentID:   "agent-lead",
		Strategy:          protocol.AssignmentStrategySelf,
		Status:            protocol.WorkAssignmentStatusAssigned,
	}
}

func testRootAttempt(
	executionID string,
	planID string,
	workItemID string,
	specID string,
	assignmentID string,
	attemptID string,
	ownerAgentID string,
) protocol.WorkAttempt {
	return protocol.WorkAttempt{
		ID:              attemptID,
		ExecutionID:     executionID,
		PlanID:          planID,
		WorkItemID:      workItemID,
		SpecID:          specID,
		AssignmentID:    assignmentID,
		ExecutorKind:    protocol.AttemptExecutorAgent,
		ExecutorAgentID: ownerAgentID,
		Status:          protocol.WorkAttemptStatusPending,
	}
}

func ptrAttempt(value protocol.WorkAttempt) *protocol.WorkAttempt {
	return &value
}

func startTestAttempt(
	t *testing.T,
	ctx context.Context,
	repository *Repository,
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
	attemptID string,
) *protocol.ExecutionSnapshot {
	t.Helper()
	assignment := findAssignment(t, snapshot, assignmentID)
	attempt := findAttempt(t, snapshot, attemptID)
	result, err := repository.StartAttempt(ctx, StartAttemptCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		ExpectedAttemptVersion:    attempt.Version,
		Attempt:                   attempt,
		Meta:                      testMeta("start-" + attemptID),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func finishTestAttempt(
	t *testing.T,
	ctx context.Context,
	repository *Repository,
	snapshot *protocol.ExecutionSnapshot,
	attemptID string,
	status protocol.WorkAttemptStatus,
) *protocol.ExecutionSnapshot {
	t.Helper()
	attempt := findAttempt(t, snapshot, attemptID)
	attempt.Status = status
	result, err := repository.FinishAttempt(ctx, FinishAttemptCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedAttemptVersion:   attempt.Version,
		Attempt:                  attempt,
		Meta:                     testMeta("finish-" + attemptID),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func submitTestWork(
	t *testing.T,
	ctx context.Context,
	repository *Repository,
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
	attemptID string,
	submissionID string,
	submitterAgentID string,
) *protocol.ExecutionSnapshot {
	t.Helper()
	assignment := findAssignment(t, snapshot, assignmentID)
	result, err := repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission: protocol.WorkSubmission{
			ID:               submissionID,
			ExecutionID:      assignment.ExecutionID,
			PlanID:           assignment.PlanID,
			WorkItemID:       assignment.WorkItemID,
			SpecID:           assignment.SpecID,
			AssignmentID:     assignment.ID,
			AttemptID:        attemptID,
			SubmitterAgentID: submitterAgentID,
			ResultSummary:    "completed " + assignment.WorkItemID,
			Evidence:         []string{"test"},
		},
		Meta: testMeta("submit-" + submissionID),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func reviewTestWork(
	t *testing.T,
	ctx context.Context,
	repository *Repository,
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
	submissionID string,
	acceptanceID string,
	decision protocol.WorkAcceptanceDecision,
) *protocol.ExecutionSnapshot {
	t.Helper()
	assignment := findAssignment(t, snapshot, assignmentID)
	submission := findSubmission(t, snapshot, submissionID)
	result, err := repository.Review(ctx, ReviewCommand{
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
			Decision:     decision,
			ReviewerKind: protocol.WorkReviewerAgent,
			ReviewerID:   "agent-lead",
			CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
				Criterion: "verified",
				Passed:    decision == protocol.WorkAcceptanceAccepted,
			}},
			Feedback: "reviewed",
		},
		Meta: testMeta("review-" + acceptanceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func findAssignment(t *testing.T, snapshot *protocol.ExecutionSnapshot, id string) protocol.WorkAssignment {
	t.Helper()
	for _, item := range snapshot.Assignments {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("Assignment %s not found in %#v", id, snapshot.Assignments)
	return protocol.WorkAssignment{}
}

func findAttempt(t *testing.T, snapshot *protocol.ExecutionSnapshot, id string) protocol.WorkAttempt {
	t.Helper()
	for _, item := range snapshot.Attempts {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("Attempt %s not found in %#v", id, snapshot.Attempts)
	return protocol.WorkAttempt{}
}

func findDispatch(t *testing.T, snapshot *protocol.ExecutionSnapshot, id string) protocol.ExecutionDispatch {
	t.Helper()
	for _, item := range snapshot.Dispatches {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("Dispatch %s not found in %#v", id, snapshot.Dispatches)
	return protocol.ExecutionDispatch{}
}

func findState(t *testing.T, snapshot *protocol.ExecutionSnapshot, workItemID string) protocol.WorkItemState {
	t.Helper()
	for _, item := range snapshot.WorkItemStates {
		if item.WorkItemID == workItemID {
			return item
		}
	}
	t.Fatalf("Work Item state %s not found in %#v", workItemID, snapshot.WorkItemStates)
	return protocol.WorkItemState{}
}

func findSubmission(t *testing.T, snapshot *protocol.ExecutionSnapshot, id string) protocol.WorkSubmission {
	t.Helper()
	for _, item := range snapshot.Submissions {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("Submission %s not found in %#v", id, snapshot.Submissions)
	return protocol.WorkSubmission{}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func assertEventSequence(t *testing.T, db *sql.DB, executionID string, want int) {
	t.Helper()
	rows, err := db.Query(`
SELECT sequence
FROM execution_events
WHERE execution_id = ?
ORDER BY sequence`, executionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	sequence := 0
	for rows.Next() {
		sequence++
		var got int
		if err = rows.Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != sequence {
			t.Fatalf("event sequence[%d] = %d", sequence, got)
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if sequence != want {
		t.Fatalf("event count = %d, want %d", sequence, want)
	}
}

func testMeta(suffix string) CommandMeta {
	return CommandMeta{
		CommandID: "command-" + suffix,
		EventID:   "event-" + suffix,
		ActorKind: protocol.ExecutionActorSystem,
		ActorID:   "test",
	}
}
