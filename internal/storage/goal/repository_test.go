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
	if _, err := repository.UpdateGoal(ctx, *updated, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale update error = %v, want sql.ErrNoRows", err)
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

func TestRepositoryCheckpoints(t *testing.T) {
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
	created, err := repository.CreateCheckpoint(ctx, protocol.GoalCheckpoint{
		ID:                "checkpoint-1",
		GoalID:            "goal-1",
		SessionKey:        "agent:nexus:ws:dm:chat",
		Summary:           "First durable summary",
		ContinuationCount: 2,
		Usage:             protocol.GoalUsage{TotalTokens: 42},
		CreatedAt:         now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "checkpoint-1" || created.Usage.TotalTokens != 42 {
		t.Fatalf("created = %#v, want persisted checkpoint", created)
	}
	latest, err := repository.LatestCheckpoint(ctx, "goal-1")
	if err != nil {
		t.Fatal(err)
	}
	if latest == nil || latest.Summary != "First durable summary" {
		t.Fatalf("latest = %#v, want checkpoint summary", latest)
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
	for _, path := range []string{
		"../../../db/migrations/sqlite/00025_session_goals.sql",
		"../../../db/migrations/sqlite/00026_goal_codex_statuses.sql",
	} {
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
