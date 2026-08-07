// INPUT: Provider 可见域、主记录与期望聚合版本。
// OUTPUT: Provider 行读取、创建，以及统一进入 Mutation 的主记录写入。
// POS: provider 表仓储入口；现有聚合写入必须经过 configuration_version CAS。
package provider

import (
	"context"
	"database/sql"
	"strings"
)

func (r *Repository) ListVisible(ctx context.Context, ownerUserID string) ([]Entity, error) {
	rows, err := r.db.QueryContext(ctx, `
	SELECT
	    id,
	    COALESCE(owner_user_id, ''),
	    visibility,
	    provider_kind,
	    provider,
	    preset_key,
	    api_format,
	    display_name,
		    auth_token,
		    base_url,
		    models_path,
		    enabled,
		    last_test_status,
		    last_test_error,
	    last_test_at,
	    configuration_version,
	    created_at,
	    updated_at
	FROM provider
	WHERE visibility = 'public'
	   OR (visibility = 'private' AND owner_user_id = `+r.bind(1)+`)
	ORDER BY
	    CASE WHEN visibility = 'private' THEN 0 ELSE 1 END,
	    created_at ASC,
	    provider ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Entity, 0)
	for rows.Next() {
		item, scanErr := scanEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListPrivate 只返回当前 owner 自己管理的私有 Provider。
func (r *Repository) ListPrivate(ctx context.Context, ownerUserID string) ([]Entity, error) {
	rows, err := r.db.QueryContext(ctx, `
	SELECT
	    id,
	    COALESCE(owner_user_id, ''),
	    visibility,
	    provider_kind,
	    provider,
	    preset_key,
	    api_format,
	    display_name,
	    auth_token,
	    base_url,
	    models_path,
	    enabled,
	    last_test_status,
	    last_test_error,
	    last_test_at,
	    configuration_version,
	    created_at,
	    updated_at
	FROM provider
	WHERE visibility = 'private' AND owner_user_id = `+r.bind(1)+`
	ORDER BY created_at ASC, provider ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Entity, 0)
	for rows.Next() {
		item, scanErr := scanEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListPublic(ctx context.Context) ([]Entity, error) {
	rows, err := r.db.QueryContext(ctx, `
	SELECT
	    id,
	    COALESCE(owner_user_id, ''),
	    visibility,
	    provider_kind,
	    provider,
	    preset_key,
	    api_format,
	    display_name,
		    auth_token,
		    base_url,
		    models_path,
		    enabled,
		    last_test_status,
	    last_test_error,
	    last_test_at,
	    configuration_version,
	    created_at,
	    updated_at
	FROM provider
	WHERE visibility = 'public'
	ORDER BY created_at ASC, provider ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Entity, 0)
	for rows.Next() {
		item, scanErr := scanEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetVisibleByProvider(ctx context.Context, ownerUserID string, provider string) (*Entity, error) {
	row := r.db.QueryRowContext(ctx, `
	SELECT
	    id,
	    COALESCE(owner_user_id, ''),
	    visibility,
	    provider_kind,
	    provider,
	    preset_key,
	    api_format,
	    display_name,
		    auth_token,
		    base_url,
		    models_path,
		    enabled,
		    last_test_status,
	    last_test_error,
	    last_test_at,
	    configuration_version,
	    created_at,
	    updated_at
	FROM provider
WHERE provider = `+r.bind(1)+`
  AND (
      visibility = 'public'
      OR (visibility = 'private' AND owner_user_id = `+r.bind(2)+`)
  )
ORDER BY CASE WHEN visibility = 'private' THEN 0 ELSE 1 END
LIMIT 1`, strings.TrimSpace(provider), strings.TrimSpace(ownerUserID))
	item, err := scanEntity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) GetScopedByProvider(
	ctx context.Context,
	visibility string,
	ownerUserID string,
	provider string,
) (*Entity, error) {
	args := []any{strings.TrimSpace(provider), strings.TrimSpace(visibility)}
	ownerClause := "owner_user_id IS NULL"
	if strings.TrimSpace(visibility) == VisibilityPrivate {
		args = append(args, strings.TrimSpace(ownerUserID))
		ownerClause = "owner_user_id = " + r.bind(3)
	}
	row := r.db.QueryRowContext(ctx, `
	SELECT
	    id,
	    COALESCE(owner_user_id, ''),
	    visibility,
	    provider_kind,
	    provider,
	    preset_key,
	    api_format,
	    display_name,
		    auth_token,
		    base_url,
		    models_path,
		    enabled,
		    last_test_status,
	    last_test_error,
	    last_test_at,
	    configuration_version,
	    created_at,
	    updated_at
	FROM provider
WHERE provider = `+r.bind(1)+`
  AND visibility = `+r.bind(2)+`
  AND `+ownerClause+`
LIMIT 1`, args...)
	item, err := scanEntity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) Create(ctx context.Context, item Entity) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO provider (
		    id, owner_user_id, visibility, provider_kind, provider, preset_key, api_format, display_name, auth_token, base_url,
		    models_path, enabled, last_test_status,
		    last_test_error, last_test_at, configuration_version, created_at, updated_at
		) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`, `+r.bind(10)+`, `+r.bind(11)+`, `+r.bind(12)+`, `+r.bind(13)+`, `+r.bind(14)+`, `+r.bind(15)+`, `+r.bind(16)+`, `+r.bind(17)+`, `+r.bind(18)+`)`,
		item.ID,
		nullableOwnerUserID(item),
		item.Visibility,
		item.ProviderKind,
		item.Provider,
		item.PresetKey,
		item.APIFormat,
		item.DisplayName,
		item.AuthToken,
		item.BaseURL,
		item.ModelsPath,
		item.Enabled,
		item.LastTestStatus,
		item.LastTestError,
		item.LastTestAt,
		normalizedConfigurationVersion(item.ConfigurationVersion),
		item.CreatedAt.UTC(),
		item.UpdatedAt.UTC(),
	)
	return requireAffected(result, err, 1, nil)
}

func (r *Repository) Update(ctx context.Context, item Entity) error {
	version, err := r.configurationVersionByID(ctx, item.ID)
	if err != nil {
		return err
	}
	_, err = r.WithProviderMutation(ctx, item.ID, version, func(mutation *Mutation) error {
		return mutation.UpdateProvider(ctx, item)
	})
	return err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	version, err := r.configurationVersionByID(ctx, id)
	if err != nil {
		return err
	}
	_, err = r.WithProviderMutation(ctx, id, version, func(mutation *Mutation) error {
		return mutation.DeleteProvider(ctx)
	})
	return err
}

func (r *Repository) UpdateTestState(ctx context.Context, item Entity) error {
	version, err := r.configurationVersionByID(ctx, item.ID)
	if err != nil {
		return err
	}
	_, err = r.WithProviderMutation(ctx, item.ID, version, func(mutation *Mutation) error {
		return mutation.UpdateTestState(ctx, item)
	})
	return err
}

func (r *Repository) configurationVersionByID(ctx context.Context, id string) (int64, error) {
	var version int64
	err := r.db.QueryRowContext(
		ctx,
		`SELECT configuration_version FROM provider WHERE id = `+r.bind(1),
		strings.TrimSpace(id),
	).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, ErrProviderNotFound
	}
	if err != nil {
		return 0, err
	}
	return version, nil
}
