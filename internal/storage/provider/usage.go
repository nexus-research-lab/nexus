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
