package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (r *Repository) ListPlans(ctx context.Context) ([]PlanEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT plan_key, display_name, status, monthly_token_limit, notes, sort_order, created_at, updated_at
FROM subscription_plans
ORDER BY sort_order ASC, plan_key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list subscription plans: %w", err)
	}
	defer rows.Close()

	var plans []PlanEntity
	for rows.Next() {
		plan, scanErr := scanPlan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscription plans: %w", err)
	}
	return plans, nil
}

func (r *Repository) GetPlan(ctx context.Context, planKey string) (*PlanEntity, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT plan_key, display_name, status, monthly_token_limit, notes, sort_order, created_at, updated_at
FROM subscription_plans
WHERE plan_key = `+r.dialect.Bind(1), planKey)

	plan, err := scanPlan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *Repository) UpsertPlan(ctx context.Context, entity PlanEntity) error {
	now := time.Now().UTC()
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = now
	}
	if entity.UpdatedAt.IsZero() {
		entity.UpdatedAt = now
	}
	if entity.SortOrder == 0 {
		entity.SortOrder = 100
	}

	query := `
INSERT INTO subscription_plans (
  plan_key,
  display_name,
  status,
  monthly_token_limit,
  notes,
  sort_order,
  created_at,
  updated_at
) VALUES (
  ` + r.dialect.Bind(1) + `,
  ` + r.dialect.Bind(2) + `,
  ` + r.dialect.Bind(3) + `,
  ` + r.dialect.Bind(4) + `,
  ` + r.dialect.Bind(5) + `,
  ` + r.dialect.Bind(6) + `,
  ` + r.dialect.Bind(7) + `,
  ` + r.dialect.Bind(8) + `
)
ON CONFLICT(plan_key) DO UPDATE SET
  display_name = excluded.display_name,
  status = excluded.status,
  monthly_token_limit = excluded.monthly_token_limit,
  notes = excluded.notes,
  sort_order = excluded.sort_order,
  updated_at = excluded.updated_at`

	_, err := r.db.ExecContext(
		ctx,
		query,
		entity.PlanKey,
		entity.DisplayName,
		entity.Status,
		entity.MonthlyTokenLimit,
		entity.Notes,
		entity.SortOrder,
		r.dialect.TimestampValue(entity.CreatedAt),
		r.dialect.TimestampValue(entity.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert subscription plan: %w", err)
	}
	return nil
}

type planScanner interface {
	Scan(dest ...any) error
}

func scanPlan(scanner planScanner) (PlanEntity, error) {
	var entity PlanEntity
	var monthlyLimit sql.NullInt64
	if err := scanner.Scan(
		&entity.PlanKey,
		&entity.DisplayName,
		&entity.Status,
		&monthlyLimit,
		&entity.Notes,
		&entity.SortOrder,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	); err != nil {
		return PlanEntity{}, fmt.Errorf("scan subscription plan: %w", err)
	}
	if monthlyLimit.Valid {
		entity.MonthlyTokenLimit = &monthlyLimit.Int64
	}
	return entity, nil
}
