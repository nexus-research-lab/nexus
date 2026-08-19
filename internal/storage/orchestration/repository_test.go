package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

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

func TestRepositoryEnforcesOneCurrentExecutionPerSession(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	first := createTestCommand("current-a")
	if _, err := repository.Create(ctx, first); err != nil {
		t.Fatal(err)
	}

	second := createTestCommand("current-b")
	second.Execution.SessionKey = first.Execution.SessionKey
	if _, err := repository.Create(ctx, second); err == nil {
		t.Fatal("created two current Executions for the same owner/session")
	}
	current, err := repository.FindCurrent(ctx, first.Execution.OwnerUserID, first.Execution.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != first.Execution.ID {
		t.Fatalf("current Execution = %#v, want %s", current, first.Execution.ID)
	}

	if _, err = repository.db.Exec(
		`UPDATE executions SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE execution_id = ?`,
		first.Execution.ID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.Create(ctx, second); err != nil {
		t.Fatalf("create replacement after terminal Execution: %v", err)
	}
}

func TestRepositoryManagedExecutionViewWaitsForARealReplacementGraph(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	firstCommand := createTestCommand("view-first")
	firstSnapshot, err := repository.Create(ctx, firstCommand)
	if err != nil {
		t.Fatal(err)
	}
	firstSnapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("view-first", firstSnapshot.Execution.Version, "view-first", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.db.Exec(
		`UPDATE executions SET status = 'completed', completed_at = CURRENT_TIMESTAMP WHERE execution_id = ?`,
		firstSnapshot.Execution.ID,
	); err != nil {
		t.Fatal(err)
	}

	secondCommand := createTestCommand("view-second")
	secondCommand.Execution.SessionKey = firstCommand.Execution.SessionKey
	secondSnapshot, err := repository.Create(ctx, secondCommand)
	if err != nil {
		t.Fatal(err)
	}
	currentManaged, err := repository.FindCurrentManaged(
		ctx,
		firstCommand.Execution.OwnerUserID,
		firstCommand.Execution.SessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if currentManaged != nil {
		t.Fatalf("transient replacement appeared as current WorkGraph: %+v", currentManaged)
	}
	latestManaged, err := repository.FindLatestManaged(
		ctx,
		firstCommand.Execution.OwnerUserID,
		firstCommand.Execution.SessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if latestManaged == nil || latestManaged.ID != firstSnapshot.Execution.ID {
		t.Fatalf("latest managed WorkGraph = %+v, want %s", latestManaged, firstSnapshot.Execution.ID)
	}

	secondSnapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("view-second", secondSnapshot.Execution.Version, "view-second", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	currentManaged, err = repository.FindCurrentManaged(
		ctx,
		firstCommand.Execution.OwnerUserID,
		firstCommand.Execution.SessionKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if currentManaged == nil || currentManaged.ID != secondSnapshot.Execution.ID {
		t.Fatalf("new managed WorkGraph = %+v, want %s", currentManaged, secondSnapshot.Execution.ID)
	}
}

func TestRepositoryRejectRetryAcceptTakeoverAndCompletion(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("flow"))
	if err != nil {
		t.Fatal(err)
	}
	plan := testPlanCommand("flow", snapshot.Execution.Version, "flow", "", 1)
	plan.WorkItems[1].Item.Terminal = true
	plan.Dependencies = []protocol.ExecutionPlanDependency{{
		WorkItemID:          "work-flow-2",
		DependsOnWorkItemID: "work-flow-1",
		Kind:                protocol.WorkDependencyHard,
	}}
	snapshot, err = repository.WritePlan(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err = repository.Assign(ctx, assignTestCommand(snapshot, "work-flow-1", "spec-flow-1", "a1", "agent-a"))
	if err != nil {
		t.Fatal(err)
	}
	duplicate := assignTestCommand(snapshot, "work-flow-1", "spec-flow-1", "duplicate", "agent-z")
	if _, err = repository.Assign(ctx, duplicate); !errors.Is(err, ErrWorkNotReady) {
		t.Fatalf("duplicate Assignment error = %v, want ErrWorkNotReady", err)
	}
	if current, getErr := repository.Get(ctx, "execution-flow"); getErr != nil || current.Version != snapshot.Execution.Version {
		t.Fatalf("duplicate Assignment changed Execution: current=%#v err=%v", current, getErr)
	}

	takeover := TakeoverCommand{
		ExpectedExecutionVersion:         snapshot.Execution.Version,
		ExpectedCurrentAssignmentVersion: 1,
		CurrentAssignmentID:              "assignment-a1",
		Replacement: testAssignment(
			"execution-flow", "plan-flow", "work-flow-1", "spec-flow-1", "assignment-a2", "agent-b",
		),
		RootAttempt: ptrAttempt(testRootAttempt(
			"execution-flow", "plan-flow", "work-flow-1", "spec-flow-1",
			"assignment-a2", "attempt-a2", "agent-b",
		)),
		Meta: testMeta("takeover-a2"),
	}
	takeover.Replacement.TakeoverReason = "agent unavailable"
	snapshot, err = repository.Takeover(ctx, takeover)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(t, ctx, repository, snapshot, "assignment-a2", "attempt-a2")
	snapshot = finishTestAttempt(t, ctx, repository, snapshot, "attempt-a2", protocol.WorkAttemptStatusSucceeded)
	snapshot = submitTestWork(t, ctx, repository, snapshot, "assignment-a2", "attempt-a2", "submission-a2", "agent-b")
	snapshot = reviewTestWork(
		t, ctx, repository, snapshot, "assignment-a2", "submission-a2", "acceptance-reject",
		protocol.WorkAcceptanceRejected,
	)
	if !contains(snapshot.ReadyWorkItemIDs, "work-flow-1") {
		t.Fatalf("rejected Work Item not ready for retry: %#v", snapshot.ReadyWorkItemIDs)
	}

	snapshot, err = repository.Assign(ctx, assignTestCommand(snapshot, "work-flow-1", "spec-flow-1", "a3", "agent-c"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(t, ctx, repository, snapshot, "assignment-a3", "attempt-a3")
	snapshot = finishTestAttempt(t, ctx, repository, snapshot, "attempt-a3", protocol.WorkAttemptStatusSucceeded)
	snapshot = submitTestWork(t, ctx, repository, snapshot, "assignment-a3", "attempt-a3", "submission-a3", "agent-c")
	snapshot = reviewTestWork(
		t, ctx, repository, snapshot, "assignment-a3", "submission-a3", "acceptance-a3",
		protocol.WorkAcceptanceAccepted,
	)
	viewSnapshot, history, err := repository.GetWorkGraphState(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if viewSnapshot == nil || viewSnapshot.Execution.Version != snapshot.Execution.Version ||
		len(history.Assignments) != 3 || len(history.Attempts) != 3 ||
		len(history.Submissions) != 2 || len(history.Acceptances) != 2 {
		t.Fatalf("atomic WorkGraph history lost a retry cycle: snapshot=%+v history=%+v", viewSnapshot, history)
	}
	if history.Submissions[0].ID != "submission-a2" ||
		history.Submissions[1].ID != "submission-a3" ||
		history.Acceptances[0].Decision != protocol.WorkAcceptanceRejected ||
		history.Acceptances[1].Decision != protocol.WorkAcceptanceAccepted {
		t.Fatalf("WorkGraph retry history order/content = %+v / %+v", history.Submissions, history.Acceptances)
	}
	if !contains(snapshot.ReadyWorkItemIDs, "work-flow-2") {
		t.Fatalf("accepted dependency did not unlock downstream: %#v", snapshot.ReadyWorkItemIDs)
	}

	snapshot, err = repository.Assign(ctx, assignTestCommand(snapshot, "work-flow-2", "spec-flow-2", "b1", "agent-lead"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(t, ctx, repository, snapshot, "assignment-b1", "attempt-b1")
	snapshot = finishTestAttempt(t, ctx, repository, snapshot, "attempt-b1", protocol.WorkAttemptStatusSucceeded)
	snapshot = submitTestWork(t, ctx, repository, snapshot, "assignment-b1", "attempt-b1", "submission-b1", "agent-lead")
	snapshot = reviewTestWork(
		t, ctx, repository, snapshot, "assignment-b1", "submission-b1", "acceptance-b1",
		protocol.WorkAcceptanceAccepted,
	)
	if len(snapshot.CompletionBlockers) != 0 {
		t.Fatalf("completion blockers after terminal acceptance = %#v", snapshot.CompletionBlockers)
	}
	snapshot, err = repository.Complete(ctx, CompleteCommand{
		ExecutionID:              "execution-flow",
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Meta:                     testMeta("complete-flow"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusCompleted {
		t.Fatalf("Execution status = %q, want completed", snapshot.Execution.Status)
	}
}

func TestRepositoryKeepsRootAttemptAlongsideConcurrentChildren(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("child-parallel"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("child-parallel", snapshot.Execution.Version, "child-parallel", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.Assign(
		ctx,
		assignTestCommand(snapshot, "work-child-parallel-1", "spec-child-parallel-1", "child-parent", "agent-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	parentAssignment := findAssignment(t, snapshot, "assignment-child-parent")
	parent := findAttempt(t, snapshot, "attempt-child-parent")
	parent.RuntimeSessionKey = "runtime-session-shared"
	parent.RuntimeRoundID = "runtime-round-shared"
	parent.AgentRoundID = "agent-round-shared"
	snapshot, err = repository.StartAttempt(ctx, StartAttemptCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: parentAssignment.Version,
		ExpectedAttemptVersion:    parent.Version,
		Attempt:                   parent,
		Meta:                      testMeta("start-child-parent"),
	})
	if err != nil {
		t.Fatal(err)
	}

	for index, toolUseID := range []string{"tool-child-a", "tool-child-b"} {
		assignment := findAssignment(t, snapshot, "assignment-child-parent")
		child := protocol.WorkAttempt{
			ID:                fmt.Sprintf("attempt-subagent-%d", index+1),
			ExecutionID:       snapshot.Execution.ID,
			PlanID:            snapshot.Plan.ID,
			WorkItemID:        assignment.WorkItemID,
			SpecID:            assignment.SpecID,
			AssignmentID:      assignment.ID,
			ParentAttemptID:   "attempt-child-parent",
			ExecutorKind:      protocol.AttemptExecutorSubagent,
			ParentAgentID:     "agent-a",
			RuntimeSessionKey: parent.RuntimeSessionKey,
			RuntimeRoundID:    parent.RuntimeRoundID,
			AgentRoundID:      parent.AgentRoundID,
			ToolUseID:         toolUseID,
			Status:            protocol.WorkAttemptStatusRunning,
		}
		snapshot, err = repository.StartAttempt(ctx, StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			Attempt:                   child,
			Meta:                      testMeta("start-" + toolUseID),
		})
		if err != nil {
			t.Fatalf("start child %s: %v", toolUseID, err)
		}
	}

	runningChildren := 0
	for _, attempt := range snapshot.Attempts {
		if attempt.ParentAttemptID == "attempt-child-parent" &&
			attempt.Status == protocol.WorkAttemptStatusRunning {
			runningChildren++
		}
	}
	if runningChildren != 2 {
		t.Fatalf("running child Attempts = %d, want 2", runningChildren)
	}

	for _, childID := range []string{"attempt-subagent-1", "attempt-subagent-2"} {
		snapshot = finishTestAttempt(
			t,
			ctx,
			repository,
			snapshot,
			childID,
			protocol.WorkAttemptStatusSucceeded,
		)
	}
	boundedChildren := 0
	for _, attempt := range snapshot.Attempts {
		if attempt.ParentAttemptID == parent.ID {
			boundedChildren++
		}
	}
	if boundedChildren != 1 {
		t.Fatalf("bounded Snapshot child Attempts = %d, want latest 1", boundedChildren)
	}
	workGraphChildren, err := repository.ListWorkGraphChildAttempts(ctx, snapshot.Plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(workGraphChildren) != 2 ||
		workGraphChildren[0].ID != "attempt-subagent-1" ||
		workGraphChildren[1].ID != "attempt-subagent-2" {
		t.Fatalf("WorkGraph child Attempt history = %#v, want both siblings", workGraphChildren)
	}

	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		parent.ID,
		protocol.WorkAttemptStatusSucceeded,
	)
	if finishedParent := findAttempt(t, snapshot, parent.ID); finishedParent.Status != protocol.WorkAttemptStatusSucceeded {
		t.Fatalf("root Attempt status = %q, want succeeded", finishedParent.Status)
	}
	snapshot = submitTestWork(
		t,
		ctx,
		repository,
		snapshot,
		parentAssignment.ID,
		parent.ID,
		"submission-child-parent",
		"agent-a",
	)
	if submission := findSubmission(t, snapshot, "submission-child-parent"); submission.AttemptID != parent.ID {
		t.Fatalf("Submission Attempt = %q, want %q", submission.AttemptID, parent.ID)
	}
}

func TestRepositoryScopesRootAttemptRoundIdentityToAssignment(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("assignment-round"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand(
			"assignment-round",
			snapshot.Execution.Version,
			"assignment-round",
			"",
			1,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 2; index++ {
		suffix := fmt.Sprintf("assignment-round-%d", index)
		snapshot, err = repository.Assign(
			ctx,
			assignTestCommand(
				snapshot,
				fmt.Sprintf("work-assignment-round-%d", index),
				fmt.Sprintf("spec-assignment-round-%d", index),
				suffix,
				"agent-a",
			),
		)
		if err != nil {
			t.Fatalf("assign Work Item %d: %v", index, err)
		}
	}

	const (
		runtimeSessionKey = "agent:agent-a:ws:group:conversation-1"
		runtimeRoundID    = "agent-round-shared"
		agentRoundID      = "agent-round-shared"
	)
	for index := 1; index <= 2; index++ {
		assignmentID := fmt.Sprintf("assignment-assignment-round-%d", index)
		attemptID := fmt.Sprintf("attempt-assignment-round-%d", index)
		assignment := findAssignment(t, snapshot, assignmentID)
		attempt := findAttempt(t, snapshot, attemptID)
		attempt.RuntimeSessionKey = runtimeSessionKey
		attempt.RuntimeRoundID = runtimeRoundID
		attempt.AgentRoundID = agentRoundID
		snapshot, err = repository.StartAttempt(ctx, StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    attempt.Version,
			Attempt:                   attempt,
			Meta:                      testMeta("start-" + attemptID),
		})
		if err != nil {
			t.Fatalf("start different Assignment %d in shared round: %v", index, err)
		}
		snapshot = finishTestAttempt(
			t,
			ctx,
			repository,
			snapshot,
			attemptID,
			protocol.WorkAttemptStatusFailed,
		)
	}

	assignment := findAssignment(t, snapshot, "assignment-assignment-round-2")
	duplicate := protocol.WorkAttempt{
		ID:                "attempt-assignment-round-2-retry",
		ExecutionID:       assignment.ExecutionID,
		PlanID:            assignment.PlanID,
		WorkItemID:        assignment.WorkItemID,
		SpecID:            assignment.SpecID,
		AssignmentID:      assignment.ID,
		ExecutorKind:      protocol.AttemptExecutorAgent,
		ExecutorAgentID:   assignment.OwnerAgentID,
		RuntimeSessionKey: runtimeSessionKey,
		RuntimeRoundID:    runtimeRoundID,
		AgentRoundID:      agentRoundID,
		Status:            protocol.WorkAttemptStatusRunning,
	}
	if _, err = repository.StartAttempt(ctx, StartAttemptCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Attempt:                   duplicate,
		Meta:                      testMeta("start-duplicate-assignment-round"),
	}); err == nil {
		t.Fatal("same Assignment created two root Attempts in one physical round")
	}
	current, getErr := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	duplicateCount := 0
	for _, attempt := range current.Attempts {
		if attempt.ID == duplicate.ID {
			duplicateCount++
		}
	}
	if current.Execution.Version != snapshot.Execution.Version || duplicateCount != 0 {
		t.Fatalf("rejected duplicate changed snapshot: %#v", current)
	}
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

func TestRepositoryAcceptsEveryGoalActivationReason(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	reasons := []protocol.GoalActivationReason{
		protocol.GoalActivationReasonPersistenceRequested,
		protocol.GoalActivationReasonObservedBoundary,
		protocol.GoalActivationReasonRoomDependencyChain,
		protocol.GoalActivationReasonExternalWait,
		protocol.GoalActivationReasonScheduledRetry,
		protocol.GoalActivationReasonContextBoundary,
		protocol.GoalActivationReasonRecoveryRequired,
		protocol.GoalActivationReasonSubstantialComplexity,
	}
	for index, reason := range reasons {
		reason := reason
		t.Run(string(reason), func(t *testing.T) {
			suffix := fmt.Sprintf("activation-reason-%d", index)
			snapshot, err := repository.Create(ctx, createTestCommand(suffix))
			if err != nil {
				t.Fatal(err)
			}
			goalID := "goal-" + suffix
			if _, err = repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status)
VALUES (?, ?, ?, 'active')`, goalID, snapshot.Execution.SessionKey, "persist reason"); err != nil {
				t.Fatal(err)
			}
			bound, err := repository.BindGoal(ctx, BindGoalCommand{
				ExpectedExecutionVersion: snapshot.Execution.Version,
				Execution: protocol.Execution{
					ID:                    snapshot.Execution.ID,
					GoalID:                goalID,
					GoalObjectiveRevision: 1,
					GoalActivationOrigin:  protocol.GoalActivationOriginAdaptivePromoted,
					GoalActivationReason:  reason,
				},
				Meta: testMeta("bind-" + suffix),
			})
			if err != nil {
				t.Fatalf("BindGoal(%q): %v", reason, err)
			}
			if bound.Execution.GoalActivationReason != reason {
				t.Fatalf(
					"persisted activation reason = %q, want %q",
					bound.Execution.GoalActivationReason,
					reason,
				)
			}
		})
	}
}

func TestRepositoryBlockInterruptsCurrentExecutionAndResumeStartsFreshAttempt(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("resume"))
	if err != nil {
		t.Fatal(err)
	}
	plan := testPlanCommand("resume", snapshot.Execution.Version, "resume", "", 1)
	snapshot, err = repository.WritePlan(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(snapshot, "work-resume-1", "spec-resume-1", "resume", "agent-worker")
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-resume",
		DedupeKey:     "dispatch-resume",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "wait for approval",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.ClaimDispatch(
		ctx,
		"dispatch-resume",
		1,
		"worker-1",
		time.Minute,
	); err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(t, ctx, repository, snapshot, "assignment-resume", "attempt-resume")
	state := findState(t, snapshot, "work-resume-1")
	block := BlockCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    state.WorkItemID,
			ExecutionID:   state.ExecutionID,
			CurrentSpecID: state.CurrentSpecID,
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   "external approval missing",
			NeededInput:   "approval-42",
		},
		Meta: testMeta("block-resume"),
	}
	snapshot, err = repository.Block(ctx, block)
	if err != nil {
		t.Fatal(err)
	}
	state = findState(t, snapshot, "work-resume-1")
	if state.Status != protocol.WorkItemStatusWaitingInput ||
		state.BlockReason != "external approval missing" ||
		state.NeededInput != "approval-42" {
		t.Fatalf("blocked state = %#v", state)
	}
	attempt := findAttempt(t, snapshot, "attempt-resume")
	if attempt.Status != protocol.WorkAttemptStatusInterrupted ||
		attempt.FailureReason != "blocked: external approval missing" ||
		attempt.FinishedAt == nil {
		t.Fatalf("blocked Attempt = %#v", attempt)
	}
	dispatch := findDispatch(t, snapshot, "dispatch-resume")
	if dispatch.Status != protocol.ExecutionDispatchStatusCancelled ||
		dispatch.LeaseOwner != "" ||
		dispatch.LeaseExpiresAt != nil {
		t.Fatalf("blocked Dispatch = %#v", dispatch)
	}
	assignment := findAssignment(t, snapshot, "assignment-resume")
	if assignment.Status != protocol.WorkAssignmentStatusReleased ||
		assignment.ReleasedAt == nil {
		t.Fatalf("blocked Assignment = %#v", assignment)
	}
	for _, blocker := range snapshot.CompletionBlockers {
		if blocker == "attempt:attempt-resume:running" ||
			blocker == "dispatch:dispatch-resume:claimed" {
			t.Fatalf("live execution blocker survived Block: %#v", snapshot.CompletionBlockers)
		}
	}
	replayed, err := repository.Block(ctx, block)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != snapshot.Execution.Version {
		t.Fatalf("replayed Block version = %d, want %d", replayed.Execution.Version, snapshot.Execution.Version)
	}

	assignment = findAssignment(t, snapshot, "assignment-resume")
	blockedAttempt := testRootAttempt(
		assignment.ExecutionID,
		assignment.PlanID,
		assignment.WorkItemID,
		assignment.SpecID,
		assignment.ID,
		"attempt-before-resume",
		assignment.OwnerAgentID,
	)
	blockedAttempt.Status = protocol.WorkAttemptStatusRunning
	blockedVersion := snapshot.Execution.Version
	if _, startErr := repository.StartAttempt(ctx, StartAttemptCommand{
		ExpectedExecutionVersion:  blockedVersion,
		ExpectedAssignmentVersion: assignment.Version,
		Attempt:                   blockedAttempt,
		Meta:                      testMeta("attempt-before-resume"),
	}); !errors.Is(startErr, ErrWorkNotReady) {
		t.Fatalf("StartAttempt while waiting_input error = %v, want %v", startErr, ErrWorkNotReady)
	}
	unchanged, err := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Execution.Version != blockedVersion {
		t.Fatalf(
			"blocked StartAttempt changed execution version = %d, want %d",
			unchanged.Execution.Version,
			blockedVersion,
		)
	}

	resume := ResumeCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    state.WorkItemID,
			ExecutionID:   state.ExecutionID,
			CurrentSpecID: state.CurrentSpecID,
			Status:        protocol.WorkItemStatusOpen,
			Metadata:      state.Metadata,
		},
		Resolution: "approval received",
		Evidence:   []string{"approval-42"},
		Meta:       testMeta("resume-work"),
	}
	snapshot, err = repository.Resume(ctx, resume)
	if err != nil {
		t.Fatal(err)
	}
	state = findState(t, snapshot, "work-resume-1")
	if state.Status != protocol.WorkItemStatusOpen ||
		state.BlockReason != "" ||
		state.NeededInput != "" ||
		state.Metadata["last_resume_resolution"] != "approval received" {
		t.Fatalf("resumed state = %#v", state)
	}
	evidence, ok := state.Metadata["last_resume_evidence"].([]any)
	if !ok || len(evidence) != 1 || evidence[0] != "approval-42" {
		t.Fatalf("resume evidence = %#v", state.Metadata["last_resume_evidence"])
	}
	var resumeEventPayload string
	if err = repository.db.QueryRow(
		`SELECT payload_json FROM execution_events WHERE command_id = ?`,
		resume.Meta.CommandID,
	).Scan(&resumeEventPayload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resumeEventPayload, `"resolution":"approval received"`) ||
		!strings.Contains(resumeEventPayload, `"evidence":["approval-42"]`) {
		t.Fatalf("Resume event payload = %s", resumeEventPayload)
	}
	if findAttempt(t, snapshot, "attempt-resume").Status != protocol.WorkAttemptStatusInterrupted ||
		findDispatch(t, snapshot, "dispatch-resume").Status != protocol.ExecutionDispatchStatusCancelled {
		t.Fatal("Resume revived a terminated Attempt or Dispatch")
	}
	if !contains(snapshot.ReadyWorkItemIDs, "work-resume-1") {
		t.Fatalf("resumed Work Item is not ready for reassignment: %#v", snapshot.ReadyWorkItemIDs)
	}
	replayed, err = repository.Resume(ctx, resume)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.Version != snapshot.Execution.Version {
		t.Fatalf("replayed Resume version = %d, want %d", replayed.Execution.Version, snapshot.Execution.Version)
	}

	reassign := assignTestCommand(
		snapshot,
		"work-resume-1",
		"spec-resume-1",
		"after-resume",
		"agent-worker",
	)
	snapshot, err = repository.Assign(ctx, reassign)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		"assignment-after-resume",
		"attempt-after-resume",
	)
	var oldAttemptStatus string
	if err = repository.db.QueryRow(
		`SELECT status FROM execution_attempts WHERE attempt_id = ?`,
		"attempt-resume",
	).Scan(&oldAttemptStatus); err != nil {
		t.Fatal(err)
	}
	if findAttempt(t, snapshot, "attempt-after-resume").Status != protocol.WorkAttemptStatusRunning ||
		oldAttemptStatus != string(protocol.WorkAttemptStatusInterrupted) {
		t.Fatalf("fresh Attempt chain = %#v old=%s", snapshot.Attempts, oldAttemptStatus)
	}
}

func TestRepositoryBlockPreservesDeliveredDispatchReceipt(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("delivered-block"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("delivered-block", snapshot.Execution.Version, "delivered-block", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(
		snapshot,
		"work-delivered-block-1",
		"spec-delivered-block-1",
		"delivered-block",
		"agent-worker",
	)
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-delivered-block",
		DedupeKey:     "dispatch-delivered-block",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := repository.ClaimDispatch(
		ctx,
		"dispatch-delivered-block",
		1,
		"worker-delivered",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repository.MarkDispatchDelivered(
		ctx,
		claimed.ID,
		claimed.Version,
		"worker-delivered",
		"handoff-delivered",
		"queue-delivered",
	); err != nil {
		t.Fatal(err)
	}
	state := findState(t, snapshot, "work-delivered-block-1")
	snapshot, err = repository.Block(ctx, BlockCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    state.WorkItemID,
			ExecutionID:   state.ExecutionID,
			CurrentSpecID: state.CurrentSpecID,
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   "waiting",
			NeededInput:   "answer",
		},
		Meta: testMeta("block-delivered"),
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatch := findDispatch(t, snapshot, "dispatch-delivered-block")
	if dispatch.Status != protocol.ExecutionDispatchStatusDelivered ||
		dispatch.HandoffID != "handoff-delivered" ||
		dispatch.QueueItemID != "queue-delivered" {
		t.Fatalf("delivered receipt changed by Block = %#v", dispatch)
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

func TestRepositoryPlanRevisionNewSpecStartsCommandLifecycle(t *testing.T) {
	for _, previousStatus := range []protocol.WorkItemStatus{
		protocol.WorkItemStatusWaitingInput,
		protocol.WorkItemStatusCancelled,
		protocol.WorkItemStatusSuperseded,
	} {
		t.Run(string(previousStatus), func(t *testing.T) {
			repository := newRepositoryTestStore(t)
			ctx := context.Background()
			suffix := "revision-lifecycle-" + string(previousStatus)
			snapshot, err := repository.Create(ctx, createTestCommand(suffix))
			if err != nil {
				t.Fatal(err)
			}
			oldPlan := testPlanCommand(suffix, snapshot.Execution.Version, suffix+"-old", "", 1)
			snapshot, err = repository.WritePlan(ctx, oldPlan)
			if err != nil {
				t.Fatal(err)
			}
			oldWork := oldPlan.WorkItems[0]
			if _, err = repository.db.Exec(
				`UPDATE execution_work_item_states
SET status = ?,
    block_reason = ?,
    needed_input = ?,
    metadata_json = ?,
    version = version + 1
WHERE work_item_id = ?`,
				previousStatus,
				"stale reason",
				"stale input",
				`{"last_resume_resolution":"old answer","last_resume_evidence":["old evidence"]}`,
				oldWork.WorkItem.ID,
			); err != nil {
				t.Fatal(err)
			}
			snapshot, err = repository.GetSnapshot(ctx, snapshot.Execution.ID)
			if err != nil {
				t.Fatal(err)
			}
			previousState := findState(t, snapshot, oldWork.WorkItem.ID)

			replacement := testPlanCommand(
				suffix,
				snapshot.Execution.Version,
				suffix+"-new",
				oldPlan.Plan.ID,
				2,
			)
			replacement.Plan.RevisionReason = "spec changed"
			reuseStableWorkItemForRevision(&replacement.WorkItems[0], oldWork.WorkItem)
			replacement.WorkItems[0].Spec.Version = 2
			replacement.WorkItems[0].ExpectedStateVersion = previousState.Version
			replacement.Meta = testMeta("plan-" + suffix + "-new-spec")

			updated, err := repository.WritePlan(ctx, replacement)
			if err != nil {
				t.Fatal(err)
			}
			state := findState(t, updated, oldWork.WorkItem.ID)
			if state.CurrentSpecID != replacement.WorkItems[0].Spec.ID ||
				state.Status != protocol.WorkItemStatusOpen ||
				state.BlockReason != "" ||
				state.NeededInput != "" ||
				len(state.Metadata) != 0 ||
				state.Version != previousState.Version+1 {
				t.Fatalf(
					"new Spec state = %#v, want spec=%q status=open cleared blockers/metadata version=%d",
					state,
					replacement.WorkItems[0].Spec.ID,
					previousState.Version+1,
				)
			}
		})
	}
}

func TestRepositoryPlanRevisionSameSpecPreservesLifecycle(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("revision-same-spec"))
	if err != nil {
		t.Fatal(err)
	}
	oldPlan := testPlanCommand(
		"revision-same-spec",
		snapshot.Execution.Version,
		"revision-same-spec-old",
		"",
		1,
	)
	snapshot, err = repository.WritePlan(ctx, oldPlan)
	if err != nil {
		t.Fatal(err)
	}
	workID := oldPlan.WorkItems[0].WorkItem.ID
	if _, err = repository.db.Exec(
		`UPDATE execution_work_item_states
SET status = 'waiting_input',
    block_reason = 'preserve reason',
    needed_input = 'preserve input',
    metadata_json = '{"last_resume_resolution":"preserve answer","last_resume_evidence":["preserve evidence"]}',
    version = version + 1
WHERE work_item_id = ?`,
		workID,
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	previousState := findState(t, snapshot, workID)

	replacement := oldPlan
	replacement.ExpectedExecutionVersion = snapshot.Execution.Version
	replacement.Plan.ID = "plan-revision-same-spec-new"
	replacement.Plan.Revision = 2
	replacement.Plan.BasePlanID = oldPlan.Plan.ID
	replacement.Plan.RevisionReason = "layout and dependency changed"
	replacement.WorkItems[0].Item.Position = 1
	replacement.WorkItems[1].Item.Position = 0
	replacement.Dependencies = []protocol.ExecutionPlanDependency{{
		WorkItemID:          replacement.WorkItems[1].WorkItem.ID,
		DependsOnWorkItemID: replacement.WorkItems[0].WorkItem.ID,
		Kind:                protocol.WorkDependencyHard,
	}}
	replacement.Meta = testMeta("plan-revision-same-spec-new")

	updated, err := repository.WritePlan(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	state := findState(t, updated, workID)
	if state.CurrentSpecID != previousState.CurrentSpecID ||
		state.Status != protocol.WorkItemStatusWaitingInput ||
		state.BlockReason != "preserve reason" ||
		state.NeededInput != "preserve input" ||
		state.Metadata["last_resume_resolution"] != "preserve answer" ||
		state.Version != previousState.Version {
		t.Fatalf("same Spec lifecycle changed: before=%#v after=%#v", previousState, state)
	}
}

func TestRepositoryPlanRevisionNeverDropsUnreviewedSubmission(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("revision-review"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("revision-review", snapshot.Execution.Version, "revision-review-old", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.Assign(
		ctx,
		assignTestCommand(
			snapshot,
			"work-revision-review-old-1",
			"spec-revision-review-old-1",
			"revision-review-old",
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
		"assignment-revision-review-old",
		"attempt-revision-review-old",
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		"attempt-revision-review-old",
		protocol.WorkAttemptStatusSucceeded,
	)
	snapshot = submitTestWork(
		t,
		ctx,
		repository,
		snapshot,
		"assignment-revision-review-old",
		"attempt-revision-review-old",
		"submission-revision-review",
		"agent-worker",
	)
	versionBefore := snapshot.Execution.Version
	replacement := testPlanCommand(
		"revision-review",
		versionBefore,
		"revision-review-new",
		"plan-revision-review-old",
		2,
	)
	replacement.Plan.RevisionReason = "replace submitted work"
	replacement.SupersedeActiveWork = true
	if _, err = repository.WritePlan(ctx, replacement); !errors.Is(err, ErrInvariant) {
		t.Fatalf("replacement with unreviewed Submission error = %v, want ErrInvariant", err)
	}
	current, err := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Execution.Version != versionBefore ||
		current.Plan == nil ||
		current.Plan.ID != "plan-revision-review-old" ||
		len(current.Submissions) != 1 ||
		len(current.Acceptances) != 0 {
		t.Fatalf("unreviewed Submission was not preserved = %#v", current)
	}
}

func TestRepositoryUnreviewedSubmissionRejectsBlockAndTakeoverButAllowsReview(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("review-lock"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand("review-lock", snapshot.Execution.Version, "review-lock", "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.Assign(
		ctx,
		assignTestCommand(
			snapshot,
			"work-review-lock-1",
			"spec-review-lock-1",
			"review-lock",
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
		"assignment-review-lock",
		"attempt-review-lock",
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		"attempt-review-lock",
		protocol.WorkAttemptStatusSucceeded,
	)
	snapshot = submitTestWork(
		t,
		ctx,
		repository,
		snapshot,
		"assignment-review-lock",
		"attempt-review-lock",
		"submission-review-lock",
		"agent-worker",
	)
	versionBefore := snapshot.Execution.Version
	state := findState(t, snapshot, "work-review-lock-1")
	if _, err = repository.Block(ctx, BlockCommand{
		ExpectedExecutionVersion: versionBefore,
		ExpectedStateVersion:     state.Version,
		State: protocol.WorkItemState{
			WorkItemID:    state.WorkItemID,
			ExecutionID:   state.ExecutionID,
			CurrentSpecID: state.CurrentSpecID,
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   "new external wait",
			NeededInput:   "approval-2",
		},
		Meta: testMeta("block-review-lock"),
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("Block with pending Submission error = %v, want %v", err, ErrInvariant)
	}

	current := findAssignment(t, snapshot, "assignment-review-lock")
	replacement := assignTestCommand(
		snapshot,
		current.WorkItemID,
		current.SpecID,
		"review-lock-replacement",
		"agent-replacement",
	)
	replacement.Assignment.TakeoverReason = "replace owner before review"
	if _, err = repository.Takeover(ctx, TakeoverCommand{
		ExpectedExecutionVersion:         versionBefore,
		ExpectedCurrentAssignmentVersion: current.Version,
		CurrentAssignmentID:              current.ID,
		Replacement:                      replacement.Assignment,
		RootAttempt:                      replacement.RootAttempt,
		Meta:                             testMeta("takeover-review-lock"),
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("Takeover with pending Submission error = %v, want %v", err, ErrInvariant)
	}

	unchanged, err := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Execution.Version != versionBefore ||
		findState(t, unchanged, state.WorkItemID).Status != protocol.WorkItemStatusOpen ||
		findAssignment(t, unchanged, current.ID).Status != protocol.WorkAssignmentStatusActive ||
		len(unchanged.Submissions) != 1 ||
		len(unchanged.Acceptances) != 0 {
		t.Fatalf("pending Submission chain changed after rejected mutations = %#v", unchanged)
	}

	parallel, err := repository.Assign(
		ctx,
		assignTestCommand(
			unchanged,
			"work-review-lock-2",
			"spec-review-lock-2",
			"review-lock-safe",
			"agent-parallel",
		),
	)
	if err != nil {
		t.Fatalf("Assign unrelated safe Work Item: %v", err)
	}
	safeCurrent := findAssignment(t, parallel, "assignment-review-lock-safe")
	safeReplacement := assignTestCommand(
		parallel,
		safeCurrent.WorkItemID,
		safeCurrent.SpecID,
		"review-lock-safe-replacement",
		"agent-parallel-replacement",
	)
	safeReplacement.Assignment.TakeoverReason = "parallel owner unavailable"
	parallel, err = repository.Takeover(ctx, TakeoverCommand{
		ExpectedExecutionVersion:         parallel.Execution.Version,
		ExpectedCurrentAssignmentVersion: safeCurrent.Version,
		CurrentAssignmentID:              safeCurrent.ID,
		Replacement:                      safeReplacement.Assignment,
		RootAttempt:                      safeReplacement.RootAttempt,
		Meta:                             testMeta("takeover-review-lock-safe"),
	})
	if err != nil {
		t.Fatalf("Takeover unrelated safe Work Item: %v", err)
	}
	safeState := findState(t, parallel, "work-review-lock-2")
	parallel, err = repository.Block(ctx, BlockCommand{
		ExpectedExecutionVersion: parallel.Execution.Version,
		ExpectedStateVersion:     safeState.Version,
		State: protocol.WorkItemState{
			WorkItemID:    safeState.WorkItemID,
			ExecutionID:   safeState.ExecutionID,
			CurrentSpecID: safeState.CurrentSpecID,
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   "parallel approval missing",
			NeededInput:   "parallel-approval",
		},
		Meta: testMeta("block-review-lock-safe"),
	})
	if err != nil {
		t.Fatalf("Block unrelated safe Work Item: %v", err)
	}
	if findState(t, parallel, safeState.WorkItemID).Status != protocol.WorkItemStatusWaitingInput ||
		findAssignment(t, parallel, current.ID).Status != protocol.WorkAssignmentStatusActive ||
		len(parallel.Submissions) != 1 ||
		len(parallel.Acceptances) != 0 {
		t.Fatalf("safe parallel mutations changed pending review chain = %#v", parallel)
	}

	reviewed := reviewTestWork(
		t,
		ctx,
		repository,
		parallel,
		current.ID,
		"submission-review-lock",
		"acceptance-review-lock",
		protocol.WorkAcceptanceAccepted,
	)
	if len(reviewed.Acceptances) != 1 ||
		reviewed.Acceptances[0].SubmissionID != "submission-review-lock" ||
		findAssignment(t, reviewed, current.ID).Status != protocol.WorkAssignmentStatusCompleted {
		t.Fatalf("Submission was not reviewable after rejected mutations = %#v", reviewed)
	}
}

func TestDeriveSnapshotAcceptedSpecIgnoresStaleWaitingInput(t *testing.T) {
	snapshot := &protocol.ExecutionSnapshot{
		Execution: protocol.Execution{
			ID:     "execution-accepted-waiting",
			Status: protocol.ExecutionStatusActive,
		},
		Plan: &protocol.ExecutionPlanRevision{
			ID:          "plan-accepted-waiting",
			ExecutionID: "execution-accepted-waiting",
			Status:      protocol.PlanRevisionStatusActive,
		},
		PlanItems: []protocol.ExecutionPlanItem{{
			PlanID:      "plan-accepted-waiting",
			ExecutionID: "execution-accepted-waiting",
			WorkItemID:  "work-accepted-waiting",
			SpecID:      "spec-accepted-waiting",
			Required:    true,
			Terminal:    true,
		}},
		WorkItemStates: []protocol.WorkItemState{{
			WorkItemID:    "work-accepted-waiting",
			ExecutionID:   "execution-accepted-waiting",
			CurrentSpecID: "spec-accepted-waiting",
			Status:        protocol.WorkItemStatusWaitingInput,
			BlockReason:   "stale",
			NeededInput:   "already received",
		}},
		Acceptances: []protocol.WorkAcceptance{{
			SubmissionID: "submission-accepted-waiting",
			WorkItemID:   "work-accepted-waiting",
			SpecID:       "spec-accepted-waiting",
			Decision:     protocol.WorkAcceptanceAccepted,
		}},
	}
	deriveSnapshot(snapshot)
	if len(snapshot.CompletionBlockers) != 0 {
		t.Fatalf("accepted current spec retained stale blockers = %#v", snapshot.CompletionBlockers)
	}
	if len(snapshot.ReadyWorkItemIDs) != 0 {
		t.Fatalf("accepted current spec became ready again = %#v", snapshot.ReadyWorkItemIDs)
	}
}

func newRepositoryTestStore(t *testing.T) *Repository {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "repository.db")
	migrationDB, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
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

func orchestrationMigrationDir(t *testing.T, dialect string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate orchestration repository test")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", dialect)
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

func reuseStableWorkItemForRevision(work *PlanWorkItem, stable protocol.WorkItem) {
	work.WorkItem = stable
	work.Spec.WorkItemID = stable.ID
	work.State.WorkItemID = stable.ID
	work.State.CurrentSpecID = work.Spec.ID
	work.Item.WorkItemID = stable.ID
	work.Item.SpecID = work.Spec.ID
	for index := range work.OutputClaims {
		work.OutputClaims[index].WorkItemID = stable.ID
		work.OutputClaims[index].SpecID = work.Spec.ID
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

func TestIsTransientMutationError(t *testing.T) {
	for _, err := range []error{
		ErrVersionConflict,
		fmt.Errorf("wrapped: %w", ErrVersionConflict),
		errors.New("database is locked (SQLITE_BUSY)"),
		errors.New("database table is locked"),
		errors.New("SQLITE_LOCKED: shared cache conflict"),
	} {
		if !IsTransientMutationError(err) {
			t.Fatalf("error %q was not classified as transient", err)
		}
	}
	if IsTransientMutationError(ErrInvariant) ||
		IsTransientMutationError(errors.New("constraint failed")) {
		t.Fatal("deterministic invariant error was classified as transient")
	}
}
