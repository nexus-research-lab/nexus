package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (r *Repository) GetAccount(
	ctx context.Context,
	ownerUserID string,
	periodStart time.Time,
	periodEnd time.Time,
) (*AccountEntity, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT p.owner_user_id, p.username, p.display_name, p.role, p.status,
       e.plan_key, e.plan_name, e.monthly_token_limit,
       COALESCE(SUM(t.total_tokens), 0),
       COUNT(DISTINCT t.session_key),
       COUNT(t.usage_key),
       p.created_at, e.updated_at
FROM owner_profiles p
JOIN owner_entitlements e ON e.owner_user_id = p.owner_user_id
LEFT JOIN token_usage_records t ON t.owner_user_id = p.owner_user_id
  AND t.occurred_at >= `+r.dialect.Bind(1)+`
  AND t.occurred_at < `+r.dialect.Bind(2)+`
WHERE p.owner_user_id = `+r.dialect.Bind(3)+`
GROUP BY p.owner_user_id, p.username, p.display_name, p.role, p.status,
         e.plan_key, e.plan_name, e.monthly_token_limit, p.created_at, e.updated_at`,
		r.dialect.TimestampValue(periodStart),
		r.dialect.TimestampValue(periodEnd),
		ownerUserID,
	)
	account, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) ListUsage(
	ctx context.Context,
	periodStart time.Time,
	periodEnd time.Time,
) ([]UsageEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT b.control_user_id,
       COALESCE(SUM(t.total_tokens), 0),
       COUNT(DISTINCT t.session_key),
       COUNT(t.usage_key)
FROM local_owner_bindings b
LEFT JOIN token_usage_records t ON t.owner_user_id = b.local_owner_key
  AND t.occurred_at >= `+r.dialect.Bind(1)+`
  AND t.occurred_at < `+r.dialect.Bind(2)+`
GROUP BY b.control_user_id
ORDER BY b.control_user_id ASC`,
		r.dialect.TimestampValue(periodStart),
		r.dialect.TimestampValue(periodEnd),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]UsageEntity, 0)
	for rows.Next() {
		var item UsageEntity
		if err = rows.Scan(
			&item.ControlUserID,
			&item.UsedTokens,
			&item.SessionCount,
			&item.MessageCount,
		); err != nil {
			return nil, fmt.Errorf("scan subscription usage: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type accountScanner interface {
	Scan(dest ...any) error
}

func scanAccount(scanner accountScanner) (AccountEntity, error) {
	var account AccountEntity
	var monthlyLimit sql.NullInt64
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
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return AccountEntity{}, fmt.Errorf("scan subscription account: %w", err)
	}
	if monthlyLimit.Valid {
		account.MonthlyTokenLimit = &monthlyLimit.Int64
	}
	return account, nil
}
