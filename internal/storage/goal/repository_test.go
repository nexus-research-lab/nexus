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
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    map[string]any{"source": "test"},
	}

	created, err := repository.CreateGoal(ctx, item)
	if err != nil {
		t.Fatal(err)
	}
	if created.TokenBudget == nil || *created.TokenBudget != budget || created.Metadata["source"] != "test" {
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
	body, err := os.ReadFile("../../../db/migrations/sqlite/00025_session_goals.sql")
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
			t.Fatalf("exec migration statement %q: %v", statement, err)
		}
	}
}
