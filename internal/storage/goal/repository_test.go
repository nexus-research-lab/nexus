// repository_test.go 覆盖 Goal 仓储基础操作与续跑计划测试。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	_ "modernc.org/sqlite"
)

func TestRepositoryGoalLifecycle(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	budget := int64(100)
	item := protocol.Goal{
		ID:          "goal-1",
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "ship",
		Status:      protocol.GoalStatusActive,
		TokenBudget: &budget,
		Usage: protocol.GoalUsage{
			InputTokens:          10,
			OutputTokens:         2,
			CacheReadInputTokens: 50,
			ActualTotalTokens:    80,
		},
		TimeUsedSeconds: 12,
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
		Metadata:        map[string]any{"source": "test"},
	}

	created, err := repository.CreateGoal(ctx, item)
	if err != nil {
		t.Fatal(err)
	}
	if created.TokenBudget == nil || *created.TokenBudget != budget || created.TimeUsedSeconds != 12 ||
		created.Metadata["source"] != "test" || created.Usage.BudgetTokens() != 12 ||
		created.Usage.ActualTokens() != 80 || created.Usage.ActualTokensAreEstimated() {
		t.Fatalf("created = %#v, want persisted actual/budget usage and metadata", created)
	}
	current, err := repository.GetCurrentGoal(ctx, item.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != item.ID {
		t.Fatalf("current = %#v, want goal-1", current)
	}

	created.Status = protocol.GoalStatusPaused
	created.Version++
	created.UpdatedAt = now.Add(time.Minute)
	updated, err := repository.UpdateGoal(ctx, *created, 1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != protocol.GoalStatusPaused || updated.Version != 2 {
		t.Fatalf("updated = %#v, want paused v2", updated)
	}
	updated.Status = protocol.GoalStatusBudgetLimited
	updated.Version++
	updated.UpdatedAt = now.Add(2 * time.Minute)
	budgetLimited, err := repository.UpdateGoal(ctx, *updated, 2)
	if err != nil {
		t.Fatal(err)
	}
	current, err = repository.GetCurrentGoal(ctx, item.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != item.ID || budgetLimited.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("current = %#v updated = %#v, want budget_limited current goal", current, budgetLimited)
	}
	currentGoals, err := repository.ListCurrentGoals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(currentGoals) != 1 || currentGoals[0].ID != item.ID {
		t.Fatalf("current goals = %#v, want budget-limited goal-1", currentGoals)
	}
	budgetLimited.Status = protocol.GoalStatusComplete
	budgetLimited.Version++
	budgetLimited.UpdatedAt = now.Add(3 * time.Minute)
	completed, err := repository.UpdateGoal(ctx, *budgetLimited, 3)
	if err != nil {
		t.Fatal(err)
	}
	current, err = repository.GetCurrentGoal(ctx, item.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil || completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("current = %#v updated = %#v, want completed goal no longer current", current, completed)
	}
	currentGoals, err = repository.ListCurrentGoals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(currentGoals) != 0 {
		t.Fatalf("current goals = %#v, want completed goal excluded", currentGoals)
	}
	if _, err := repository.UpdateGoal(ctx, *updated, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale update error = %v, want sql.ErrNoRows", err)
	}

	runnable, err := repository.ListRunnableGoals(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runnable) != 0 {
		t.Fatalf("runnable = %#v, want no non-active goals", runnable)
	}

	_, err = repository.CreateGoal(ctx, protocol.Goal{
		ID:         "goal-2",
		SessionKey: "agent:nexus:ws:dm:chat-2",
		Objective:  "resume",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.AppendEvent(ctx, protocol.GoalEvent{
		ID:         "event-goal-2",
		GoalID:     "goal-2",
		SessionKey: "agent:nexus:ws:dm:chat-2",
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceSystem,
		CreatedAt:  now,
	}); err != nil {
		t.Fatal(err)
	}
	runnable, err = repository.ListRunnableGoals(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runnable) != 1 || runnable[0].ID != "goal-2" {
		t.Fatalf("runnable = %#v, want active goal-2", runnable)
	}

	deleted, err := repository.DeleteGoal(ctx, "goal-2")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("DeleteGoal(goal-2) = false, want true")
	}
	current, err = repository.GetGoal(ctx, "goal-2")
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("goal-2 = %#v, want nil after delete", current)
	}
	events, err := repository.ListEvents(ctx, "goal-2", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("goal-2 events = %#v, want none after delete", events)
	}
	deleted, err = repository.DeleteGoal(ctx, "goal-2")
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("second DeleteGoal(goal-2) = true, want false")
	}
}

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigration(t, db)
	return NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
}

func applyGoalMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	applyGoalMigrationFiles(t, db,
		"../../../db/migrations/sqlite/00025_session_goals.sql",
		"../../../db/migrations/sqlite/00026_goal_codex_statuses.sql",
		"../../../db/migrations/sqlite/00027_goal_budget_token_total.sql",
		"../../../db/migrations/sqlite/00028_goal_remove_cleared_status.sql",
		"../../../db/migrations/sqlite/00037_session_goals_compat.sql",
		"../../../db/migrations/sqlite/00051_goal_token_totals.sql",
		"../../../db/migrations/sqlite/00052_goal_usage_source_checkpoints.sql",
		"../../../db/migrations/sqlite/00053_goal_usage_source_round_pending.sql",
		"../../../db/migrations/sqlite/00054_goal_usage_finalization.sql",
		"../../../db/migrations/sqlite/00055_goal_usage_source_baseline.sql",
		"../../../db/migrations/sqlite/00104_goal_continuation_plans.sql",
	)
}

func applyGoalMigrationFiles(t *testing.T, db *sql.DB, paths ...string) {
	t.Helper()
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		upSQL := strings.Split(string(body), "-- +goose Down")[0]
		upSQL = strings.ReplaceAll(upSQL, "-- +goose Up", "")
		for _, statement := range strings.Split(upSQL, ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("exec migration %s statement %q: %v", path, statement, err)
			}
		}
	}
}

func TestRepositoryGoalContinuationPlanSurvivesReopenAndFencesDuplicateWorkers(t *testing.T) {
	ctx := context.Background()
	databaseURL := filepath.Join(t.TempDir(), "goal-continuation.sqlite")
	db := openGoalContinuationTestDB(t, databaseURL)
	applyGoalMigration(t, db)
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	now := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	goal := protocol.Goal{
		ID: "goal-durable", SessionKey: "agent:nexus:ws:dm:durable",
		Objective: "finish durable recovery", Status: protocol.GoalStatusActive,
		ContinuationCount: 1, Version: 2, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repository.CreateGoal(ctx, protocol.Goal{
		ID: goal.ID, SessionKey: goal.SessionKey, Objective: goal.Objective,
		Status: goal.Status, Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	event := protocol.GoalEvent{
		ID: "event-scheduled", GoalID: goal.ID, SessionKey: goal.SessionKey,
		EventType: "continuation_scheduled", Source: protocol.GoalUpdateSourceSystem,
		RoundID: "round-durable", CreatedAt: now,
	}
	next := now
	plan := protocol.GoalContinuationPlan{
		RoundID: "round-durable", GoalID: goal.ID, SessionKey: goal.SessionKey,
		ObjectiveRevision: 1, PreviousRoundID: "round-before",
		Prompt: "secret server prompt", Purpose: "goal_continuation",
		Metadata: map[string]string{"goal_id": goal.ID},
		Status:   protocol.GoalContinuationPlanStatusScheduled, Version: 1,
		NextAttemptAt: &next, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repository.ReserveGoalContinuation(ctx, goal, 1, event, plan); err != nil {
		t.Fatal(err)
	}
	deadline, err := repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline == nil || !deadline.Equal(next) {
		t.Fatalf("scheduled continuation deadline = %v, err=%v", deadline, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db = openGoalContinuationTestDB(t, databaseURL)
	repository = NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	recovered, err := repository.GetOpenGoalContinuation(ctx, goal.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if recovered == nil || recovered.Prompt != plan.Prompt || recovered.PreviousRoundID != plan.PreviousRoundID {
		t.Fatalf("recovered = %#v, want complete server-only plan", recovered)
	}
	leaseEnd := now.Add(time.Minute)
	claimed, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, now, leaseEnd)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != protocol.GoalContinuationPlanStatusClaimed || claimed.AttemptCount != 1 {
		t.Fatalf("claimed = %#v", claimed)
	}
	deadline, err = repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline == nil || !deadline.Equal(leaseEnd) {
		t.Fatalf("claimed continuation deadline = %v, err=%v", deadline, err)
	}
	if _, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, now, leaseEnd); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("duplicate claim error = %v, want CAS miss", err)
	}
	if _, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, leaseEnd.Add(time.Second), leaseEnd.Add(time.Minute)); err != nil {
		t.Fatalf("expired lease recovery: %v", err)
	}
	startedAt := leaseEnd.Add(2 * time.Second)
	startedRecoveryAt := startedAt.Add(15 * time.Minute)
	if err := repository.MarkGoalContinuationStarted(ctx, plan.RoundID, startedAt, startedRecoveryAt); err != nil {
		t.Fatal(err)
	}
	deadline, err = repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline == nil || !deadline.Equal(startedRecoveryAt) {
		t.Fatalf("started continuation deadline = %v, err=%v", deadline, err)
	}
	if err := repository.MarkGoalContinuationStarted(ctx, plan.RoundID, leaseEnd.Add(3*time.Second), startedRecoveryAt); err != nil {
		t.Fatalf("idempotent started settlement: %v", err)
	}
	if open, err := repository.GetOpenGoalContinuation(ctx, goal.ID, 1); err != nil || open == nil || open.Status != protocol.GoalContinuationPlanStatusStarted {
		t.Fatalf("open after started = %#v, %v; want started receipt owned by continuation recovery", open, err)
	}
	if _, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, startedAt.Add(time.Minute), startedAt.Add(2*time.Minute)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("live started receipt claim error = %v, want CAS miss", err)
	}
	if recovered, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, startedRecoveryAt.Add(time.Second), startedRecoveryAt.Add(time.Minute)); err != nil || recovered.Status != protocol.GoalContinuationPlanStatusClaimed || recovered.AttemptCount != 3 {
		t.Fatalf("expired started recovery = %#v, %v", recovered, err)
	}
	if err := repository.SettleGoalContinuation(ctx, goal.ID, plan.RoundID, 1, startedRecoveryAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	deadline, err = repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline != nil {
		t.Fatalf("settled continuation deadline = %v, err=%v", deadline, err)
	}
	if err := repository.SettleGoalContinuation(ctx, goal.ID, plan.RoundID, 1, startedRecoveryAt.Add(3*time.Second)); err != nil {
		t.Fatalf("idempotent terminal settlement: %v", err)
	}
	if err := repository.SettleGoalContinuation(ctx, "goal-wrong", plan.RoundID, 1, startedRecoveryAt.Add(4*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("mismatched terminal settlement error = %v, want fail closed", err)
	}
	if open, err := repository.GetOpenGoalContinuation(ctx, goal.ID, 1); err != nil || open != nil {
		t.Fatalf("open after terminal settlement = %#v, %v", open, err)
	}
}

func TestRepositoryGoalContinuationRetryPreservesCountAndCancelsOnRevisionChange(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID: "goal-retry", SessionKey: "agent:nexus:ws:dm:retry", Objective: "retry",
		Status: protocol.GoalStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	created.ContinuationCount = 1
	created.Version++
	next := now
	plan := protocol.GoalContinuationPlan{
		RoundID: "round-retry", GoalID: created.ID, SessionKey: created.SessionKey,
		ObjectiveRevision: 1, Prompt: "retry prompt", Purpose: "goal_continuation",
		Status: protocol.GoalContinuationPlanStatusScheduled, Version: 1,
		NextAttemptAt: &next, CreatedAt: now, UpdatedAt: now,
	}
	event := protocol.GoalEvent{ID: "event-retry", GoalID: created.ID, SessionKey: created.SessionKey, EventType: "continuation_scheduled", Source: protocol.GoalUpdateSourceSystem, RoundID: plan.RoundID, CreatedAt: now}
	if _, err = repository.ReserveGoalContinuation(ctx, *created, 1, event, plan); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.ClaimGoalContinuation(ctx, plan.RoundID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = repository.RetryGoalContinuation(ctx, plan.RoundID, "temporary startup failure", now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	current, err := repository.GetGoal(ctx, created.ID)
	if err != nil || current.ContinuationCount != 1 || current.LastError != "" {
		t.Fatalf("Goal after launch retry = %#v, %v", current, err)
	}
	current.Objective = "retargeted"
	current.Metadata = map[string]any{protocol.GoalMetadataObjectiveRevision: int64(2)}
	current.Version++
	current.UpdatedAt = now.Add(2 * time.Minute)
	retargetEvent := protocol.GoalEvent{ID: "event-retarget", GoalID: current.ID, SessionKey: current.SessionKey, EventType: "updated", Source: protocol.GoalUpdateSourceModel, CreatedAt: current.UpdatedAt}
	if _, err = repository.UpdateGoalWithEvents(ctx, *current, 2, []protocol.GoalEvent{retargetEvent}); err != nil {
		t.Fatal(err)
	}
	if open, err := repository.GetOpenGoalContinuation(ctx, current.ID, 1); err != nil || open != nil {
		t.Fatalf("old revision open plan = %#v, %v", open, err)
	}
}

func openGoalContinuationTestDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
