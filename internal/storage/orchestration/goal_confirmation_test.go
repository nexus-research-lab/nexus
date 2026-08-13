package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
)

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

func TestGoalConfirmationReceiptIsAtomicWithGoalBoundCreate(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	command := createTestCommand("goal-confirmation-create")
	command.Execution.GoalID = "goal-confirmation-create"
	command.Execution.GoalObjectiveRevision = 1
	command.Execution.GoalActivationOrigin = protocol.GoalActivationOriginUserExplicit
	command.Execution.GoalActivationReason = protocol.GoalActivationReasonPersistenceRequested
	if _, err := repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status)
VALUES (?, ?, ?, 'active')`,
		command.Execution.GoalID,
		command.Execution.SessionKey,
		command.Execution.Objective,
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.Create(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := repository.GetGoalConfirmationReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.State != GoalConfirmationPending ||
		receipt.GoalID != command.Execution.GoalID {
		t.Fatalf("Goal-bound Create receipt = %#v", receipt)
	}
}

func TestGoalConfirmationReceiptIsAtomicWithGoalBoundCreateWithPlan(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	create := createTestCommand("goal-confirmation-plan")
	create.Execution.GoalID = "goal-confirmation-plan"
	create.Execution.GoalObjectiveRevision = 1
	create.Execution.GoalActivationOrigin = protocol.GoalActivationOriginUserExplicit
	create.Execution.GoalActivationReason = protocol.GoalActivationReasonPersistenceRequested
	if _, err := repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status)
VALUES (?, ?, ?, 'active')`,
		create.Execution.GoalID,
		create.Execution.SessionKey,
		create.Execution.Objective,
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err := repository.CreateWithPlan(ctx, CreateWithPlanCommand{
		Execution: create.Execution,
		Plan:      testPlanCommand("goal-confirmation-plan", 1, "goal-confirmation-plan", "", 1),
		Meta:      create.Meta,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Plan == nil {
		t.Fatalf("Goal-bound CreateWithPlan returned no Plan: %#v", snapshot)
	}
	receipt, err := repository.GetGoalConfirmationReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.State != GoalConfirmationPending ||
		receipt.GoalID != create.Execution.GoalID ||
		receipt.GoalObjectiveRevision != create.Execution.GoalObjectiveRevision {
		t.Fatalf("Goal-bound CreateWithPlan receipt = %#v", receipt)
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
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
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

func TestGoalConfirmationMigrationDefinesBothDialects(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		payload, err := os.ReadFile(filepath.Join(
			orchestrationMigrationDir(t, dialect),
			"00098_execution_goal_confirmations.sql",
		))
		if err != nil {
			t.Fatal(err)
		}
		text := string(payload)
		for _, required := range []string{
			"CREATE TABLE execution_goal_confirmations",
			"completion_criteria_json",
			"state IN ('pending', 'confirmed')",
			"idx_execution_goal_confirmations_recoverable",
			"execution_binding_state",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s migration missing %q", dialect, required)
			}
		}
	}
}
