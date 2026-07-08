package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

func (r *Repository) ListAccounts(ctx context.Context, periodStart time.Time, periodEnd time.Time) ([]AccountEntity, error) {
	query := r.accountQuery("WHERE u.user_id <> "+r.dialect.Bind(3)) + "\nORDER BY u.created_at ASC, u.user_id ASC"
	rows, err := r.db.QueryContext(
		ctx,
		query,
		r.dialect.TimestampValue(periodStart),
		r.dialect.TimestampValue(periodEnd),
		"__system__",
	)
	if err != nil {
		return nil, fmt.Errorf("list subscription accounts: %w", err)
	}
	defer rows.Close()

	var accounts []AccountEntity
	for rows.Next() {
		account, scanErr := scanAccount(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription accounts: %w", err)
	}
	return accounts, nil
}

func (r *Repository) GetAccount(ctx context.Context, ownerUserID string, periodStart time.Time, periodEnd time.Time) (*AccountEntity, error) {
	row := r.db.QueryRowContext(
		ctx,
		r.accountQuery("WHERE u.user_id = "+r.dialect.Bind(3)),
		r.dialect.TimestampValue(periodStart),
		r.dialect.TimestampValue(periodEnd),
		ownerUserID,
	)
	account, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (r *Repository) accountQuery(whereClause string) string {
	return `
SELECT
  u.user_id,
  u.username,
  COALESCE(u.display_name, ''),
  u.role,
  u.status,
  COALESCE(us.plan_key, 'free') AS plan_key,
  COALESCE(sp.display_name, 'Free') AS plan_name,
  sp.monthly_token_limit,
  COALESCE(SUM(t.total_tokens), 0) AS used_tokens,
  COUNT(DISTINCT t.session_key) AS session_count,
  COUNT(t.usage_key) AS message_count,
  us.period_start,
  us.period_end,
  u.created_at,
  u.updated_at
FROM users u
LEFT JOIN user_subscriptions us ON us.owner_user_id = u.user_id
LEFT JOIN subscription_plans sp ON sp.plan_key = COALESCE(us.plan_key, 'free')
LEFT JOIN token_usage_records t ON t.owner_user_id = u.user_id
  AND t.occurred_at >= ` + r.dialect.Bind(1) + `
  AND t.occurred_at < ` + r.dialect.Bind(2) + `
` + whereClause + `
GROUP BY
  u.user_id,
  u.username,
  u.display_name,
  u.role,
  u.status,
  us.plan_key,
  sp.display_name,
  sp.monthly_token_limit,
  us.period_start,
  us.period_end,
  u.created_at,
  u.updated_at`
}

func (r *Repository) UpsertUserSubscription(ctx context.Context, entity UserSubscriptionEntity) error {
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	if entity.UpdatedAt.IsZero() {
		entity.UpdatedAt = now
	}

	query := `
INSERT INTO user_subscriptions (
  owner_user_id,
  plan_key,
  period_start,
  period_end,
  created_at,
  updated_at
) VALUES (
  ` + r.dialect.Bind(1) + `,
  ` + r.dialect.Bind(2) + `,
  ` + r.dialect.Bind(3) + `,
  ` + r.dialect.Bind(4) + `,
  ` + r.dialect.Bind(5) + `,
  ` + r.dialect.Bind(6) + `
)
ON CONFLICT(owner_user_id) DO UPDATE SET
  plan_key = excluded.plan_key,
  period_start = excluded.period_start,
  period_end = excluded.period_end,
  updated_at = excluded.updated_at`

	_, err := r.db.ExecContext(
		ctx,
		query,
		entity.OwnerUserID,
		entity.PlanKey,
		timeValue(entity.PeriodStart, r.dialect),
		timeValue(entity.PeriodEnd, r.dialect),
		r.dialect.TimestampValue(entity.CreatedAt),
		r.dialect.TimestampValue(entity.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert user subscription: %w", err)
	}
	return nil
}

type accountScanner interface {
	Scan(dest ...any) error
}

func scanAccount(scanner accountScanner) (AccountEntity, error) {
	var account AccountEntity
	var monthlyLimit sql.NullInt64
	var periodStart sql.NullTime
	var periodEnd sql.NullTime
	if err := scanner.Scan(
		&account.OwnerUserID,
		&account.Username,
		&account.DisplayName,
		&account.Role,
		&account.UserStatus,
		&account.PlanKey,
		&account.PlanName,
		&monthlyLimit,
		&account.UsedTokens,
		&account.SessionCount,
		&account.MessageCount,
		&periodStart,
		&periodEnd,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return AccountEntity{}, fmt.Errorf("scan subscription account: %w", err)
	}
	if monthlyLimit.Valid {
		account.MonthlyTokenLimit = &monthlyLimit.Int64
	}
	if periodStart.Valid {
		account.PeriodStart = &periodStart.Time
	}
	if periodEnd.Valid {
		account.PeriodEnd = &periodEnd.Time
	}
	return account, nil
}

func timeValue(value *time.Time, dialect storage.SQLDialect) any {
	if value == nil {
		return nil
	}
	return dialect.TimestampValue(*value)
}
