package provider

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

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
	SET model_id = `+r.bind(1)+`,
	    display_name = `+r.bind(2)+`,
	    enabled = `+r.bind(3)+`,
	    is_default = `+r.bind(4)+`,
	    capabilities_override_json = `+r.bind(5)+`,
	    context_window = `+r.bind(6)+`,
	    max_output_tokens = `+r.bind(7)+`,
	    provider_options_json = `+r.bind(8)+`,
	    updated_at = `+r.bind(9)+`
	WHERE id = `+r.bind(10),
		item.ModelID,
		item.DisplayName,
		item.Enabled,
		item.IsDefault,
		item.CapabilitiesOverrideJSON,
		item.ContextWindow,
		item.MaxOutputTokens,
		item.ProviderOptionsJSON,
		item.UpdatedAt.UTC(),
		item.ID,
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
