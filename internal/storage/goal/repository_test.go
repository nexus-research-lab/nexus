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

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRepositoryGoalLifecycle(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	budget := int64(100)
	item := protocol.Goal{
		ID:              "goal-1",
		SessionKey:      "agent:nexus:ws:dm:chat",
		Objective:       "ship",
		Status:          protocol.GoalStatusActive,
		TokenBudget:     &budget,
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
	if created.TokenBudget == nil || *created.TokenBudget != budget || created.TimeUsedSeconds != 12 || created.Metadata["source"] != "test" {
		t.Fatalf("created = %#v, want persisted budget and metadata", created)
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
	deleted, err = repository.DeleteGoal(ctx, "goal-2")
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("second DeleteGoal(goal-2) = true, want false")
	}
}

func TestRepositoryEvents(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	_, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:         "goal-1",
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "ship",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = repository.AppendEvent(ctx, protocol.GoalEvent{
		ID:         "event-1",
		GoalID:     "goal-1",
		SessionKey: "agent:nexus:ws:dm:chat",
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceUser,
		Payload:    map[string]any{"ok": true},
		CreatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	events, err := repository.ListEvents(ctx, "goal-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Payload["ok"] != true {
		t.Fatalf("events = %#v, want persisted event", events)
	}
}

func TestRepositoryGoalCompatMigrationCreatesCurrentSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigrationFiles(t, db, "../../../db/migrations/sqlite/00037_session_goals_compat.sql")

	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(context.Background(), protocol.Goal{
		ID:              "goal-compat",
		SessionKey:      "agent:nexus:ws:dm:compat",
		Objective:       "continue",
		Status:          protocol.GoalStatusActive,
		TimeUsedSeconds: 3,
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.TimeUsedSeconds != 3 {
		t.Fatalf("compat migration goal = %#v, want time_used_seconds persisted", created)
	}
	if err := repository.AppendEvent(context.Background(), protocol.GoalEvent{
		ID:         "event-compat",
		GoalID:     created.ID,
		SessionKey: created.SessionKey,
		EventType:  "created",
		Source:     protocol.GoalUpdateSourceSystem,
		CreatedAt:  now,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGoalCompatMigrationRunsAfterAppliedVersion36(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	seedAppliedGooseVersions(t, db, 36)

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	assertGoalTablesExist(t, db)

	var version int64
	if err := db.QueryRow("SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 39 {
		t.Fatalf("goose version = %d, want 39", version)
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

func seedAppliedGooseVersions(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	)`); err != nil {
		t.Fatal(err)
	}
	for current := 1; current <= version; current++ {
		if _, err := db.Exec(
			"INSERT INTO goose_db_version(version_id, is_applied) VALUES (?, 1)",
			current,
		); err != nil {
			t.Fatal(err)
		}
	}
}

func assertGoalTablesExist(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, tableName := range []string{"session_goals", "goal_events"} {
		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			tableName,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %s count = %d, want 1", tableName, count)
		}
	}
}
