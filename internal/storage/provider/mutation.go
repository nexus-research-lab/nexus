// INPUT: Provider ID、expected configuration_version 与一次聚合内的 Provider/model/runtime 写入。
// OUTPUT: 单事务 CAS、精确 RowsAffected、单调版本推进与原子提交/回滚。
// POS: Provider、模型、默认选择、测试状态和强制删除重分配的共享事务边界。
package provider

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Mutation 是已经通过 Provider configuration_version CAS 的数据库事务。
type Mutation struct {
	repository *Repository
	tx         *sql.Tx
	providerID string
	version    int64
}

// WithProviderMutation 在一个事务内先推进 Provider 版本，再执行聚合写入。
func (r *Repository) WithProviderMutation(
	ctx context.Context,
	providerID string,
	expectedVersion int64,
	apply func(*Mutation) error,
) (int64, error) {
	mutation, err := r.beginProviderMutation(ctx, providerID, expectedVersion)
	if err != nil {
		return 0, err
	}
	defer func() { _ = mutation.tx.Rollback() }()
	if err = apply(mutation); err != nil {
		return 0, err
	}
	if err = mutation.tx.Commit(); err != nil {
		return 0, err
	}
	return mutation.version, nil
}

func (r *Repository) beginProviderMutation(
	ctx context.Context,
	providerID string,
	expectedVersion int64,
) (*Mutation, error) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return nil, ErrProviderNotFound
	}
	if expectedVersion <= 0 {
		return nil, fmt.Errorf("expected provider configuration_version 必须大于 0")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE provider
		SET configuration_version = configuration_version + 1,
		    updated_at = `+r.currentTimestamp()+`
		WHERE id = `+r.bind(1)+`
		  AND configuration_version = `+r.bind(2),
		providerID,
		expectedVersion,
	)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if affected != 1 {
		var currentVersion int64
		queryErr := tx.QueryRowContext(
			ctx,
			`SELECT configuration_version FROM provider WHERE id = `+r.bind(1),
			providerID,
		).Scan(&currentVersion)
		_ = tx.Rollback()
		if queryErr == sql.ErrNoRows {
			return nil, ErrProviderNotFound
		}
		if queryErr != nil {
			return nil, queryErr
		}
		return nil, fmt.Errorf(
			"%w: expected=%d current=%d",
			ErrConfigurationVersionConflict,
			expectedVersion,
			currentVersion,
		)
	}
	return &Mutation{
		repository: r,
		tx:         tx,
		providerID: providerID,
		version:    expectedVersion + 1,
	}, nil
}

// Version 返回本事务提交后的 Provider configuration_version。
func (m *Mutation) Version() int64 {
	return m.version
}

// UpdateProvider 写入 Provider 主记录；版本已由事务入口推进。
func (m *Mutation) UpdateProvider(ctx context.Context, item Entity) error {
	result, err := m.tx.ExecContext(ctx, `
		UPDATE provider
		SET display_name = `+m.repository.bind(1)+`,
		    auth_token = `+m.repository.bind(2)+`,
		    base_url = `+m.repository.bind(3)+`,
		    models_path = `+m.repository.bind(4)+`,
		    enabled = `+m.repository.bind(5)+`,
		    preset_key = `+m.repository.bind(6)+`,
		    api_format = `+m.repository.bind(7)+`,
		    provider_kind = `+m.repository.bind(8)+`,
		    updated_at = `+m.repository.bind(9)+`
		WHERE id = `+m.repository.bind(10),
		item.DisplayName,
		item.AuthToken,
		item.BaseURL,
		item.ModelsPath,
		item.Enabled,
		item.PresetKey,
		item.APIFormat,
		item.ProviderKind,
		item.UpdatedAt.UTC(),
		m.providerID,
	)
	return requireAffected(result, err, 1, ErrProviderNotFound)
}

// ListModels 读取事务内目标 Provider 的模型快照。
func (m *Mutation) ListModels(ctx context.Context) ([]ModelEntity, error) {
	rows, err := m.tx.QueryContext(ctx, `
		SELECT
		    id, provider_id, model_id, display_name, category, enabled, is_default,
		    capabilities_auto_json, capabilities_override_json, context_window,
		    max_output_tokens, provider_options_json, last_seen_at, created_at, updated_at
		FROM provider_models
		WHERE provider_id = `+m.repository.bind(1)+`
		ORDER BY enabled DESC, is_default DESC, display_name ASC, model_id ASC`,
		m.providerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ModelEntity, 0)
	for rows.Next() {
		item, scanErr := scanModelEntity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetModel 读取事务内目标模型。
func (m *Mutation) GetModel(ctx context.Context, modelID string) (*ModelEntity, error) {
	row := m.tx.QueryRowContext(ctx, `
		SELECT
		    id, provider_id, model_id, display_name, category, enabled, is_default,
		    capabilities_auto_json, capabilities_override_json, context_window,
		    max_output_tokens, provider_options_json, last_seen_at, created_at, updated_at
		FROM provider_models
		WHERE provider_id = `+m.repository.bind(1)+`
		  AND model_id = `+m.repository.bind(2)+`
		LIMIT 1`,
		m.providerID,
		strings.TrimSpace(modelID),
	)
	item, err := scanModelEntity(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpsertModels 合并远端或手工模型卡，且只允许写当前 Provider。
func (m *Mutation) UpsertModels(ctx context.Context, items []ModelEntity) error {
	for _, item := range items {
		if strings.TrimSpace(item.ProviderID) != m.providerID {
			return fmt.Errorf("模型 %s 不属于当前 Provider", item.ModelID)
		}
		result, err := m.tx.ExecContext(ctx, `
			INSERT INTO provider_models (
			    id, provider_id, model_id, display_name, category, enabled,
			    is_default, capabilities_auto_json, capabilities_override_json, context_window,
			    max_output_tokens, provider_options_json, last_seen_at, created_at, updated_at
			) VALUES (`+m.repository.bind(1)+`, `+m.repository.bind(2)+`, `+m.repository.bind(3)+`, `+m.repository.bind(4)+`, `+m.repository.bind(5)+`, `+m.repository.bind(6)+`, `+m.repository.bind(7)+`, `+m.repository.bind(8)+`, `+m.repository.bind(9)+`, `+m.repository.bind(10)+`, `+m.repository.bind(11)+`, `+m.repository.bind(12)+`, `+m.repository.bind(13)+`, `+m.repository.bind(14)+`, `+m.repository.bind(15)+`)
			ON CONFLICT (provider_id, model_id) DO UPDATE SET
			    display_name = excluded.display_name,
			    category = excluded.category,
			    capabilities_auto_json = excluded.capabilities_auto_json,
			    context_window = excluded.context_window,
			    max_output_tokens = excluded.max_output_tokens,
			    last_seen_at = excluded.last_seen_at,
			    updated_at = excluded.updated_at`,
			item.ID,
			m.providerID,
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
		if err = requireAffected(result, err, 1, nil); err != nil {
			return err
		}
	}
	return nil
}

// UpdateModel 更新已存在模型卡，并对不存在返回稳定错误。
func (m *Mutation) UpdateModel(ctx context.Context, item ModelEntity) error {
	if strings.TrimSpace(item.ProviderID) != m.providerID {
		return ErrModelNotFound
	}
	result, err := m.tx.ExecContext(ctx, `
		UPDATE provider_models
		SET model_id = `+m.repository.bind(1)+`,
		    display_name = `+m.repository.bind(2)+`,
		    enabled = `+m.repository.bind(3)+`,
		    is_default = `+m.repository.bind(4)+`,
		    capabilities_override_json = `+m.repository.bind(5)+`,
		    context_window = `+m.repository.bind(6)+`,
		    max_output_tokens = `+m.repository.bind(7)+`,
		    provider_options_json = `+m.repository.bind(8)+`,
		    updated_at = `+m.repository.bind(9)+`
		WHERE id = `+m.repository.bind(10)+`
		  AND provider_id = `+m.repository.bind(11),
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
		m.providerID,
	)
	return requireAffected(result, err, 1, ErrModelNotFound)
}

// DeleteModel 删除当前 Provider 下的指定模型卡。
func (m *Mutation) DeleteModel(ctx context.Context, item ModelEntity) error {
	if strings.TrimSpace(item.ProviderID) != m.providerID {
		return ErrModelNotFound
	}
	result, err := m.tx.ExecContext(ctx, `
		DELETE FROM provider_models
		WHERE id = `+m.repository.bind(1)+`
		  AND provider_id = `+m.repository.bind(2),
		item.ID,
		m.providerID,
	)
	return requireAffected(result, err, 1, ErrModelNotFound)
}

// HasDefaultModelInScope 判断目标 Provider 的 owner/public + provider_kind 范围是否已有默认模型。
func (m *Mutation) HasDefaultModelInScope(ctx context.Context) (bool, error) {
	var count int
	err := m.tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM provider_models model
		JOIN provider candidate ON candidate.id = model.provider_id
		JOIN provider target ON target.id = `+m.repository.bind(1)+`
		WHERE model.enabled = `+m.repository.trueValue()+`
		  AND model.is_default = `+m.repository.trueValue()+`
		  AND candidate.provider_kind = target.provider_kind
		  AND (
		      (target.visibility = 'public' AND candidate.visibility = 'public')
		      OR (
		          target.visibility = 'private'
		          AND candidate.visibility = 'private'
		          AND candidate.owner_user_id = target.owner_user_id
		      )
		  )`,
		m.providerID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateDefaultModel 原子切换同 scope/provider_kind 的默认模型，并推进所有受影响 Provider 版本。
func (m *Mutation) UpdateDefaultModel(ctx context.Context, modelID string, updatedAt time.Time) error {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ErrModelNotFound
	}
	if _, err := m.tx.ExecContext(ctx, `
		UPDATE provider
		SET configuration_version = configuration_version + 1,
		    updated_at = `+m.repository.bind(1)+`
		WHERE id <> `+m.repository.bind(2)+`
		  AND EXISTS (
		      SELECT 1
		      FROM provider_models current_default
		      WHERE current_default.provider_id = provider.id
		        AND current_default.is_default = `+m.repository.trueValue()+`
		  )
		  AND id IN (
		      SELECT candidate.id
		      FROM provider candidate
		      JOIN provider target ON target.id = `+m.repository.bind(3)+`
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
		m.providerID,
		m.providerID,
	); err != nil {
		return err
	}
	if _, err := m.tx.ExecContext(ctx, `
		UPDATE provider_models
		SET is_default = `+m.repository.falseValue()+`,
		    updated_at = `+m.repository.bind(1)+`
		WHERE is_default = `+m.repository.trueValue()+`
		  AND provider_id IN (
		      SELECT candidate.id
		      FROM provider candidate
		      JOIN provider target ON target.id = `+m.repository.bind(2)+`
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
		m.providerID,
	); err != nil {
		return err
	}
	result, err := m.tx.ExecContext(ctx, `
		UPDATE provider_models
		SET is_default = `+m.repository.trueValue()+`,
		    enabled = `+m.repository.trueValue()+`,
		    updated_at = `+m.repository.bind(1)+`
		WHERE provider_id = `+m.repository.bind(2)+`
		  AND model_id = `+m.repository.bind(3),
		updatedAt.UTC(),
		m.providerID,
		modelID,
	)
	return requireAffected(result, err, 1, ErrModelNotFound)
}

// UpdateTestState 写入测试状态；版本已在事务入口推进。
func (m *Mutation) UpdateTestState(ctx context.Context, item Entity) error {
	result, err := m.tx.ExecContext(ctx, `
		UPDATE provider
		SET last_test_status = `+m.repository.bind(1)+`,
		    last_test_error = `+m.repository.bind(2)+`,
		    last_test_at = `+m.repository.bind(3)+`
		WHERE id = `+m.repository.bind(4),
		item.LastTestStatus,
		item.LastTestError,
		item.LastTestAt,
		m.providerID,
	)
	return requireAffected(result, err, 1, ErrProviderNotFound)
}

// RuntimeBindingCount 返回事务内所有状态仍引用当前 Provider 的 Agent 数。
func (m *Mutation) RuntimeBindingCount(ctx context.Context, item Entity) (int, error) {
	var row *sql.Row
	if item.Visibility == VisibilityPublic {
		row = m.tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM runtimes rt
			JOIN agents a ON a.id = rt.agent_id
			WHERE COALESCE(NULLIF(TRIM(rt.provider), ''), '') = `+m.repository.bind(1)+`
			  AND NOT EXISTS (
			      SELECT 1
			      FROM provider private_provider
			      WHERE private_provider.visibility = 'private'
			        AND private_provider.owner_user_id = a.owner_user_id
			        AND private_provider.provider = `+m.repository.bind(2)+`
			  )`,
			strings.TrimSpace(item.Provider),
			strings.TrimSpace(item.Provider),
		)
	} else {
		row = m.tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM runtimes rt
			JOIN agents a ON a.id = rt.agent_id
			WHERE a.owner_user_id = `+m.repository.bind(1)+`
			  AND COALESCE(NULLIF(TRIM(rt.provider), ''), '') = `+m.repository.bind(2),
			strings.TrimSpace(item.OwnerUserID),
			strings.TrimSpace(item.Provider),
		)
	}
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ValidateRuntimeReplacement 在重分配前确认替代 Provider/model 仍存在且启用。
func (m *Mutation) ValidateRuntimeReplacement(
	ctx context.Context,
	replacementProviderID string,
	modelID string,
	expectedVersion int64,
) error {
	lockResult, err := m.tx.ExecContext(ctx, `
		UPDATE provider
		SET configuration_version = configuration_version
		WHERE id = `+m.repository.bind(1)+`
		  AND id <> `+m.repository.bind(2)+`
		  AND configuration_version = `+m.repository.bind(3),
		strings.TrimSpace(replacementProviderID),
		m.providerID,
		expectedVersion,
	)
	if err = requireAffected(lockResult, err, 1, nil); err != nil {
		return fmt.Errorf("替代 Provider 已变化；请重新 plan 后重试: %w", err)
	}
	var count int
	err = m.tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM provider replacement
		JOIN provider_models model ON model.provider_id = replacement.id
		WHERE replacement.id = `+m.repository.bind(1)+`
		  AND replacement.id <> `+m.repository.bind(2)+`
		  AND replacement.enabled = `+m.repository.trueValue()+`
		  AND replacement.configuration_version = `+m.repository.bind(3)+`
		  AND model.model_id = `+m.repository.bind(4)+`
		  AND model.enabled = `+m.repository.trueValue(),
		strings.TrimSpace(replacementProviderID),
		m.providerID,
		expectedVersion,
		strings.TrimSpace(modelID),
	).Scan(&count)
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("替代 Provider/model 已变化；请重新 plan 后重试")
	}
	return nil
}

// ReplaceRuntimeProvider 原子重定向受影响 Agent，并推进各自 runtime_version。
func (m *Mutation) ReplaceRuntimeProvider(
	ctx context.Context,
	deleting Entity,
	newProvider string,
	newModel string,
) ([]string, error) {
	var rows *sql.Rows
	var err error
	if deleting.Visibility == VisibilityPublic {
		rows, err = m.tx.QueryContext(ctx, `
			UPDATE runtimes
			SET provider = `+m.repository.bind(1)+`,
			    model = `+m.repository.bind(2)+`,
			    runtime_version = runtime_version + 1,
			    updated_at = `+m.repository.currentTimestamp()+`
			WHERE COALESCE(NULLIF(TRIM(provider), ''), '') = `+m.repository.bind(3)+`
			  AND agent_id IN (
			      SELECT a.id
			      FROM agents a
			      WHERE NOT EXISTS (
			          SELECT 1
			          FROM provider private_provider
			          WHERE private_provider.visibility = 'private'
			            AND private_provider.owner_user_id = a.owner_user_id
			            AND private_provider.provider = `+m.repository.bind(4)+`
			      )
			  )
			RETURNING agent_id`,
			strings.TrimSpace(newProvider),
			strings.TrimSpace(newModel),
			strings.TrimSpace(deleting.Provider),
			strings.TrimSpace(deleting.Provider),
		)
	} else {
		rows, err = m.tx.QueryContext(ctx, `
			UPDATE runtimes
			SET provider = `+m.repository.bind(1)+`,
			    model = `+m.repository.bind(2)+`,
			    runtime_version = runtime_version + 1,
			    updated_at = `+m.repository.currentTimestamp()+`
			WHERE COALESCE(NULLIF(TRIM(provider), ''), '') = `+m.repository.bind(3)+`
			  AND agent_id IN (
			      SELECT id FROM agents WHERE owner_user_id = `+m.repository.bind(4)+`
			  )
			RETURNING agent_id`,
			strings.TrimSpace(newProvider),
			strings.TrimSpace(newModel),
			strings.TrimSpace(deleting.Provider),
			strings.TrimSpace(deleting.OwnerUserID),
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agentIDs := make([]string, 0)
	for rows.Next() {
		var agentID string
		if err = rows.Scan(&agentID); err != nil {
			return nil, err
		}
		agentID = strings.TrimSpace(agentID)
		if agentID != "" {
			agentIDs = append(agentIDs, agentID)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(agentIDs)
	return agentIDs, nil
}

// DeleteProvider 删除事务目标，并要求恰好删除一行。
func (m *Mutation) DeleteProvider(ctx context.Context) error {
	result, err := m.tx.ExecContext(
		ctx,
		`DELETE FROM provider WHERE id = `+m.repository.bind(1),
		m.providerID,
	)
	return requireAffected(result, err, 1, ErrProviderNotFound)
}

func requireAffected(result sql.Result, err error, expected int64, zeroErr error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == expected {
		return nil
	}
	if affected == 0 && zeroErr != nil {
		return zeroErr
	}
	return fmt.Errorf("unexpected rows affected: got=%d want=%d", affected, expected)
}
