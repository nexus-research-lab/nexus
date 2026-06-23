package skills

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (r *Repository) ListImportedSkills(ctx context.Context, ownerUserID string) ([]ImportedSkillEntity, error) {
	rows, err := r.db.QueryContext(ctx, importedSkillSelectSQL()+`
WHERE owner_user_id = `+r.bind(1)+`
ORDER BY skill_name ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanImportedSkillRows(rows)
}

func (r *Repository) GetImportedSkill(ctx context.Context, ownerUserID string, skillName string) (*ImportedSkillEntity, error) {
	row := r.db.QueryRowContext(ctx, importedSkillSelectSQL()+`
WHERE owner_user_id = `+r.bind(1)+` AND skill_name = `+r.bind(2),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(skillName),
	)
	item, err := scanImportedSkill(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) UpsertImportedSkill(ctx context.Context, item ImportedSkillEntity) error {
	now := time.Now().UTC()
	if item.LastImportedAt == nil {
		item.LastImportedAt = &now
	}
	args := importedSkillArgs(item)
	if r.isPostgres {
		_, err := r.db.ExecContext(ctx, `
INSERT INTO imported_skills (
    owner_user_id, skill_name, title, description, scope, tags, category_key, category_name,
    recommendation, version, source_id, source_kind, source_ref, source_name, source_trust,
    import_mode, git_url, git_branch, git_path, git_commit, raw_url, detail_url, content_hash,
    last_imported_at, last_checked_at, last_error, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
    $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
ON CONFLICT (owner_user_id, skill_name) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    scope = EXCLUDED.scope,
    tags = EXCLUDED.tags,
    category_key = EXCLUDED.category_key,
    category_name = EXCLUDED.category_name,
    recommendation = EXCLUDED.recommendation,
    version = EXCLUDED.version,
    source_id = EXCLUDED.source_id,
    source_kind = EXCLUDED.source_kind,
    source_ref = EXCLUDED.source_ref,
    source_name = EXCLUDED.source_name,
    source_trust = EXCLUDED.source_trust,
    import_mode = EXCLUDED.import_mode,
    git_url = EXCLUDED.git_url,
    git_branch = EXCLUDED.git_branch,
    git_path = EXCLUDED.git_path,
    git_commit = EXCLUDED.git_commit,
    raw_url = EXCLUDED.raw_url,
    detail_url = EXCLUDED.detail_url,
    content_hash = EXCLUDED.content_hash,
    last_imported_at = EXCLUDED.last_imported_at,
    last_checked_at = EXCLUDED.last_checked_at,
    last_error = EXCLUDED.last_error,
    updated_at = CURRENT_TIMESTAMP`,
			args...,
		)
		return err
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO imported_skills (
    owner_user_id, skill_name, title, description, scope, tags, category_key, category_name,
    recommendation, version, source_id, source_kind, source_ref, source_name, source_trust,
    import_mode, git_url, git_branch, git_path, git_commit, raw_url, detail_url, content_hash,
    last_imported_at, last_checked_at, last_error, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
    CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
ON CONFLICT(owner_user_id, skill_name) DO UPDATE SET
    title = excluded.title,
    description = excluded.description,
    scope = excluded.scope,
    tags = excluded.tags,
    category_key = excluded.category_key,
    category_name = excluded.category_name,
    recommendation = excluded.recommendation,
    version = excluded.version,
    source_id = excluded.source_id,
    source_kind = excluded.source_kind,
    source_ref = excluded.source_ref,
    source_name = excluded.source_name,
    source_trust = excluded.source_trust,
    import_mode = excluded.import_mode,
    git_url = excluded.git_url,
    git_branch = excluded.git_branch,
    git_path = excluded.git_path,
    git_commit = excluded.git_commit,
    raw_url = excluded.raw_url,
    detail_url = excluded.detail_url,
    content_hash = excluded.content_hash,
    last_imported_at = excluded.last_imported_at,
    last_checked_at = excluded.last_checked_at,
    last_error = excluded.last_error,
    updated_at = CURRENT_TIMESTAMP`,
		args...,
	)
	return err
}

func (r *Repository) DeleteImportedSkill(ctx context.Context, ownerUserID string, skillName string) error {
	_, err := r.db.ExecContext(
		ctx,
		"DELETE FROM imported_skills WHERE owner_user_id = "+r.bind(1)+" AND skill_name = "+r.bind(2),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(skillName),
	)
	return err
}

func importedSkillSelectSQL() string {
	return `
SELECT owner_user_id, skill_name, title, description, scope, tags, category_key, category_name,
       recommendation, version, source_id, source_kind, source_ref, source_name, source_trust,
       import_mode, git_url, git_branch, git_path, git_commit, raw_url, detail_url, content_hash,
       last_imported_at, last_checked_at, last_error, created_at, updated_at
FROM imported_skills
`
}

func importedSkillArgs(item ImportedSkillEntity) []any {
	return []any{
		strings.TrimSpace(item.OwnerUserID),
		strings.TrimSpace(item.SkillName),
		strings.TrimSpace(item.Title),
		strings.TrimSpace(item.Description),
		strings.TrimSpace(item.Scope),
		strings.TrimSpace(item.TagsJSON),
		strings.TrimSpace(item.CategoryKey),
		strings.TrimSpace(item.CategoryName),
		strings.TrimSpace(item.Recommendation),
		strings.TrimSpace(item.Version),
		strings.TrimSpace(item.SourceID),
		strings.TrimSpace(item.SourceKind),
		strings.TrimSpace(item.SourceRef),
		strings.TrimSpace(item.SourceName),
		strings.TrimSpace(item.SourceTrust),
		strings.TrimSpace(item.ImportMode),
		strings.TrimSpace(item.GitURL),
		strings.TrimSpace(item.GitBranch),
		strings.TrimSpace(item.GitPath),
		strings.TrimSpace(item.GitCommit),
		strings.TrimSpace(item.RawURL),
		strings.TrimSpace(item.DetailURL),
		strings.TrimSpace(item.ContentHash),
		item.LastImportedAt,
		item.LastCheckedAt,
		strings.TrimSpace(item.LastError),
	}
}

func scanImportedSkillRows(rows *sql.Rows) ([]ImportedSkillEntity, error) {
	items := make([]ImportedSkillEntity, 0)
	for rows.Next() {
		item, err := scanImportedSkill(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanImportedSkill(row rowScanner) (ImportedSkillEntity, error) {
	var item ImportedSkillEntity
	var lastImported sql.NullTime
	var lastChecked sql.NullTime
	if err := row.Scan(
		&item.OwnerUserID,
		&item.SkillName,
		&item.Title,
		&item.Description,
		&item.Scope,
		&item.TagsJSON,
		&item.CategoryKey,
		&item.CategoryName,
		&item.Recommendation,
		&item.Version,
		&item.SourceID,
		&item.SourceKind,
		&item.SourceRef,
		&item.SourceName,
		&item.SourceTrust,
		&item.ImportMode,
		&item.GitURL,
		&item.GitBranch,
		&item.GitPath,
		&item.GitCommit,
		&item.RawURL,
		&item.DetailURL,
		&item.ContentHash,
		&lastImported,
		&lastChecked,
		&item.LastError,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return ImportedSkillEntity{}, err
	}
	if lastImported.Valid {
		item.LastImportedAt = &lastImported.Time
	}
	if lastChecked.Valid {
		item.LastCheckedAt = &lastChecked.Time
	}
	return item, nil
}
