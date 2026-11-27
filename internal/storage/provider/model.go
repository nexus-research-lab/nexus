// INPUT: Provider 聚合标识、模型卡与默认模型目标。
// OUTPUT: 模型快照读取，以及统一进入 configuration_version CAS 事务的模型写入。
// POS: Provider 模型持久化入口；禁止绕过 Mutation 直接改写 provider_models。
package provider

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (r *Repository) ListModelsByProviderID(ctx context.Context, providerID string) ([]ModelEntity, error) {
	models, err := r.ListModelsByProviderIDs(ctx, []string{providerID})
	if err != nil {
		return nil, err
	}
	return models[strings.TrimSpace(providerID)], nil
}

// ListModelsByProviderIDs 一次读取多个 Provider 的模型快照，组内顺序与单个读取一致。
func (r *Repository) ListModelsByProviderIDs(ctx context.Context, providerIDs []string) (map[string][]ModelEntity, error) {
	ids := uniqueNonEmptyStrings(providerIDs)
	result := make(map[string][]ModelEntity, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for index, id := range ids {
		placeholders = append(placeholders, r.bind(index+1))
		args = append(args, id)
	}
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
	WHERE provider_id IN (`+strings.Join(placeholders, ", ")+`)
	ORDER BY provider_id, enabled DESC, is_default DESC, display_name ASC, model_id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		item, scanErr := scanModelEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[item.ProviderID] = append(result[item.ProviderID], item)
	}
	return result, rows.Err()
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
	if len(items) == 0 {
		return nil
	}
	providerID := strings.TrimSpace(items[0].ProviderID)
	for _, item := range items[1:] {
		if strings.TrimSpace(item.ProviderID) != providerID {
			return errors.New("一次模型写入只能属于一个 Provider")
		}
	}
	version, err := r.configurationVersionByID(ctx, providerID)
	if err != nil {
		return err
	}
	_, err = r.WithProviderMutation(ctx, providerID, version, func(mutation *Mutation) error {
		return mutation.UpsertModels(ctx, items)
	})
	return err
}

func (r *Repository) UpdateModel(ctx context.Context, item ModelEntity) error {
	version, err := r.configurationVersionByID(ctx, item.ProviderID)
	if err != nil {
		return err
	}
	_, err = r.WithProviderMutation(ctx, item.ProviderID, version, func(mutation *Mutation) error {
		return mutation.UpdateModel(ctx, item)
	})
	return err
}

func (r *Repository) UpdateDefaultModel(ctx context.Context, providerID string, modelID string, updatedAt time.Time) error {
	version, err := r.configurationVersionByID(ctx, providerID)
	if err != nil {
		return err
	}
	_, err = r.WithProviderMutation(ctx, providerID, version, func(mutation *Mutation) error {
		return mutation.UpdateDefaultModel(ctx, modelID, updatedAt)
	})
	return err
}
