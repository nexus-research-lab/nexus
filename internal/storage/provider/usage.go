package provider

import (
	"context"
	"strings"
)

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

func rowsAffected(result interface{ RowsAffected() (int64, error) }, err error) (int, error) {
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
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

// ListActiveOwnerUserIDs 返回所有仍有可运行 Agent 的用户，用于公共 Provider 失效前的默认模型校验。
func (r *Repository) ListActiveOwnerUserIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT owner_user_id
		FROM agents
		WHERE status = 'active'
		ORDER BY owner_user_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owners := make([]string, 0)
	for rows.Next() {
		var ownerUserID string
		if scanErr := rows.Scan(&ownerUserID); scanErr != nil {
			return nil, scanErr
		}
		if ownerUserID = strings.TrimSpace(ownerUserID); ownerUserID != "" {
			owners = append(owners, ownerUserID)
		}
	}
	return owners, rows.Err()
}

// ListRuntimeBindingsByOwner 返回当前用户活跃 Agent 的显式模型绑定快照。
func (r *Repository) ListRuntimeBindingsByOwner(ctx context.Context, ownerUserID string) ([]RuntimeBindingEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
		    a.id,
		    COALESCE(NULLIF(TRIM(rt.provider), ''), ''),
		    COALESCE(NULLIF(TRIM(rt.model), ''), ''),
		    a.is_main
		FROM agents a
		JOIN runtimes rt ON rt.agent_id = a.id
		WHERE a.status = 'active'
		  AND a.owner_user_id = `+r.bind(1)+`
		ORDER BY a.is_main DESC, a.id ASC`, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RuntimeBindingEntity, 0)
	for rows.Next() {
		var item RuntimeBindingEntity
		if scanErr := rows.Scan(&item.AgentID, &item.Provider, &item.Model, &item.IsMain); scanErr != nil {
			return nil, scanErr
		}
		item.AgentID = strings.TrimSpace(item.AgentID)
		item.Provider = strings.TrimSpace(item.Provider)
		item.Model = strings.TrimSpace(item.Model)
		items = append(items, item)
	}
	return items, rows.Err()
}

// ClearRuntimeSelectionsByOwner 将指定 Agent 的显式模型绑定清空为“跟随默认模型”。
func (r *Repository) ClearRuntimeSelectionsByOwner(
	ctx context.Context,
	ownerUserID string,
	agentIDs []string,
) (int, error) {
	uniqueAgentIDs := uniqueNonEmptyStrings(agentIDs)
	if len(uniqueAgentIDs) == 0 {
		return 0, nil
	}
	placeholders := make([]string, 0, len(uniqueAgentIDs))
	args := make([]any, 0, len(uniqueAgentIDs)+1)
	args = append(args, strings.TrimSpace(ownerUserID))
	for index, agentID := range uniqueAgentIDs {
		placeholders = append(placeholders, r.bind(index+2))
		args = append(args, agentID)
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE runtimes
		SET provider = NULL,
		    model = NULL,
		    updated_at = `+r.currentTimestamp()+`
		WHERE agent_id IN (
		    SELECT id
		    FROM agents
		    WHERE owner_user_id = `+r.bind(1)+`
		      AND id IN (`+strings.Join(placeholders, ", ")+`)
		)`, args...)
	return rowsAffected(result, err)
}

func uniqueNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
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
