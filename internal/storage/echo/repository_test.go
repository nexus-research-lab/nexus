package echo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	_ "modernc.org/sqlite"
)

func TestDeliveredCountIsSharedPerOwner(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	_, err = db.Exec(`CREATE TABLE echo_attempts (
owner_user_id TEXT NOT NULL, agent_id TEXT NOT NULL, status TEXT NOT NULL, finished_at DATETIME)`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = db.Exec(
		`INSERT INTO echo_attempts (owner_user_id, agent_id, status, finished_at) VALUES
('owner-1', 'agent-1', 'delivered', ?), ('owner-1', 'agent-2', 'delivered', ?),
('owner-2', 'agent-3', 'delivered', ?)`,
		now,
		now,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := repository.CountDeliveredSince(context.Background(), "owner-1", now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 2 {
		t.Fatalf("delivered = %d, want global count across both Agents", delivered)
	}
}

func TestAggregateTimesSupportSQLiteValues(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	_, err = db.Exec(`CREATE TABLE echo_attempts (
owner_user_id TEXT NOT NULL, session_key TEXT NOT NULL, status TEXT NOT NULL,
due_at DATETIME, finished_at DATETIME)`)
	if err != nil {
		t.Fatal(err)
	}
	dueAt := time.Date(2026, 8, 21, 6, 30, 0, 0, time.UTC)
	finishedAt := dueAt.Add(-time.Hour)
	_, err = db.Exec(
		`INSERT INTO echo_attempts (owner_user_id, session_key, status, due_at, finished_at)
VALUES ('owner-1', 'session-1', 'scheduled', ?, NULL),
('owner-1', 'session-1', 'delivered', NULL, ?)`,
		dueAt,
		finishedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	nextDueAt, err := repository.NextDueAt(context.Background())
	if err != nil || nextDueAt == nil || !nextDueAt.Equal(dueAt) {
		t.Fatalf("NextDueAt() = %v, %v; want %v", nextDueAt, err, dueAt)
	}
	lastDeliveredAt, err := repository.LastDeliveredAtForSession(
		context.Background(),
		"owner-1",
		"session-1",
	)
	if err != nil || lastDeliveredAt == nil || !lastDeliveredAt.Equal(finishedAt) {
		t.Fatalf("LastDeliveredAtForSession() = %v, %v; want %v", lastDeliveredAt, err, finishedAt)
	}
}
