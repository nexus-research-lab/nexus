package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 封装 provider 配置的 SQL 读写。
type Repository struct {
	db         *sql.DB
	isPostgres bool
}

const (
	// VisibilityPublic 表示平台公共 Provider。
	VisibilityPublic = "public"
	// VisibilityPrivate 表示用户私有 Provider。
	VisibilityPrivate = "private"
)

// NewRepository 创建 provider SQL 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

func (r *Repository) bind(index int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

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
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO provider (
		    id, owner_user_id, visibility, provider_kind, provider, preset_key, api_format, display_name, auth_token, base_url,
		    models_path, enabled, last_test_status,
		    last_test_error, last_test_at, created_at, updated_at
		) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`, `+r.bind(10)+`, `+r.bind(11)+`, `+r.bind(12)+`, `+r.bind(13)+`, `+r.bind(14)+`, `+r.bind(15)+`, `+r.bind(16)+`, `+r.bind(17)+`)`,
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
		item.CreatedAt.UTC(),
		item.UpdatedAt.UTC(),
	)
	return err
}

func (r *Repository) Update(ctx context.Context, item Entity) error {
	_, err := r.db.ExecContext(ctx, `
	UPDATE provider
	SET display_name = `+r.bind(1)+`,
		    auth_token = `+r.bind(2)+`,
		    base_url = `+r.bind(3)+`,
		    models_path = `+r.bind(4)+`,
		    enabled = `+r.bind(5)+`,
		    preset_key = `+r.bind(6)+`,
		    api_format = `+r.bind(7)+`,
		    updated_at = `+r.bind(8)+`
		WHERE id = `+r.bind(9),
		item.DisplayName,
		item.AuthToken,
		item.BaseURL,
		item.ModelsPath,
		item.Enabled,
		item.PresetKey,
		item.APIFormat,
		item.UpdatedAt.UTC(),
		item.ID,
	)
	return err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM provider WHERE id = `+r.bind(1), strings.TrimSpace(id))
	return err
}

func (r *Repository) ReplaceRuntimeProviderForOwner(
	ctx context.Context,
	ownerUserID string,
	oldProvider string,
	newProvider string,
	newModel string,
) (int, error) {
	result, err := r.db.ExecContext(ctx, `
	UPDATE runtimes
	SET provider = `+r.bind(1)+`,
	    model = `+r.bind(2)+`,
	    updated_at = `+r.currentTimestamp()+`
	WHERE COALESCE(NULLIF(TRIM(provider), ''), '') = `+r.bind(3)+`
	  AND agent_id IN (
	      SELECT id FROM agents WHERE owner_user_id = `+r.bind(4)+`
	  )`,
		strings.TrimSpace(newProvider),
		strings.TrimSpace(newModel),
		strings.TrimSpace(oldProvider),
		strings.TrimSpace(ownerUserID),
	)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return int(count), nil
}

func (r *Repository) ReplaceRuntimeProviderForPublic(
	ctx context.Context,
	oldProvider string,
	newProvider string,
	newModel string,
) (int, error) {
	result, err := r.db.ExecContext(ctx, `
	UPDATE runtimes
	SET provider = `+r.bind(1)+`,
	    model = `+r.bind(2)+`,
	    updated_at = `+r.currentTimestamp()+`
	WHERE COALESCE(NULLIF(TRIM(provider), ''), '') = `+r.bind(3)+`
	  AND agent_id IN (
	      SELECT a.id
	      FROM agents a
	      WHERE NOT EXISTS (
	          SELECT 1
	          FROM provider private_provider
	          WHERE private_provider.visibility = 'private'
	            AND private_provider.owner_user_id = a.owner_user_id
	            AND private_provider.provider = `+r.bind(4)+`
	      )
	  )`,
		strings.TrimSpace(newProvider),
		strings.TrimSpace(newModel),
		strings.TrimSpace(oldProvider),
		strings.TrimSpace(oldProvider),
	)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return int(count), nil
}

func (r *Repository) UsageCountForOwner(ctx context.Context, ownerUserID string, provider string) (int, error) {
	row := r.db.QueryRowContext(ctx, `
	SELECT COUNT(*)
	FROM runtimes rt
JOIN agents a ON a.id = rt.agent_id
WHERE a.status = 'active'
  AND a.owner_user_id = `+r.bind(1)+`
  AND COALESCE(NULLIF(TRIM(rt.provider), ''), '') = `+r.bind(2), strings.TrimSpace(ownerUserID), strings.TrimSpace(provider))
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) UsageCountForPublic(ctx context.Context, provider string) (int, error) {
	row := r.db.QueryRowContext(ctx, `
	SELECT COUNT(*)
	FROM runtimes rt
JOIN agents a ON a.id = rt.agent_id
WHERE a.status = 'active'
  AND COALESCE(NULLIF(TRIM(rt.provider), ''), '') = `+r.bind(1)+`
  AND NOT EXISTS (
      SELECT 1
      FROM provider private_provider
      WHERE private_provider.visibility = 'private'
        AND private_provider.owner_user_id = a.owner_user_id
        AND private_provider.provider = `+r.bind(2)+`
  )`, strings.TrimSpace(provider), strings.TrimSpace(provider))
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) ListUsageAgentsByOwner(ctx context.Context, ownerUserID string) (map[string][]UsageAgentEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    COALESCE(NULLIF(TRIM(rt.provider), ''), '') AS provider,
    a.id,
    a.name,
    COALESCE(NULLIF(TRIM(p.display_name), ''), a.name) AS display_name,
    COALESCE(a.avatar, ''),
    a.is_main
FROM runtimes rt
JOIN agents a ON a.id = rt.agent_id
LEFT JOIN profiles p ON p.agent_id = a.id
WHERE a.status = 'active'
  AND a.owner_user_id = `+r.bind(1)+`
  AND COALESCE(NULLIF(TRIM(rt.provider), ''), '') <> ''
ORDER BY provider ASC, a.is_main DESC, display_name ASC, a.name ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]UsageAgentEntity{}
	for rows.Next() {
		var item UsageAgentEntity
		if scanErr := rows.Scan(
			&item.Provider,
			&item.AgentID,
			&item.Name,
			&item.DisplayName,
			&item.Avatar,
			&item.IsMain,
		); scanErr != nil {
			return nil, scanErr
		}
		item.Provider = strings.TrimSpace(item.Provider)
		item.Name = strings.TrimSpace(item.Name)
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.Avatar = strings.TrimSpace(item.Avatar)
		result[item.Provider] = append(result[item.Provider], item)
	}
	return result, rows.Err()
}

func (r *Repository) ListUsageAgentsByOwnerProvider(
	ctx context.Context,
	ownerUserID string,
	provider string,
) ([]UsageAgentEntity, error) {
	items, err := r.ListUsageAgentsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	return items[strings.TrimSpace(provider)], nil
}

func (r *Repository) UpdateTestState(ctx context.Context, item Entity) error {
	_, err := r.db.ExecContext(ctx, `
	UPDATE provider
	SET last_test_status = `+r.bind(1)+`,
	    last_test_error = `+r.bind(2)+`,
	    last_test_at = `+r.bind(3)+`,
	    updated_at = `+r.currentTimestamp()+`
	WHERE id = `+r.bind(4),
		item.LastTestStatus,
		item.LastTestError,
		item.LastTestAt,
		item.ID,
	)
	return err
}

func (r *Repository) ListModelsByProviderID(ctx context.Context, providerID string) ([]ModelEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
	SELECT
	    id,
	    provider_id,
	    model_id,
		    display_name,
		    category,
		    enabled,
		    is_default,
		    capabilities_auto_json,
	    capabilities_override_json,
	    context_window,
	    max_output_tokens,
	    provider_options_json,
	    last_seen_at,
	    created_at,
	    updated_at
	FROM provider_models
	WHERE provider_id = `+r.bind(1)+`
		ORDER BY enabled DESC, is_default DESC, display_name ASC, model_id ASC`, strings.TrimSpace(providerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ModelEntity, 0)
	for rows.Next() {
		item, scanErr := scanModelEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetModel(ctx context.Context, providerID string, modelID string) (*ModelEntity, error) {
	row := r.db.QueryRowContext(ctx, `
	SELECT
	    id,
	    provider_id,
	    model_id,
		    display_name,
		    category,
		    enabled,
		    is_default,
		    capabilities_auto_json,
	    capabilities_override_json,
	    context_window,
	    max_output_tokens,
	    provider_options_json,
	    last_seen_at,
	    created_at,
	    updated_at
	FROM provider_models
	WHERE provider_id = `+r.bind(1)+` AND model_id = `+r.bind(2)+`
	LIMIT 1`, strings.TrimSpace(providerID), strings.TrimSpace(modelID))
	item, err := scanModelEntity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) UpsertModels(ctx context.Context, items []ModelEntity) error {
	for _, item := range items {
		if err := r.upsertModel(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) upsertModel(ctx context.Context, item ModelEntity) error {
	_, err := r.db.ExecContext(ctx, `
	INSERT INTO provider_models (
		    id, provider_id, model_id, display_name, category, enabled,
		    is_default, capabilities_auto_json, capabilities_override_json, context_window,
		    max_output_tokens, provider_options_json, last_seen_at, created_at, updated_at
		) VALUES (`+r.bind(1)+`, `+r.bind(2)+`, `+r.bind(3)+`, `+r.bind(4)+`, `+r.bind(5)+`, `+r.bind(6)+`, `+r.bind(7)+`, `+r.bind(8)+`, `+r.bind(9)+`, `+r.bind(10)+`, `+r.bind(11)+`, `+r.bind(12)+`, `+r.bind(13)+`, `+r.bind(14)+`, `+r.bind(15)+`)
		ON CONFLICT (provider_id, model_id) DO UPDATE SET
		    display_name = excluded.display_name,
		    category = excluded.category,
	    capabilities_auto_json = excluded.capabilities_auto_json,
	    context_window = excluded.context_window,
	    max_output_tokens = excluded.max_output_tokens,
	    last_seen_at = excluded.last_seen_at,
	    updated_at = excluded.updated_at`,
		item.ID,
		item.ProviderID,
		item.ModelID,
		item.DisplayName,
		item.Category,
		item.Enabled,
		item.IsDefault,
		item.CapabilitiesAutoJSON,
		item.CapabilitiesOverrideJSON,
		item.ContextWindow,
		item.MaxOutputTokens,
		item.ProviderOptionsJSON,
		item.LastSeenAt.UTC(),
		item.CreatedAt.UTC(),
		item.UpdatedAt.UTC(),
	)
	return err
}

func (r *Repository) UpdateModel(ctx context.Context, item ModelEntity) error {
	_, err := r.db.ExecContext(ctx, `
	UPDATE provider_models
	SET enabled = `+r.bind(1)+`,
	    is_default = `+r.bind(2)+`,
	    capabilities_override_json = `+r.bind(3)+`,
	    context_window = `+r.bind(4)+`,
	    max_output_tokens = `+r.bind(5)+`,
	    provider_options_json = `+r.bind(6)+`,
	    updated_at = `+r.bind(7)+`
	WHERE provider_id = `+r.bind(8)+` AND model_id = `+r.bind(9),
		item.Enabled,
		item.IsDefault,
		item.CapabilitiesOverrideJSON,
		item.ContextWindow,
		item.MaxOutputTokens,
		item.ProviderOptionsJSON,
		item.UpdatedAt.UTC(),
		item.ProviderID,
		item.ModelID,
	)
	return err
}

func (r *Repository) UpdateDefaultModel(ctx context.Context, providerID string, modelID string, updatedAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `
	UPDATE provider_models
	SET is_default = `+r.falseValue()+`,
	    updated_at = `+r.bind(1)+`
	WHERE is_default = `+r.trueValue()+`
	  AND provider_id IN (
	      SELECT candidate.id
	      FROM provider candidate
	      JOIN provider target ON target.id = `+r.bind(2)+`
	      WHERE candidate.provider_kind = target.provider_kind
	        AND (
	            (target.visibility = 'public' AND candidate.visibility = 'public')
	            OR (
	                target.visibility = 'private'
	                AND candidate.visibility = 'private'
	                AND candidate.owner_user_id = target.owner_user_id
	            )
	        )
	  )`,
		updatedAt.UTC(),
		strings.TrimSpace(providerID),
	); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
	UPDATE provider_models
	SET is_default = `+r.trueValue()+`,
	    enabled = `+r.trueValue()+`,
	    updated_at = `+r.bind(1)+`
	WHERE provider_id = `+r.bind(2)+` AND model_id = `+r.bind(3),
		updatedAt.UTC(),
		strings.TrimSpace(providerID),
		strings.TrimSpace(modelID),
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) trueValue() string {
	if r.isPostgres {
		return "true"
	}
	return "1"
}

func (r *Repository) falseValue() string {
	if r.isPostgres {
		return "false"
	}
	return "0"
}

func (r *Repository) currentTimestamp() string {
	if r.isPostgres {
		return "now()"
	}
	return "CURRENT_TIMESTAMP"
}

func scanEntity(scanner interface {
	Scan(dest ...any) error
}) (Entity, error) {
	var item Entity
	var ownerUserID sql.NullString
	var lastTestStatus sql.NullString
	var lastTestError sql.NullString
	var lastTestAt sql.NullTime
	err := scanner.Scan(
		&item.ID,
		&ownerUserID,
		&item.Visibility,
		&item.ProviderKind,
		&item.Provider,
		&item.PresetKey,
		&item.APIFormat,
		&item.DisplayName,
		&item.AuthToken,
		&item.BaseURL,
		&item.ModelsPath,
		&item.Enabled,
		&lastTestStatus,
		&lastTestError,
		&lastTestAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return Entity{}, err
	}
	if ownerUserID.Valid {
		item.OwnerUserID = strings.TrimSpace(ownerUserID.String)
	}
	item.Visibility = strings.TrimSpace(item.Visibility)
	if item.Visibility == "" {
		item.Visibility = VisibilityPublic
	}
	item.Provider = strings.TrimSpace(item.Provider)
	item.ProviderKind = strings.TrimSpace(item.ProviderKind)
	if item.ProviderKind == "" {
		item.ProviderKind = "llm"
	}
	item.PresetKey = strings.TrimSpace(item.PresetKey)
	if item.PresetKey == "" {
		item.PresetKey = "custom"
	}
	item.APIFormat = strings.TrimSpace(item.APIFormat)
	if item.APIFormat == "" {
		item.APIFormat = "anthropic_messages"
	}
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.AuthToken = strings.TrimSpace(item.AuthToken)
	item.BaseURL = strings.TrimSpace(item.BaseURL)
	item.ModelsPath = strings.TrimSpace(item.ModelsPath)
	if item.ModelsPath == "" {
		item.ModelsPath = "/v1/models"
	}
	item.LastTestStatus = strings.TrimSpace(lastTestStatus.String)
	item.LastTestError = strings.TrimSpace(lastTestError.String)
	if lastTestAt.Valid {
		value := lastTestAt.Time.UTC()
		item.LastTestAt = &value
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

func nullableOwnerUserID(item Entity) any {
	ownerUserID := strings.TrimSpace(item.OwnerUserID)
	if strings.TrimSpace(item.Visibility) != VisibilityPrivate || ownerUserID == "" {
		return nil
	}
	return ownerUserID
}

func scanModelEntity(scanner interface {
	Scan(dest ...any) error
}) (ModelEntity, error) {
	var item ModelEntity
	var contextWindow sql.NullInt64
	var maxOutputTokens sql.NullInt64
	err := scanner.Scan(
		&item.ID,
		&item.ProviderID,
		&item.ModelID,
		&item.DisplayName,
		&item.Category,
		&item.Enabled,
		&item.IsDefault,
		&item.CapabilitiesAutoJSON,
		&item.CapabilitiesOverrideJSON,
		&contextWindow,
		&maxOutputTokens,
		&item.ProviderOptionsJSON,
		&item.LastSeenAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return ModelEntity{}, err
	}
	item.ProviderID = strings.TrimSpace(item.ProviderID)
	item.ModelID = strings.TrimSpace(item.ModelID)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	if item.DisplayName == "" {
		item.DisplayName = item.ModelID
	}
	item.Category = strings.TrimSpace(item.Category)
	item.CapabilitiesAutoJSON = normalizeJSONText(item.CapabilitiesAutoJSON)
	item.CapabilitiesOverrideJSON = normalizeJSONText(item.CapabilitiesOverrideJSON)
	item.ProviderOptionsJSON = normalizeJSONText(item.ProviderOptionsJSON)
	if contextWindow.Valid {
		value := int(contextWindow.Int64)
		item.ContextWindow = &value
	}
	if maxOutputTokens.Valid {
		value := int(maxOutputTokens.Int64)
		item.MaxOutputTokens = &value
	}
	item.LastSeenAt = item.LastSeenAt.UTC()
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

func normalizeJSONText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "{}"
	}
	return trimmed
}
