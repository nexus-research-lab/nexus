// INPUT: owner-scoped source entity 与 DB/transaction executor。
// OUTPUT: 来源列表、详情、幂等初始化、upsert 与健康元数据写入。
// POS: Skill source SQL 的单一实现，Repository 与 CatalogMutation 共同复用。
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
SELECT owner_user_id, source_id, name, kind, url, trust, managed_by, auth_type,
       credentials_encrypted, enabled, sort_order,
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
SELECT owner_user_id, source_id, name, kind, url, trust, managed_by, auth_type,
       credentials_encrypted, enabled, sort_order,
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
SELECT owner_user_id, source_id, name, kind, url, trust, managed_by, auth_type,
       credentials_encrypted, enabled, sort_order,
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
	return ensureSource(ctx, r.db, r.isPostgres, r.bind, item)
}

func ensureSource(
	ctx context.Context,
	executor sqlExecutor,
	isPostgres bool,
	bind func(int) string,
	item SourceEntity,
) error {
	if isPostgres {
		_, err := executor.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, managed_by, auth_type,
    credentials_encrypted, enabled, sort_order, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (owner_user_id, kind, url) DO NOTHING`,
			strings.TrimSpace(item.OwnerUserID),
			strings.TrimSpace(item.SourceID),
			strings.TrimSpace(item.Name),
			strings.TrimSpace(item.Kind),
			strings.TrimSpace(item.URL),
			strings.TrimSpace(item.Trust),
			strings.TrimSpace(item.ManagedBy),
			strings.TrimSpace(item.AuthType),
			strings.TrimSpace(item.CredentialsEncrypted),
			item.Enabled,
			item.SortOrder,
		)
		return err
	}
	_, err := executor.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, managed_by, auth_type,
    credentials_encrypted, enabled, sort_order, created_at, updated_at
) VALUES (`+bind(1)+`, `+bind(2)+`, `+bind(3)+`, `+bind(4)+`, `+bind(5)+`, `+bind(6)+`, `+bind(7)+`, `+bind(8)+`, `+bind(9)+`, `+bind(10)+`, `+bind(11)+`, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(owner_user_id, kind, url) DO NOTHING`,
		strings.TrimSpace(item.OwnerUserID),
		strings.TrimSpace(item.SourceID),
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Kind),
		strings.TrimSpace(item.URL),
		strings.TrimSpace(item.Trust),
		strings.TrimSpace(item.ManagedBy),
		strings.TrimSpace(item.AuthType),
		strings.TrimSpace(item.CredentialsEncrypted),
		item.Enabled,
		item.SortOrder,
	)
	return err
}

func (r *Repository) UpsertSource(ctx context.Context, item SourceEntity) error {
	return upsertSource(ctx, r.db, r.isPostgres, item)
}

func upsertSource(
	ctx context.Context,
	executor sqlExecutor,
	isPostgres bool,
	item SourceEntity,
) error {
	if isPostgres {
		_, err := executor.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, managed_by, auth_type,
    credentials_encrypted, enabled, sort_order, last_error, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (owner_user_id, source_id) DO UPDATE SET
    name = EXCLUDED.name,
    kind = EXCLUDED.kind,
    url = EXCLUDED.url,
    trust = EXCLUDED.trust,
    managed_by = EXCLUDED.managed_by,
    auth_type = EXCLUDED.auth_type,
    credentials_encrypted = EXCLUDED.credentials_encrypted,
    enabled = EXCLUDED.enabled,
    sort_order = EXCLUDED.sort_order,
    last_error = EXCLUDED.last_error,
    updated_at = CURRENT_TIMESTAMP`,
			sourceArgs(item)...,
		)
		return err
	}
	_, err := executor.ExecContext(ctx, `
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url, trust, managed_by, auth_type,
    credentials_encrypted, enabled, sort_order, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(owner_user_id, source_id) DO UPDATE SET
    name = excluded.name,
    kind = excluded.kind,
    url = excluded.url,
    trust = excluded.trust,
    managed_by = excluded.managed_by,
    auth_type = excluded.auth_type,
    credentials_encrypted = excluded.credentials_encrypted,
    enabled = excluded.enabled,
    sort_order = excluded.sort_order,
    last_error = excluded.last_error,
    updated_at = CURRENT_TIMESTAMP`,
		sourceArgs(item)...,
	)
	return err
}

func (r *Repository) RecordSourceCheck(ctx context.Context, ownerUserID string, sourceID string, checkedAt time.Time, lastError string) error {
	return recordSourceCheck(
		ctx,
		r.db,
		r.isPostgres,
		ownerUserID,
		sourceID,
		checkedAt,
		lastError,
	)
}

func recordSourceCheck(
	ctx context.Context,
	executor sqlExecutor,
	isPostgres bool,
	ownerUserID string,
	sourceID string,
	checkedAt time.Time,
	lastError string,
) error {
	if isPostgres {
		_, err := executor.ExecContext(
			ctx,
			"UPDATE skill_sources SET last_checked_at = $3, last_error = $4, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = $1 AND source_id = $2",
			strings.TrimSpace(ownerUserID),
			strings.TrimSpace(sourceID),
			checkedAt.UTC(),
			strings.TrimSpace(lastError),
		)
		return err
	}
	_, err := executor.ExecContext(
		ctx,
		"UPDATE skill_sources SET last_checked_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = ? AND source_id = ?",
		checkedAt.UTC(),
		strings.TrimSpace(lastError),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(sourceID),
	)
	return err
}

// DeleteSource 删除当前用户的一条 skill 来源。
func (r *Repository) DeleteSource(ctx context.Context, ownerUserID string, sourceID string) error {
	_, err := r.db.ExecContext(
		ctx,
		"DELETE FROM skill_sources WHERE owner_user_id = "+r.bind(1)+" AND source_id = "+r.bind(2),
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
		strings.TrimSpace(item.ManagedBy),
		strings.TrimSpace(item.AuthType),
		strings.TrimSpace(item.CredentialsEncrypted),
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
		&item.ManagedBy,
		&item.AuthType,
		&item.CredentialsEncrypted,
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
