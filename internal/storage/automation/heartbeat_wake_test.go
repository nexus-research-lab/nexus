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
