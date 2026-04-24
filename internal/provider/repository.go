package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type repository struct {
	db         *sql.DB
	isPostgres bool
}

func newRepository(cfg config.Config, db *sql.DB) *repository {
	return &repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

func (r *repository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (r *repository) list(ctx context.Context) ([]entity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    id,
    provider,
    display_name,
    auth_token,
    base_url,
    model,
    enabled,
    is_default,
    created_at,
    updated_at
FROM provider
ORDER BY created_at ASC, provider ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity, 0)
	for rows.Next() {
		item, scanErr := scanEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *repository) getByProvider(ctx context.Context, provider string) (*entity, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
    id,
    provider,
    display_name,
    auth_token,
    base_url,
    model,
    enabled,
    is_default,
    created_at,
    updated_at
FROM provider
WHERE provider = `+r.bind(1)+`
LIMIT 1`, strings.TrimSpace(provider))
	item, err := scanEntity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *repository) create(ctx context.Context, item entity) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO provider (
    id, provider, display_name, auth_token, base_url, model, enabled, is_default, created_at, updated_at
) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`, `+r.bind(10)+`)`,
		item.ID,
		item.Provider,
		item.DisplayName,
		item.AuthToken,
		item.BaseURL,
		item.Model,
		item.Enabled,
		item.IsDefault,
		item.CreatedAt.UTC(),
		item.UpdatedAt.UTC(),
	)
	return err
}

func (r *repository) update(ctx context.Context, item entity) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE provider
SET display_name = `+r.bind(1)+`,
    auth_token = `+r.bind(2)+`,
    base_url = `+r.bind(3)+`,
    model = `+r.bind(4)+`,
    enabled = `+r.bind(5)+`,
    is_default = `+r.bind(6)+`,
    updated_at = `+r.bind(7)+`
WHERE provider = `+r.bind(8),
		item.DisplayName,
		item.AuthToken,
		item.BaseURL,
		item.Model,
		item.Enabled,
		item.IsDefault,
		item.UpdatedAt.UTC(),
		item.Provider,
	)
	return err
}

func (r *repository) updateDefaultFlags(ctx context.Context, targetProvider string) error {
	if strings.TrimSpace(targetProvider) == "" {
		query := `
UPDATE provider
SET is_default = ` + r.falseValue() + `,
    updated_at = ` + r.currentTimestamp()
		_, err := r.db.ExecContext(ctx, query)
		return err
	}
	query := `
UPDATE provider
SET is_default = CASE WHEN provider = ` + r.bind(1) + ` THEN ` + r.trueValue() + ` ELSE ` + r.falseValue() + ` END,
    updated_at = ` + r.currentTimestamp() + `
WHERE enabled = ` + r.trueValue()
	_, err := r.db.ExecContext(ctx, query, strings.TrimSpace(targetProvider))
	return err
}

func (r *repository) delete(ctx context.Context, provider string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM provider WHERE provider = `+r.bind(1), strings.TrimSpace(provider))
	return err
}

func (r *repository) listUsageCounts(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT COALESCE(NULLIF(TRIM(provider), ''), '') AS provider, COUNT(*)
FROM runtimes
WHERE COALESCE(NULLIF(TRIM(provider), ''), '') <> ''
  AND agent_id IN (SELECT id FROM agents WHERE status = 'active')
GROUP BY COALESCE(NULLIF(TRIM(provider), ''), '')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var provider string
		var count int
		if scanErr := rows.Scan(&provider, &count); scanErr != nil {
			return nil, scanErr
		}
		result[strings.TrimSpace(provider)] = count
	}
	return result, rows.Err()
}

func (r *repository) trueValue() string {
	if r.isPostgres {
		return "true"
	}
	return "1"
}

func (r *repository) falseValue() string {
	if r.isPostgres {
		return "false"
	}
	return "0"
}

func (r *repository) currentTimestamp() string {
	if r.isPostgres {
		return "now()"
	}
	return "CURRENT_TIMESTAMP"
}

func scanEntity(scanner interface {
	Scan(dest ...any) error
}) (entity, error) {
	var item entity
	err := scanner.Scan(
		&item.ID,
		&item.Provider,
		&item.DisplayName,
		&item.AuthToken,
		&item.BaseURL,
		&item.Model,
		&item.Enabled,
		&item.IsDefault,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return entity{}, err
	}
	item.Provider = strings.TrimSpace(item.Provider)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.AuthToken = strings.TrimSpace(item.AuthToken)
	item.BaseURL = strings.TrimSpace(item.BaseURL)
	item.Model = strings.TrimSpace(item.Model)
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}
