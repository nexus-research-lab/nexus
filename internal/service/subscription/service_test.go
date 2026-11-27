package subscription

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func TestUsageOverviewAggregatesCurrentMonthByControlUser(t *testing.T) {
	service, db := newTestService(t)
	fixedNow := time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	insertControlProjection(t, db, "owner-1", "control-1", 200000)
	insertUsage(t, db, "owner-1", "usage-current", 1200, fixedNow.Add(-2*time.Hour))
	insertUsage(t, db, "owner-1", "usage-previous", 9000, time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC))

	overview, err := service.UsageOverview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Accounts) != 1 || overview.Accounts[0].ControlUserID != "control-1" {
		t.Fatalf("usage accounts = %+v", overview.Accounts)
	}
	if overview.Accounts[0].UsedTokens != 1200 || overview.Accounts[0].MessageCount != 1 {
		t.Fatalf("current usage = %+v", overview.Accounts[0])
	}
}

func TestServerFailsClosedWithoutEntitlementProjection(t *testing.T) {
	service, db := newTestService(t)
	insertOwnerProfile(t, db, "owner-missing")
	if err := service.EnsureQuotaAvailable(context.Background(), "owner-missing"); !errors.Is(err, ErrEntitlementUnavailable) {
		t.Fatalf("缺少 Control entitlement 应 fail closed，实际: %v", err)
	}
}

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	t.Cleanup(func() { _ = db.Close() })
	return NewServiceWithDB(cfg, db), db
}

func insertControlProjection(
	t *testing.T,
	db *sql.DB,
	ownerUserID string,
	controlUserID string,
	limit int64,
) {
	t.Helper()
	insertOwnerProfile(t, db, ownerUserID)
	now := time.Now().UTC()
	if _, err := db.Exec(`
INSERT INTO local_owner_bindings (
  deployment_id, control_user_id, local_owner_key, created_at, updated_at
) VALUES (?, ?, ?, ?, ?)`, "deployment-1", controlUserID, ownerUserID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO owner_entitlements (
  owner_user_id, plan_key, plan_name, monthly_token_limit, updated_at, projected_at
) VALUES (?, 'free', 'Free', ?, ?, ?)`, ownerUserID, limit, now, now); err != nil {
		t.Fatal(err)
	}
}

func insertOwnerProfile(t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO owner_profiles (
  owner_user_id, username, display_name, role, status, created_at, updated_at
) VALUES (?, ?, ?, 'member', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		userID, userID, userID,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func insertUsage(
	t *testing.T,
	db *sql.DB,
	ownerUserID string,
	usageKey string,
	totalTokens int64,
	occurredAt time.Time,
) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO token_usage_records (
  owner_user_id, usage_key, source, session_key, message_id,
  input_tokens, output_tokens, cache_creation_input_tokens,
  cache_read_input_tokens, total_tokens, occurred_at
) VALUES (?, ?, 'test', ?, ?, 0, 0, 0, 0, ?, ?)`,
		ownerUserID,
		usageKey,
		"session-"+usageKey,
		"message-"+usageKey,
		totalTokens,
		occurredAt,
	)
	if err != nil {
		t.Fatal(err)
	}
}
