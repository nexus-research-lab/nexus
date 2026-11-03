// INPUT: Real SQLite and optional isolated PostgreSQL databases with the complete migration chain.
// OUTPUT: The same owner isolation, CAS, transaction rollback and command replay checks on both drivers.
// POS: Cross-repository integration coverage; each PostgreSQL run owns a fresh disposable schema.
package storage_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	goalrepo "github.com/nexus-research-lab/nexus/internal/storage/goal"
	orchestrationrepo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	"github.com/pressly/goose/v3"
)

func TestDatabaseIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			db, err := storage.OpenMigrationDB(cfg)
			if err != nil {
				t.Fatal(err)
			}
			migrations := filepath.Join("..", "..", "db", "migrations", driver)
			dialect := goose.DialectSQLite3
			if driver == "postgres" {
				dialect = goose.DialectPostgres
			}
			provider, err := goose.NewProvider(dialect, db, os.DirFS(migrations))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = provider.Up(t.Context()); err != nil {
				t.Fatal(err)
			}
			if results, err := provider.Up(t.Context()); err != nil || len(results) != 0 {
				t.Fatalf("migration replay: %v, %v", results, err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = storage.OpenDB(cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			t.Run("room_owner_and_revision", func(t *testing.T) { checkRoomIsolation(t, db, driver) })
			t.Run("goal_cas_and_json", func(t *testing.T) { checkGoalCAS(t, db, cfg) })
			t.Run("goal_event_rollback", func(t *testing.T) { checkGoalRollback(t, db, cfg) })
			t.Run("execution_replay", func(t *testing.T) { checkExecutionReplay(t, db, driver) })
		})
	}
}

func integrationDatabase(t *testing.T, driver string) config.Config {
	t.Helper()
	cfg := config.Config{DatabaseDriver: driver, DatabaseURL: filepath.Join(t.TempDir(), "integration.db")}
	if driver == "sqlite" {
		return cfg
	}
	dsn := os.Getenv("NEXUS_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("set NEXUS_TEST_POSTGRES_URL to run PostgreSQL integration tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("nexus_test_%d", time.Now().UnixNano())
	if _, err = admin.ExecContext(t.Context(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Errorf("cleanup schema: %v", err)
		}
	})
	endpoint, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := endpoint.Query()
	query.Set("search_path", schema)
	endpoint.RawQuery = query.Encode()
	cfg.DatabaseURL = endpoint.String()
	return cfg
}

func checkRoomIsolation(t *testing.T, db *sql.DB, driver string) {
	t.Helper()
	ctx := t.Context()
	repo := roomrepo.NewSQLRepository(driver, db)
	created, err := repo.CreateRoom(ctx, roomrepo.CreateRoomBundle{
		Room:         protocol.RoomRecord{ID: "room-parity", OwnerUserID: "owner-a", RoomType: protocol.RoomTypeGroup, Name: "跨驱动 Room", SkillNames: []string{}},
		Conversation: protocol.ConversationRecord{ID: "conversation-parity", RoomID: "room-parity", ConversationType: protocol.ConversationTypeMain, Title: "main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Room.ConfigurationVersion != 1 {
		t.Fatalf("created version = %d", created.Room.ConfigurationVersion)
	}
	if other, err := repo.GetRoom(ctx, "other-owner", "room-parity"); err != nil || other != nil {
		t.Fatalf("owner isolation result: %+v, %v", other, err)
	}
	version := int64(1)
	if _, err := repo.CreateConversation(ctx, roomrepo.CreateConversationBundle{OwnerUserID: "owner-a", RoomID: "room-parity", ExpectedConfigurationVersion: &version, Conversation: protocol.ConversationRecord{ID: "topic-parity", RoomID: "room-parity", ConversationType: protocol.ConversationTypeTopic, Title: "topic"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateConversationAtVersion(ctx, "owner-a", "room-parity", "topic-parity", "stale title", 1); !errors.Is(err, roomrepo.ErrConfigurationVersionConflict) {
		t.Fatalf("stale update = %v", err)
	}
	current, err := repo.GetRoom(ctx, "owner-a", "room-parity")
	if err != nil || current.Room.ConfigurationVersion != 2 {
		t.Fatalf("failed write changed version: %+v, %v", current, err)
	}
}

func checkGoalCAS(t *testing.T, db *sql.DB, cfg config.Config) {
	t.Helper()
	ctx := t.Context()
	repo := goalrepo.NewRepository(cfg, db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	item := protocol.Goal{ID: "goal-parity", SessionKey: "session-parity", Objective: "保留用户数据", Status: protocol.GoalStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now, Metadata: map[string]any{"source": "integration", "nested": map[string]any{"ok": true}}}
	created, err := repo.CreateGoal(ctx, item)
	if err != nil {
		t.Fatal(err)
	}
	if created.Metadata["source"] != "integration" || !created.CreatedAt.Equal(now) {
		t.Fatalf("JSON/time roundtrip: %+v", created)
	}
	created.Status = protocol.GoalStatusPaused
	created.Version = 2
	if _, err = repo.UpdateGoal(ctx, *created, 1); err != nil {
		t.Fatal(err)
	}
	created.Objective = "stale update"
	created.Version = 3
	if _, err = repo.UpdateGoal(ctx, *created, 1); err == nil {
		t.Fatal("stale Goal update accepted")
	}
	current, err := repo.GetGoal(ctx, item.ID)
	if err != nil || current.Version != 2 || current.Objective != item.Objective {
		t.Fatalf("stale Goal changed state: %+v, %v", current, err)
	}
}

func checkExecutionReplay(t *testing.T, db *sql.DB, driver string) {
	t.Helper()
	ctx := t.Context()
	repo := orchestrationrepo.NewSQLRepository(driver, db)
	command := orchestrationrepo.CreateCommand{Execution: protocol.Execution{ID: "execution-parity", OwnerUserID: "owner-a", SessionKey: "session-execution-parity", ScopeKind: protocol.ExecutionScopeDM, CoordinatorAgentID: "agent-lead", Origin: protocol.ExecutionOriginUserRequest, Objective: "verify replay", CompletionCriteria: []string{"verified"}, Status: protocol.ExecutionStatusActive}, Meta: orchestrationrepo.CommandMeta{CommandID: "command-parity", EventID: "event-parity", ActorKind: protocol.ExecutionActorUser, ActorID: "owner-a"}}
	for range 2 {
		snapshot, err := repo.Create(ctx, command)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Execution.Version != 1 {
			t.Fatalf("replay advanced version: %d", snapshot.Execution.Version)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM execution_events WHERE execution_id = "+storage.NewSQLDialect(driver).Bind(1), "execution-parity").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("replay event count = %d", count)
	}
}

func checkGoalRollback(t *testing.T, db *sql.DB, cfg config.Config) {
	t.Helper()
	repo := goalrepo.NewRepository(cfg, db)
	now := time.Now().UTC().Truncate(time.Microsecond)
	first := protocol.Goal{ID: "atomic-first", SessionKey: "atomic-first-session", Objective: "atomic creation", Status: protocol.GoalStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now}
	event := protocol.GoalEvent{ID: "atomic-event", GoalID: first.ID, SessionKey: first.SessionKey, EventType: "created", Source: protocol.GoalUpdateSourceExternal, CreatedAt: now}
	if _, err := repo.CreateGoalWithEvent(t.Context(), first, event); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "atomic-second"
	second.SessionKey = "atomic-second-session"
	event.GoalID = second.ID
	event.SessionKey = second.SessionKey
	if _, err := repo.CreateGoalWithEvent(t.Context(), second, event); err == nil {
		t.Fatal("duplicate event accepted")
	}
	if item, err := repo.GetGoal(t.Context(), second.ID); err != nil || item != nil {
		t.Fatalf("failed event did not roll back Goal: %+v, %v", item, err)
	}
	if events, err := repo.ListEvents(t.Context(), first.ID, 10); err != nil || len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("original events changed: %+v, %v", events, err)
	}
}
