package goal

import (
	"context"
	"database/sql"
	"errors"
	"os"
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

func TestRepositoryCreateGoalWithEventIsAtomic(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	first := protocol.Goal{
		ID:         "goal-atomic-first",
		SessionKey: "agent:nexus:ws:dm:atomic-first",
		Objective:  "persist atomically",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	firstEvent := protocol.GoalEvent{
		ID:         "event-atomic-created",
		GoalID:     first.ID,
		SessionKey: first.SessionKey,
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceExternal,
		CreatedAt:  now,
	}
	created, err := repository.CreateGoalWithEvent(ctx, first, firstEvent)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != first.ID {
		t.Fatalf("created Goal = %#v, want %q", created, first.ID)
	}
	events, err := repository.ListEvents(ctx, first.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != firstEvent.ID {
		t.Fatalf("created events = %#v, want only %#v", events, firstEvent)
	}

	second := protocol.Goal{
		ID:         "goal-atomic-rollback",
		SessionKey: "agent:nexus:ws:dm:atomic-rollback",
		Objective:  "roll back with event",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now.Add(time.Second),
		UpdatedAt:  now.Add(time.Second),
	}
	duplicateEvent := protocol.GoalEvent{
		ID:         firstEvent.ID,
		GoalID:     second.ID,
		SessionKey: second.SessionKey,
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceExternal,
		CreatedAt:  now.Add(time.Second),
	}
	if _, err := repository.CreateGoalWithEvent(ctx, second, duplicateEvent); err == nil {
		t.Fatal("CreateGoalWithEvent duplicate event error = nil, want transaction failure")
	}
	rolledBack, err := repository.GetGoal(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack != nil {
		t.Fatalf("rolled back Goal = %#v, want nil", rolledBack)
	}
}

func TestRepositoryUpdateGoalWithEventsIsAtomic(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	item := protocol.Goal{
		ID:         "goal-update-atomic",
		SessionKey: "agent:nexus:ws:dm:update-atomic",
		Objective:  "preserve row and event agreement",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	createdEvent := protocol.GoalEvent{
		ID:         "event-update-atomic-created",
		GoalID:     item.ID,
		SessionKey: item.SessionKey,
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceExternal,
		CreatedAt:  now,
	}
	created, err := repository.CreateGoalWithEvent(ctx, item, createdEvent)
	if err != nil {
		t.Fatal(err)
	}
	created.Status = protocol.GoalStatusPaused
	created.Version++
	created.UpdatedAt = now.Add(time.Second)
	duplicateEvent := protocol.GoalEvent{
		ID:         createdEvent.ID,
		GoalID:     created.ID,
		SessionKey: created.SessionKey,
		EventType:  "paused",
		Source:     protocol.GoalUpdateSourceUser,
		CreatedAt:  created.UpdatedAt,
	}
	if _, err := repository.UpdateGoalWithEvents(ctx, *created, 1, []protocol.GoalEvent{duplicateEvent}); err == nil {
		t.Fatal("UpdateGoalWithEvents duplicate event error = nil, want transaction failure")
	}
	rolledBack, err := repository.GetGoal(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack == nil || rolledBack.Version != 1 || rolledBack.Status != protocol.GoalStatusActive {
		t.Fatalf("Goal after event failure = %#v, want active v1", rolledBack)
	}

	pausedEvent := duplicateEvent
	pausedEvent.ID = "event-update-atomic-paused"
	updated, err := repository.UpdateGoalWithEvents(ctx, *created, 1, []protocol.GoalEvent{pausedEvent})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 || updated.Status != protocol.GoalStatusPaused {
		t.Fatalf("updated Goal = %#v, want paused v2", updated)
	}
	events, err := repository.ListEvents(ctx, item.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("Goal events = %#v, want created and paused", events)
	}
}

func TestRepositoryFinalizeGoalUsageRollsBackFenceWhenEventFails(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 11, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:          "goal-finalize-rollback",
		SessionKey:  "agent:nexus:ws:dm:finalize-rollback",
		Objective:   "keep event and fence atomic",
		Status:      protocol.GoalStatusComplete,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}
	event := protocol.GoalEvent{
		ID:         "event-duplicate",
		GoalID:     created.ID,
		SessionKey: created.SessionKey,
		EventType:  "completed",
		Source:     protocol.GoalUpdateSourceModel,
		CreatedAt:  now,
	}
	if err := repository.AppendEvent(ctx, event); err != nil {
		t.Fatal(err)
	}

	finalizedAt := now.Add(time.Second)
	item := *created
	item.Usage = protocol.GoalUsage{ActualTotalTokens: 9, ActualTotalKnown: true}.NormalizeTotals()
	item.UsageFinalized = true
	item.UsageFinalizedAt = &finalizedAt
	item.Version = 2
	item.UpdatedAt = finalizedAt
	event.EventType = "usage_finalized"
	if _, err := repository.FinalizeGoalUsage(ctx, item, created.Version, event); err == nil {
		t.Fatal("FinalizeGoalUsage() error = nil, want duplicate event failure")
	}

	stored, err := repository.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.UsageFinalized || stored.UsageFinalizedAt != nil ||
		stored.Usage.ActualTokens() != 0 || stored.Version != created.Version {
		t.Fatalf("stored = %#v, want usage/fence update rolled back with event", stored)
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
