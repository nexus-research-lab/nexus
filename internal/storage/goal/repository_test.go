package goal

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"path/filepath"
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

func TestRepositoryPreservesAuthoritativeZeroActualTotal(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:         "goal-authoritative-zero",
		SessionKey: "agent:nexus:ws:dm:authoritative-zero",
		Objective:  "preserve provider total",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.db.ExecContext(
		ctx,
		`UPDATE session_goals
SET token_used_input = 9,
    token_used_output = 1,
    token_used_total = 10,
    token_used_actual_total = 0,
    token_used_actual_estimated = 0
WHERE goal_id = ?`,
		created.ID,
	); err != nil {
		t.Fatal(err)
	}

	stored, err := repository.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Usage.ActualTokens() != 0 || stored.Usage.ActualTokensAreEstimated() {
		t.Fatalf("stored = %#v, want authoritative exact actual zero", stored)
	}
	if stored.Usage.BudgetTokens() != 10 {
		t.Fatalf("stored budget = %d, want 10", stored.Usage.BudgetTokens())
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

func TestRepositoryFinalizeGoalUsagePersistsFenceAndEventAtomically(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID:          "goal-finalize",
		SessionKey:  "agent:nexus:ws:dm:finalize",
		Objective:   "settle terminal usage",
		Status:      protocol.GoalStatusComplete,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: &now,
	})
	if err != nil {
		t.Fatal(err)
	}

	finalizedAt := now.Add(time.Second)
	item := *created
	item.Usage = item.Usage.Add(protocol.GoalUsage{
		InputTokens:       10,
		OutputTokens:      2,
		ActualTotalTokens: 42,
		ActualTotalKnown:  true,
		RuntimeSeconds:    3,
	})
	item.TimeUsedSeconds = 3
	item.UsageFinalized = true
	item.UsageFinalizedAt = &finalizedAt
	item.Version = 2
	item.UpdatedAt = finalizedAt
	event := protocol.GoalEvent{
		ID:         "event-finalized",
		GoalID:     item.ID,
		SessionKey: item.SessionKey,
		EventType:  "usage_finalized",
		Source:     protocol.GoalUpdateSourceSystem,
		RoundID:    "round-final",
		Payload:    map[string]any{"usage_finalized": true},
		CreatedAt:  finalizedAt,
	}

	finalized, err := repository.FinalizeGoalUsage(ctx, item, created.Version, event)
	if err != nil {
		t.Fatal(err)
	}
	if !finalized.UsageFinalized || finalized.UsageFinalizedAt == nil ||
		!finalized.UsageFinalizedAt.Equal(finalizedAt) ||
		finalized.Usage.BudgetTokens() != 12 ||
		finalized.Usage.ActualTokens() != 42 ||
		finalized.TimeUsedSeconds != 3 {
		t.Fatalf("finalized = %#v, want persisted final aggregate and fence", finalized)
	}
	events, err := repository.ListEvents(ctx, item.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventType != "usage_finalized" || events[0].RoundID != "round-final" {
		t.Fatalf("events = %#v, want atomic usage_finalized event", events)
	}
	current, err := repository.GetCurrentGoal(ctx, item.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil for completed finalized Goal", current)
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

func TestGoalUsageFinalizationMigrationLeavesHistoricalGoalsUnfinalized(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigrationFiles(t, db,
		"../../../db/migrations/sqlite/00037_session_goals_compat.sql",
		"../../../db/migrations/sqlite/00051_goal_token_totals.sql",
	)
	if _, err := db.Exec(`INSERT INTO session_goals (
		goal_id, session_key, objective, status, version,
		created_at, updated_at, completed_at, metadata_json
	) VALUES (
		'goal-historical-complete', 'agent:nexus:ws:dm:historical', 'done', 'complete', 1,
		CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '{}'
	)`); err != nil {
		t.Fatal(err)
	}
	applyGoalMigrationFiles(t, db, "../../../db/migrations/sqlite/00054_goal_usage_finalization.sql")

	var finalized bool
	var finalizedAt sql.NullTime
	if err := db.QueryRow(
		`SELECT usage_finalized, usage_finalized_at
		 FROM session_goals WHERE goal_id = 'goal-historical-complete'`,
	).Scan(&finalized, &finalizedAt); err != nil {
		t.Fatal(err)
	}
	if finalized || finalizedAt.Valid {
		t.Fatalf("historical finalized = %v at=%v, want false/null without terminal fence evidence", finalized, finalizedAt)
	}
}

func TestGoalUsageFinalizationMigrationsKeepSQLiteAndPostgresContract(t *testing.T) {
	for _, path := range []string{
		"../../../db/migrations/sqlite/00054_goal_usage_finalization.sql",
		"../../../db/migrations/postgres/00054_goal_usage_finalization.sql",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlText := strings.ToLower(string(body))
		for _, fragment := range []string{"usage_finalized", "usage_finalized_at", "default"} {
			if !strings.Contains(sqlText, fragment) {
				t.Fatalf("migration %s missing %q", path, fragment)
			}
		}
		if !strings.Contains(sqlText, "default 0") && !strings.Contains(sqlText, "default false") {
			t.Fatalf("migration %s must default usage_finalized to false", path)
		}
		if strings.Contains(sqlText, "update session_goals") {
			t.Fatalf("migration %s must not mark historical Goals finalized", path)
		}
	}
}

func TestGoalUsageSourceBaselineMigrationsKeepSQLiteAndPostgresContract(t *testing.T) {
	for _, path := range []string{
		"../../../db/migrations/sqlite/00055_goal_usage_source_baseline.sql",
		"../../../db/migrations/postgres/00055_goal_usage_source_baseline.sql",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlText := strings.ToLower(string(body))
		for _, fragment := range []string{
			"goal_usage_source_evidence",
			"baseline_unavailable",
			"not null",
			"default",
		} {
			if !strings.Contains(sqlText, fragment) {
				t.Fatalf("migration %s missing %q", path, fragment)
			}
		}
		if !strings.Contains(sqlText, "default 0") &&
			!strings.Contains(sqlText, "default false") {
			t.Fatalf("migration %s must default baseline_unavailable to false", path)
		}
	}
}

func TestGoalUsageBaselineMigrationRepairsAppliedVersion54WithoutSourceTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigrationFiles(t, db,
		"../../../db/migrations/sqlite/00037_session_goals_compat.sql",
		"../../../db/migrations/sqlite/00051_goal_token_totals.sql",
		"../../../db/migrations/sqlite/00052_goal_usage_source_checkpoints.sql",
		"../../../db/migrations/sqlite/00054_goal_usage_finalization.sql",
	)
	seedAppliedGooseVersions(t, db, 54)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db, "../../../db/migrations/sqlite", 55); err != nil {
		t.Fatal(err)
	}

	for _, tableName := range []string{
		"goal_usage_scope_bindings",
		"goal_usage_source_pending",
		"goal_usage_source_evidence",
		"goal_usage_parent_ledger",
	} {
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

	rows, err := db.Query("PRAGMA table_info(goal_usage_source_evidence)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	hasBaselineUnavailable := false
	for rows.Next() {
		var (
			columnID     int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if err := rows.Scan(
			&columnID,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatal(err)
		}
		if name == "baseline_unavailable" {
			hasBaselineUnavailable = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !hasBaselineUnavailable {
		t.Fatal("goal_usage_source_evidence.baseline_unavailable missing")
	}

	var version int64
	if err := db.QueryRow(
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1",
	).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 55 {
		t.Fatalf("goose version = %d, want 55", version)
	}
}

func TestGoalEventOrphanCleanupMigration(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "goal-event-orphans.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrationDir := "../../../db/migrations/sqlite"
	if err = goose.UpTo(db, migrationDir, 57); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO goal_events (event_id, goal_id, session_key, event_type, source)
VALUES ('orphan-event', 'missing-goal', 'session-orphan', 'created', 'system')
`); err != nil {
		t.Fatalf("seed orphan goal event: %v", err)
	}

	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatal(err)
	}
	var orphanCount int
	if err = db.QueryRow(`
SELECT COUNT(*)
FROM goal_events
WHERE NOT EXISTS (
	SELECT 1
	FROM session_goals
	WHERE session_goals.goal_id = goal_events.goal_id
)
`).Scan(&orphanCount); err != nil {
		t.Fatal(err)
	}
	if orphanCount != 0 {
		t.Fatalf("orphan goal event count = %d, want 0", orphanCount)
	}

	var version int64
	if err = db.QueryRow(
		"SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1",
	).Scan(&version); err != nil {
		t.Fatal(err)
	}
	migrations, err := goose.CollectMigrations(migrationDir, 0, math.MaxInt64)
	if err != nil || len(migrations) == 0 {
		t.Fatalf("collect current migrations: %v", err)
	}
	wantVersion := migrations[len(migrations)-1].Version
	if version != wantVersion {
		t.Fatalf("goose version = %d, want %d", version, wantVersion)
	}
}

func TestGoalEventOrphanCleanupMigrationKeepsDialectContract(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		path := filepath.Join(
			"../../../db/migrations",
			dialect,
			"00058_goal_event_orphan_cleanup.sql",
		)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlText := strings.ToLower(strings.Join(strings.Fields(string(body)), " "))
		for _, required := range []string{
			"delete from goal_events",
			"where not exists",
			"from session_goals",
			"session_goals.goal_id = goal_events.goal_id",
		} {
			if !strings.Contains(sqlText, required) {
				t.Fatalf("%s migration missing %q", dialect, required)
			}
		}
	}
}

func TestRepositoryGoalCompatMigrationCreatesCurrentSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigrationFiles(t, db,
		"../../../db/migrations/sqlite/00037_session_goals_compat.sql",
		"../../../db/migrations/sqlite/00051_goal_token_totals.sql",
		"../../../db/migrations/sqlite/00054_goal_usage_finalization.sql",
	)

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

func TestGoalTokenTotalsMigrationBackfillsHistoricalUsageAsEstimated(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigrationFiles(t, db, "../../../db/migrations/sqlite/00037_session_goals_compat.sql")
	for _, statement := range []string{
		`INSERT INTO session_goals (
			goal_id, session_key, objective, status,
			token_used_input, token_used_output, token_used_cache_creation,
			token_used_cache_read, token_used_reasoning, token_used_total,
			version, created_at, updated_at, metadata_json
		) VALUES (
			'goal-breakdown', 'session-1', 'ship', 'active',
			10, 20, 80, 90, 40, 240,
			1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '{}'
		)`,
		`INSERT INTO session_goals (
			goal_id, session_key, objective, status, token_used_total,
			version, created_at, updated_at, metadata_json
		) VALUES (
			'goal-total-only', 'session-2', 'ship', 'active', 77,
			1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '{}'
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	applyGoalMigrationFiles(t, db, "../../../db/migrations/sqlite/00051_goal_token_totals.sql")

	for _, testCase := range []struct {
		goalID     string
		wantBudget int64
		wantActual int64
	}{
		{goalID: "goal-breakdown", wantBudget: 30, wantActual: 220},
		{goalID: "goal-total-only", wantBudget: 77, wantActual: 77},
	} {
		var budgetTokens, actualTokens int64
		var estimated bool
		if err := db.QueryRow(
			`SELECT token_used_total, token_used_actual_total, token_used_actual_estimated
			 FROM session_goals WHERE goal_id = ?`,
			testCase.goalID,
		).Scan(&budgetTokens, &actualTokens, &estimated); err != nil {
			t.Fatal(err)
		}
		if budgetTokens != testCase.wantBudget || actualTokens != testCase.wantActual || !estimated {
			t.Fatalf("%s totals = %d/%d estimated=%v, want %d/%d true",
				testCase.goalID,
				budgetTokens,
				actualTokens,
				estimated,
				testCase.wantBudget,
				testCase.wantActual,
			)
		}
	}
}

func TestGoalCompatMigrationRunsAfterAppliedVersion36(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	seedAppliedGooseVersions(t, db, 36)
	if _, err := db.Exec(`CREATE TABLE rooms (id VARCHAR(64) NOT NULL PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE conversations (
		id VARCHAR(64) NOT NULL PRIMARY KEY,
		room_id VARCHAR(64) NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	// 该测试只验证 00037 能接续已标记为 version 36 的历史库，避免被后续领域迁移的夹具要求污染。
	if err := goose.UpTo(db, "../../../db/migrations/sqlite", 37); err != nil {
		t.Fatal(err)
	}
	assertGoalTablesExist(t, db)

	var version int64
	if err := db.QueryRow("SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 37 {
		t.Fatalf("goose version = %d, want 37", version)
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
