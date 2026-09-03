package subscription

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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

func TestEnsureQuotaAvailableBlocksAtMonthlyLimit(t *testing.T) {
	service, db := newTestService(t)
	fixedNow := time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	insertUser(t, db, "user-1", "alice", "Alice", "member")

	if err := service.EnsureQuotaAvailable(context.Background(), "user-1"); err != nil {
		t.Fatalf("未使用额度时不应拦截: %v", err)
	}

	insertUsage(t, db, "user-1", "usage-limit", 200000, fixedNow.Add(-time.Hour))
	err := service.EnsureQuotaAvailable(context.Background(), "user-1")
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("达到额度应返回 ErrQuotaExceeded，实际: %v", err)
	}
	message, ok := protocol.ClientErrorMessage(err)
	if !ok || !strings.Contains(message, "当前账号本月的订阅额度已全部用尽") ||
		!strings.Contains(message, "新的 Agent 请求") {
		t.Fatalf("额度错误缺少客户端提示: %q", message)
	}
}

func TestDesktopSystemWithoutExplicitSubscriptionHasNoFreeQuota(t *testing.T) {
	service, db := newTestServiceWithAppMode(t, "desktop")
	fixedNow := time.Date(2026, 7, 15, 8, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	insertUser(t, db, authctx.SystemUserID, authctx.SystemUserID, "Local User", "owner")
	insertUsage(t, db, authctx.SystemUserID, "usage-over-free-limit", 17554299, fixedNow.Add(-time.Hour))

	account, err := service.CurrentAccount(context.Background(), authctx.SystemUserID)
	if err != nil {
		t.Fatalf("读取 desktop 本地订阅失败: %v", err)
	}
	if account != nil {
		t.Fatalf("desktop 本地用户没有显式订阅时不应伪造 Free 套餐: %+v", account)
	}
	if err = service.EnsureQuotaAvailable(context.Background(), authctx.SystemUserID); err != nil {
		t.Fatalf("desktop 本地用户没有显式订阅时不应触发额度门禁: %v", err)
	}
}

func TestDesktopSystemIgnoresExplicitSubscription(t *testing.T) {
	service, db := newTestServiceWithAppMode(t, "desktop")
	insertUser(t, db, authctx.SystemUserID, authctx.SystemUserID, "Local User", "owner")

	if _, err := service.UpdateUserSubscription(context.Background(), UpdateUserSubscriptionInput{
		OwnerUserID: authctx.SystemUserID,
		PlanKey:     PlanAdmin,
	}); err != nil {
		t.Fatalf("写入 desktop 本地用户显式订阅失败: %v", err)
	}
	account, err := service.CurrentAccount(context.Background(), authctx.SystemUserID)
	if err != nil {
		t.Fatalf("读取 desktop 本地用户显式订阅失败: %v", err)
	}
	if account != nil {
		t.Fatalf("desktop 本地用户应屏蔽订阅投影: %+v", account)
	}
}

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	return newTestServiceWithAppMode(t, "")
}

func newTestServiceWithAppMode(t *testing.T, appMode string) (*Service, *sql.DB) {
	t.Helper()

	cfg := handlertest.NewConfig(t)
	cfg.AppMode = appMode
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	t.Cleanup(func() { _ = db.Close() })
	return NewServiceWithDB(cfg, db), db
}

func insertUser(t *testing.T, db *sql.DB, userID string, username string, displayName string, role string) {
	t.Helper()

	_, err := db.Exec(`
INSERT INTO owner_profiles (
  owner_user_id, username, display_name, role, status, created_at, updated_at
)
VALUES (?, ?, ?, ?, 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
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
