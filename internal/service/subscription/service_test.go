package subscription

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func TestOverviewAggregatesCurrentMonthUsage(t *testing.T) {
	service, db := newTestService(t)
	fixedNow := time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	insertUser(t, db, "user-1", "alice", "Alice", "member")
	insertUsage(t, db, "user-1", "usage-current", 1200, fixedNow.Add(-2*time.Hour))
	insertUsage(t, db, "user-1", "usage-previous", 9000, time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC))

	overview, err := service.Overview(context.Background())
	if err != nil {
		t.Fatalf("读取订阅概览失败: %v", err)
	}
	if len(overview.Plans) != 2 {
		t.Fatalf("默认套餐数量 = %d, want 2", len(overview.Plans))
	}
	if len(overview.Accounts) != 1 {
		t.Fatalf("账号数量 = %d, want 1", len(overview.Accounts))
	}

	account := overview.Accounts[0]
	if account.PlanKey != PlanFree {
		t.Fatalf("默认套餐 = %q, want %q", account.PlanKey, PlanFree)
	}
	if account.MonthlyTokenLimit == nil || *account.MonthlyTokenLimit != 200000 {
		t.Fatalf("默认月度额度 = %v, want 200000", account.MonthlyTokenLimit)
	}
	if account.UsedTokens != 1200 {
		t.Fatalf("当月用量 = %d, want 1200", account.UsedTokens)
	}
	if account.MessageCount != 1 {
		t.Fatalf("消息数量 = %d, want 1", account.MessageCount)
	}
}

func TestPlanLimitIsManagedByPlan(t *testing.T) {
	service, db := newTestService(t)
	service.now = func() time.Time {
		return time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	}
	insertUser(t, db, "user-1", "alice", "Alice", "member")

	limit := int64(4096)
	overview, err := service.UpsertPlan(context.Background(), UpsertPlanInput{
		PlanKey:           "team",
		DisplayName:       "Team",
		Status:            PlanStatusActive,
		MonthlyTokenLimit: &limit,
		Notes:             "团队套餐",
	})
	if err != nil {
		t.Fatalf("更新套餐失败: %v", err)
	}
	if len(overview.Plans) != 3 {
		t.Fatalf("套餐数量 = %d, want 3", len(overview.Plans))
	}

	overview, err = service.UpdateUserSubscription(context.Background(), UpdateUserSubscriptionInput{
		OwnerUserID: "user-1",
		PlanKey:     "team",
	})
	if err != nil {
		t.Fatalf("更新用户订阅失败: %v", err)
	}
	if len(overview.Accounts) != 1 {
		t.Fatalf("账号数量 = %d, want 1", len(overview.Accounts))
	}

	account := overview.Accounts[0]
	if account.PlanKey != "team" {
		t.Fatalf("套餐 = %q, want team", account.PlanKey)
	}
	if account.MonthlyTokenLimit == nil || *account.MonthlyTokenLimit != limit {
		t.Fatalf("套餐额度 = %v, want %d", account.MonthlyTokenLimit, limit)
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

func insertUser(t *testing.T, db *sql.DB, userID string, username string, displayName string, role string) {
	t.Helper()

	_, err := db.Exec(`
INSERT INTO users (user_id, username, display_name, role, status)
VALUES (?, ?, ?, ?, 'active')`,
		userID,
		username,
		displayName,
		role,
	)
	if err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
}

func insertUsage(t *testing.T, db *sql.DB, ownerUserID string, usageKey string, totalTokens int64, occurredAt time.Time) {
	t.Helper()

	_, err := db.Exec(`
INSERT INTO token_usage_records (
  owner_user_id,
  usage_key,
  source,
  session_key,
  message_id,
  input_tokens,
  output_tokens,
  cache_creation_input_tokens,
  cache_read_input_tokens,
  total_tokens,
  occurred_at
) VALUES (?, ?, 'test', ?, ?, 0, 0, 0, 0, ?, ?)`,
		ownerUserID,
		usageKey,
		"session-"+usageKey,
		"message-"+usageKey,
		totalTokens,
		occurredAt,
	)
	if err != nil {
		t.Fatalf("插入用量失败: %v", err)
	}
}
