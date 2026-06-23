package skills

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (r *Repository) ListSources(ctx context.Context, ownerUserID string) ([]SourceEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT owner_user_id, source_id, name, kind, url, trust, enabled, sort_order,
       last_checked_at, last_error, created_at, updated_at
FROM skill_sources
WHERE owner_user_id = `+r.bind(1)+`
ORDER BY sort_order ASC, created_at ASC, name ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSourceRows(rows)
}

func (r *Repository) ListEnabledSources(ctx context.Context, ownerUserID string) ([]SourceEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT owner_user_id, source_id, name, kind, url, trust, enabled, sort_order,
       last_checked_at, last_error, created_at, updated_at
FROM skill_sources
WHERE owner_user_id = `+r.bind(1)+` AND enabled = `+r.boolLiteral(true)+`
ORDER BY sort_order ASC, created_at ASC, name ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSourceRows(rows)
}

func (r *Repository) GetSource(ctx context.Context, ownerUserID string, sourceID string) (*SourceEntity, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT owner_user_id, source_id, name, kind, url, trust, enabled, sort_order,
       last_checked_at, last_error, created_at, updated_at
FROM skill_sources
WHERE owner_user_id = `+r.bind(1)+` AND source_id = `+r.bind(2),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(sourceID),
	)
	item, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) EnsureSource(ctx context.Context, item SourceEntity) error {
	if r.isPostgres {
		_, err := r.db.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, enabled, sort_order, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (owner_user_id, kind, url) DO NOTHING`,
			strings.TrimSpace(item.OwnerUserID),
			strings.TrimSpace(item.SourceID),
			strings.TrimSpace(item.Name),
			strings.TrimSpace(item.Kind),
			strings.TrimSpace(item.URL),
			strings.TrimSpace(item.Trust),
			item.Enabled,
			item.SortOrder,
		)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, enabled, sort_order, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(owner_user_id, kind, url) DO NOTHING`,
		strings.TrimSpace(item.OwnerUserID),
		strings.TrimSpace(item.SourceID),
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Kind),
		strings.TrimSpace(item.URL),
		strings.TrimSpace(item.Trust),
		item.Enabled,
		item.SortOrder,
	)
	return err
}

func (r *Repository) UpsertSource(ctx context.Context, item SourceEntity) error {
	if r.isPostgres {
		_, err := r.db.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, enabled, sort_order, last_error, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (owner_user_id, source_id) DO UPDATE SET
    name = EXCLUDED.name,
    kind = EXCLUDED.kind,
    url = EXCLUDED.url,
    trust = EXCLUDED.trust,
    enabled = EXCLUDED.enabled,
    sort_order = EXCLUDED.sort_order,
    last_error = EXCLUDED.last_error,
    updated_at = CURRENT_TIMESTAMP`,
			sourceArgs(item)...,
		)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, enabled, sort_order, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(owner_user_id, source_id) DO UPDATE SET
    name = excluded.name,
    kind = excluded.kind,
    url = excluded.url,
    trust = excluded.trust,
    enabled = excluded.enabled,
    sort_order = excluded.sort_order,
    last_error = excluded.last_error,
    updated_at = CURRENT_TIMESTAMP`,
		sourceArgs(item)...,
	)
	return err
}

func (r *Repository) RecordSourceCheck(ctx context.Context, ownerUserID string, sourceID string, checkedAt time.Time, lastError string) error {
	if r.isPostgres {
		_, err := r.db.ExecContext(
			ctx,
			"UPDATE skill_sources SET last_checked_at = $3, last_error = $4, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = $1 AND source_id = $2",
			strings.TrimSpace(ownerUserID),
			strings.TrimSpace(sourceID),
			checkedAt.UTC(),
			strings.TrimSpace(lastError),
		)
		return err
	}
	_, err := r.db.ExecContext(
		ctx,
		"UPDATE skill_sources SET last_checked_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = ? AND source_id = ?",
		checkedAt.UTC(),
		strings.TrimSpace(lastError),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(sourceID),
	)
	return err
}

func sourceArgs(item SourceEntity) []any {
	return []any{
		strings.TrimSpace(item.OwnerUserID),
		strings.TrimSpace(item.SourceID),
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Kind),
		strings.TrimSpace(item.URL),
		strings.TrimSpace(item.Trust),
		item.Enabled,
		item.SortOrder,
		strings.TrimSpace(item.LastError),
	}
}

func scanSourceRows(rows *sql.Rows) ([]SourceEntity, error) {
	items := make([]SourceEntity, 0)
	for rows.Next() {
		item, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanSource(row rowScanner) (SourceEntity, error) {
	var item SourceEntity
	var lastChecked sql.NullTime
	if err := row.Scan(
		&item.OwnerUserID,
		&item.SourceID,
		&item.Name,
		&item.Kind,
		&item.URL,
		&item.Trust,
		&item.Enabled,
		&item.SortOrder,
		&lastChecked,
		&item.LastError,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return SourceEntity{}, err
	}
	if lastChecked.Valid {
		item.LastCheckedAt = &lastChecked.Time
	}
	return item, nil
}
