package orchestration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestRepositoryGoalRevisionRetargetPreservesCompletedPredecessor(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	oldCreate := createTestCommand("goal-revision-completed")
	if _, err := repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (?, ?, ?, 'active', ?)`,
		"goal-revision-completed",
		oldCreate.Execution.SessionKey,
		oldCreate.Execution.Objective,
		`{"objective_revision":1,"activation_origin":"adaptive_promoted","activation_reason":"observed_boundary"}`,
	); err != nil {
		t.Fatal(err)
	}
	oldCreate.Execution.GoalID = "goal-revision-completed"
	oldCreate.Execution.GoalObjectiveRevision = 1
	oldCreate.Execution.GoalActivationOrigin = protocol.GoalActivationOriginAdaptivePromoted
	oldCreate.Execution.GoalActivationReason = protocol.GoalActivationReasonObservedBoundary
	old, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: oldCreate.Execution,
		Plan:      testPlanCommand("goal-revision-completed", 1, "goal-revision-completed", "", 1),
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
	if err != nil || completed.Execution.Status != protocol.ExecutionStatusCompleted {
		t.Fatalf("completed predecessor = %+v err=%v", completed, err)
	}
	command := SupersedeGoalRevisionCommand{
		ExecutionID:              completed.Execution.ID,
		ExpectedExecutionVersion: completed.Execution.Version,
		GoalID:                   completed.Execution.GoalID,
		OldGoalObjectiveRevision: 1,
		NewGoalObjectiveRevision: 2,
		SuccessorExecutionID:     "execution-goal-revision-completed-new",
		Reason:                   "user retargeted after terminal acceptance",
		Meta:                     testMeta("goal-revision-completed-supersede"),
	}
	preserved, err := repository.SupersedeGoalRevision(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if preserved.Execution.Status != protocol.ExecutionStatusCompleted ||
		preserved.Execution.Version != completed.Execution.Version+1 {
		t.Fatalf("terminal predecessor changed = %+v, want completed version %d", preserved.Execution, completed.Execution.Version+1)
	}
	replayed, err := repository.SupersedeGoalRevision(ctx, command)
	if err != nil || replayed.Execution.Status != protocol.ExecutionStatusCompleted {
		t.Fatalf("terminal supersede replay = %+v err=%v", replayed, err)
	}
	conflict := command
	conflict.SuccessorExecutionID = "execution-goal-revision-completed-other"
	conflict.Meta = testMeta("goal-revision-completed-conflict")
	if _, err = repository.SupersedeGoalRevision(ctx, conflict); !errors.Is(err, ErrInvariant) {
		t.Fatalf("second terminal successor reservation error = %v, want ErrInvariant", err)
	}

	successorCreate := createTestCommand("goal-revision-completed-new")
	successorCreate.Execution.ID = command.SuccessorExecutionID
	successorCreate.Execution.OwnerUserID = completed.Execution.OwnerUserID
	successorCreate.Execution.SessionKey = completed.Execution.SessionKey
	successorCreate.Execution.ScopeKind = completed.Execution.ScopeKind
	successorCreate.Execution.RoomID = completed.Execution.RoomID
	successorCreate.Execution.ConversationID = completed.Execution.ConversationID
	successorCreate.Execution.GoalID = completed.Execution.GoalID
	successorCreate.Execution.GoalObjectiveRevision = 2
	successorCreate.Execution.GoalActivationOrigin = completed.Execution.GoalActivationOrigin
	successorCreate.Execution.GoalActivationReason = completed.Execution.GoalActivationReason
	successorCreate.Execution.ReplacesExecutionID = completed.Execution.ID
	successorCreate.Execution.Objective = "retargeted after terminal acceptance"
	successor, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: successorCreate.Execution,
		Plan:      testPlanCommand("goal-revision-completed-new", 1, "goal-revision-completed-new", "", 1),
		Meta:      successorCreate.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	if successor.Execution.GoalObjectiveRevision != 2 ||
		successor.Execution.ReplacesExecutionID != completed.Execution.ID {
		t.Fatalf("terminal predecessor successor = %+v", successor.Execution)
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
