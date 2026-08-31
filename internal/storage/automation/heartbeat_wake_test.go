// INPUT: concurrent heartbeat config CAS、owner/request wake intent 与过期 claim。
// OUTPUT: 受理版本线性化、exact idempotency、冲突拒绝和可恢复 deadline。
// POS: migration 00125 durable heartbeat wake outbox 的仓储回归。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func newHeartbeatWakeRepository(t *testing.T) (*sql.DB, *Repository) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "heartbeat-wake.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO agents (
id, owner_user_id, slug, name, description, definition, status, workspace_path
) VALUES ('wake-agent', 'wake-owner', 'wake-agent', 'Wake Agent', '', '', 'active', '/tmp/wake-agent')`); err != nil {
		t.Fatal(err)
	}
	return db, NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
}

func TestHeartbeatWakeMigrationRoundTrip(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "heartbeat-wake-migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, "../../../db/migrations/sqlite", 125); err != nil {
		t.Fatal(err)
	}
	assertHeartbeatWakeColumns := func(want bool) {
		t.Helper()
		rows, queryErr := db.Query(`PRAGMA table_info(automation_system_events)`)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		defer rows.Close()
		found := map[string]bool{}
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, columnType string
			var defaultValue sql.NullString
			if scanErr := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); scanErr != nil {
				t.Fatal(scanErr)
			}
			found[name] = true
		}
		for _, column := range []string{
			"owner_user_id", "request_id", "intent_digest",
			"accepted_configuration_version", "claim_token", "claim_expires_at",
		} {
			if found[column] != want {
				t.Fatalf("column %s present=%v want=%v", column, found[column], want)
			}
		}
	}
	assertHeartbeatWakeColumns(true)
	if err = goose.DownTo(db, "../../../db/migrations/sqlite", 124); err != nil {
		t.Fatal(err)
	}
	assertHeartbeatWakeColumns(false)
}

func TestHeartbeatWakeAcceptanceLinearizesWithConfigurationUpdate(t *testing.T) {
	_, repository := newHeartbeatWakeRepository(t)
	ctx := context.Background()
	configValue := automationdomain.HeartbeatConfig{
		AgentID: "wake-agent", Enabled: true, EverySeconds: 60,
		TargetMode: automationdomain.HeartbeatTargetNone, AckMaxChars: 300,
	}
	if err := repository.UpsertHeartbeatState(ctx, "hb-initial", configValue, nil, nil); err != nil {
		t.Fatal(err)
	}

	for index := 0; index < 12; index++ {
		persisted, _, _, err := repository.GetHeartbeatState(ctx, "wake-agent")
		if err != nil || persisted == nil {
			t.Fatalf("read version: config=%+v err=%v", persisted, err)
		}
		expected := persisted.ConfigurationVersion
		updatedConfig := *persisted
		updatedConfig.AckMaxChars++
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		var wakeResult HeartbeatWakeAcceptanceResult
		var wakeErr, updateErr error
		go func(iteration int) {
			defer wg.Done()
			<-start
			wakeResult, wakeErr = repository.AcceptHeartbeatWake(ctx, HeartbeatWakeAcceptanceInput{
				EventID: "wake-linear-" + time.Unix(int64(iteration), 0).UTC().Format("150405"),
				AgentID: "wake-agent", OwnerUserID: "wake-owner",
				RequestID:                    "wake-linear-request-" + time.Unix(int64(iteration), 0).UTC().Format("150405"),
				IntentDigest:                 "wake-linear-intent-" + time.Unix(int64(iteration), 0).UTC().Format("150405"),
				Mode:                         automationdomain.WakeModeNextHeartbeat,
				ExpectedConfigurationVersion: &expected,
				AcceptedAt:                   time.Now().UTC(),
			})
		}(index)
		go func() {
			defer wg.Done()
			<-start
			updateErr = repository.UpsertHeartbeatStateAtVersion(
				ctx, "hb-update", updatedConfig, nil, nil, expected,
			)
		}()
		close(start)
		wg.Wait()
		if updateErr != nil {
			t.Fatalf("configuration update %d: %v", index, updateErr)
		}
		if wakeErr == nil {
			if wakeResult.Event.AcceptedConfigurationVersion != expected {
				t.Fatalf("wake accepted outside expected version fence: event=%+v expected=%d", wakeResult.Event, expected)
			}
			continue
		}
		if !errors.Is(wakeErr, automationdomain.ErrConfigurationVersionConflict) {
			t.Fatalf("wake race %d returned non-CAS error: %v", index, wakeErr)
		}
	}
}

func TestHeartbeatWakeRequestIsIdempotentAndIntentScoped(t *testing.T) {
	_, repository := newHeartbeatWakeRepository(t)
	ctx := context.Background()
	expected := int64(0)
	input := HeartbeatWakeAcceptanceInput{
		EventID: "wake-idempotent-first", AgentID: "wake-agent", OwnerUserID: "wake-owner",
		RequestID: "wake-idempotent-request", IntentDigest: "wake-idempotent-intent",
		Mode:                         automationdomain.WakeModeNextHeartbeat,
		ExpectedConfigurationVersion: &expected, AcceptedAt: time.Now().UTC(),
	}
	first, err := repository.AcceptHeartbeatWake(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.EventID = "wake-idempotent-retry"
	second, err := repository.AcceptHeartbeatWake(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Event.EventID != first.Event.EventID {
		t.Fatalf("same request did not replay exact wake: first=%+v second=%+v", first, second)
	}
	input.IntentDigest = "different-wake-intent"
	if _, err = repository.AcceptHeartbeatWake(ctx, input); !errors.Is(err, automationdomain.ErrHeartbeatWakeRequestConflict) {
		t.Fatalf("different intent error = %v", err)
	}
}

func TestExpiredHeartbeatWakeClaimFailsClosedWithoutRedispatch(t *testing.T) {
	_, repository := newHeartbeatWakeRepository(t)
	ctx := context.Background()
	acceptedAt := time.Now().UTC().Add(-time.Minute)
	accepted, err := repository.AcceptHeartbeatWake(ctx, HeartbeatWakeAcceptanceInput{
		EventID: "wake-recovery", AgentID: "wake-agent", Mode: automationdomain.WakeModeNow,
		AcceptedAt: acceptedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	claimed, err := repository.ClaimHeartbeatWakeEvent(
		ctx, accepted.Event.EventID, "expired-claim", acceptedAt, now.Add(-time.Second),
	)
	if err != nil || !claimed {
		t.Fatalf("initial claim = %v err=%v", claimed, err)
	}
	agents, err := repository.ListRecoverableHeartbeatWakeAgentIDs(ctx, now)
	if err != nil || len(agents) != 0 {
		t.Fatalf("started wake must not become redispatchable: agents=%+v err=%v", agents, err)
	}
	deadline, err := repository.NextRecoverableHeartbeatWakeAt(ctx)
	if err != nil || deadline == nil || deadline.After(now) {
		t.Fatalf("recovery deadline = %v err=%v", deadline, err)
	}
	failed, err := repository.FailExpiredHeartbeatWakeClaims(ctx, now)
	if err != nil || failed != 1 {
		t.Fatalf("expired claim fail-closed = %d err=%v", failed, err)
	}
	if completed, completeErr := repository.CompleteHeartbeatWakeEvent(
		ctx, accepted.Event.EventID, "expired-claim", "processed",
	); completeErr != nil || completed {
		t.Fatalf("stale claim completed event: completed=%v err=%v", completed, completeErr)
	}
	var status string
	if err = repository.db.QueryRowContext(ctx, `SELECT status FROM automation_system_events WHERE event_id = ?`, accepted.Event.EventID).Scan(&status); err != nil || status != "failed" {
		t.Fatalf("expired wake status = %q err=%v", status, err)
	}
}
