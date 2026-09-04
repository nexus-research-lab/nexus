package auth

import (
	"context"
	"database/sql"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

type controlEntitlementProjectionStore struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

func newControlEntitlementProjectionStore(
	cfgDriver string,
	db *sql.DB,
) *controlEntitlementProjectionStore {
	return &controlEntitlementProjectionStore{
		db:      db,
		dialect: storage.NewSQLDialect(cfgDriver),
	}
}

func (s *controlEntitlementProjectionStore) upsert(
	ctx context.Context,
	ownerUserID string,
	entitlement controlEntitlement,
) error {
	return s.upsertWith(ctx, s.db, ownerUserID, entitlement, s.dialect.TimestampValue(time.Now().UTC()))
}

func (s *controlEntitlementProjectionStore) upsertWith(
	ctx context.Context,
	executor ownerProjectionExecutor,
	ownerUserID string,
	entitlement controlEntitlement,
	projectedAt any,
) error {
	_, err := executor.ExecContext(ctx, `
INSERT INTO owner_entitlements (
    owner_user_id, plan_key, plan_name, monthly_token_limit, updated_at, projected_at
) VALUES (`+s.dialect.BindList(6)+`)
ON CONFLICT(owner_user_id) DO UPDATE SET
    plan_key = excluded.plan_key,
    plan_name = excluded.plan_name,
    monthly_token_limit = excluded.monthly_token_limit,
    updated_at = excluded.updated_at,
    projected_at = excluded.projected_at
WHERE owner_entitlements.updated_at <= excluded.updated_at`,
		ownerUserID,
		entitlement.PlanKey,
		entitlement.PlanName,
		entitlement.MonthlyTokenLimit,
		s.dialect.TimestampValue(entitlement.UpdatedAt),
		projectedAt,
	)
	return err
}
