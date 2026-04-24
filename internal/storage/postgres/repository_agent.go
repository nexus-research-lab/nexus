package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// AgentRepository 提供 PostgreSQL 的 Agent 仓储实现。
type AgentRepository struct {
	db *sql.DB
}

// NewAgentRepository 创建 Agent 仓储。
func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// ListActiveAgents 返回所有活跃 Agent。
func (r *AgentRepository) ListActiveAgents(ctx context.Context, ownerUserID string) ([]protocol.Agent, error) {
	query := `
	SELECT
	    a.id,
	    a.name,
	    a.owner_user_id,
	    a.workspace_path,
	    a.status,
	    a.is_main,
	    COALESCE(a.avatar, ''),
	    COALESCE(a.description, ''),
	    COALESCE(a.vibe_tags::text, '[]'),
	    COALESCE(p.display_name, ''),
	    COALESCE(p.headline, ''),
	    COALESCE(p.profile_markdown, ''),
	    a.created_at,
	    COALESCE(rt.provider, ''),
	    COALESCE(rt.permission_mode, ''),
	    COALESCE(rt.allowed_tools_json, '[]'),
    COALESCE(rt.disallowed_tools_json, '[]'),
    COALESCE(rt.mcp_servers_json, '{}'),
    rt.max_turns,
    rt.max_thinking_tokens,
    COALESCE(rt.setting_sources_json, '[]')
FROM agents a
LEFT JOIN profiles p ON p.agent_id = a.id
LEFT JOIN runtimes rt ON rt.agent_id = a.id
WHERE a.status = 'active'`
	args := []any{}
	if ownerUserID != "" {
		query += ` AND a.owner_user_id = $1`
		args = append(args, ownerUserID)
	}
	query += `
ORDER BY a.is_main DESC, a.created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []protocol.Agent
	for rows.Next() {
		item, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetAgent 返回指定 Agent。
func (r *AgentRepository) GetAgent(ctx context.Context, agentID string, ownerUserID string) (*protocol.Agent, error) {
	query := `
	SELECT
	    a.id,
	    a.name,
	    a.owner_user_id,
	    a.workspace_path,
	    a.status,
	    a.is_main,
	    COALESCE(a.avatar, ''),
	    COALESCE(a.description, ''),
	    COALESCE(a.vibe_tags::text, '[]'),
	    COALESCE(p.display_name, ''),
	    COALESCE(p.headline, ''),
	    COALESCE(p.profile_markdown, ''),
	    a.created_at,
	    COALESCE(rt.provider, ''),
	    COALESCE(rt.permission_mode, ''),
	    COALESCE(rt.allowed_tools_json, '[]'),
    COALESCE(rt.disallowed_tools_json, '[]'),
    COALESCE(rt.mcp_servers_json, '{}'),
    rt.max_turns,
    rt.max_thinking_tokens,
    COALESCE(rt.setting_sources_json, '[]')
FROM agents a
LEFT JOIN profiles p ON p.agent_id = a.id
LEFT JOIN runtimes rt ON rt.agent_id = a.id
WHERE a.id = $1`
	args := []any{agentID}
	if ownerUserID != "" {
		query += ` AND a.owner_user_id = $2`
		args = append(args, ownerUserID)
	}
	row := r.db.QueryRowContext(ctx, query, args...)

	item, err := scanAgent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetMainAgent 返回指定用户的主智能体。
func (r *AgentRepository) GetMainAgent(ctx context.Context, ownerUserID string) (*protocol.Agent, error) {
	if ownerUserID == "" {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
SELECT
    a.id,
    a.name,
    a.owner_user_id,
    a.workspace_path,
    a.status,
    a.is_main,
    COALESCE(a.avatar, ''),
    COALESCE(a.description, ''),
    COALESCE(a.vibe_tags::text, '[]'),
    COALESCE(p.display_name, ''),
    COALESCE(p.headline, ''),
    COALESCE(p.profile_markdown, ''),
    a.created_at,
    COALESCE(rt.provider, ''),
    COALESCE(rt.permission_mode, ''),
    COALESCE(rt.allowed_tools_json, '[]'),
    COALESCE(rt.disallowed_tools_json, '[]'),
    COALESCE(rt.mcp_servers_json, '{}'),
    rt.max_turns,
    rt.max_thinking_tokens,
    COALESCE(rt.setting_sources_json, '[]')
FROM agents a
LEFT JOIN profiles p ON p.agent_id = a.id
LEFT JOIN runtimes rt ON rt.agent_id = a.id
WHERE a.owner_user_id = $1 AND a.status = 'active' AND a.is_main = TRUE
LIMIT 1`, ownerUserID)

	item, err := scanAgent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateAgent 创建 Agent、Profile 与 Runtime。
func (r *AgentRepository) CreateAgent(ctx context.Context, record protocol.CreateRecord) (*protocol.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO agents (
    id, owner_user_id, slug, name, description, definition, status, workspace_path, is_main, avatar, vibe_tags
) VALUES ($1, $2, $3, $4, $5, '', $6, $7, $8, $9, $10::json)`,
		record.AgentID,
		record.OwnerUserID,
		record.Slug,
		record.Name,
		record.Description,
		record.Status,
		record.WorkspacePath,
		record.IsMain,
		nullIfEmpty(record.Avatar),
		record.VibeTagsJSON,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO profiles (id, agent_id, display_name, avatar_url, headline, profile_markdown)
VALUES ($1, $2, $3, NULL, $4, $5)`,
		record.ProfileID,
		record.AgentID,
		record.DisplayName,
		record.Headline,
		record.ProfileMarkdown,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
	INSERT INTO runtimes (
	    id, agent_id, provider, permission_mode, allowed_tools_json, disallowed_tools_json,
	    mcp_servers_json, max_turns, max_thinking_tokens, setting_sources_json, runtime_version
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		record.RuntimeID,
		record.AgentID,
		nullIfEmpty(record.Provider),
		nullIfEmpty(record.PermissionMode),
		record.AllowedToolsJSON,
		record.DisallowedToolsJSON,
		record.MCPServersJSON,
		record.MaxTurns,
		record.MaxThinkingTokens,
		record.SettingSourcesJSON,
		record.RuntimeVersion,
	); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetAgent(ctx, record.AgentID, record.OwnerUserID)
}

// UpdateAgent 更新 Agent 配置。
func (r *AgentRepository) UpdateAgent(ctx context.Context, record protocol.UpdateRecord) (*protocol.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
UPDATE agents
SET slug = $1, name = $2, workspace_path = $3, avatar = $4, description = $5, vibe_tags = $6::json, updated_at = now()
WHERE id = $7 AND owner_user_id = $8`,
		record.Slug,
		record.Name,
		record.WorkspacePath,
		nullIfEmpty(record.Avatar),
		record.Description,
		record.VibeTagsJSON,
		record.AgentID,
		record.OwnerUserID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE profiles
SET display_name = $1, updated_at = now()
WHERE agent_id = $2`,
		record.Name,
		record.AgentID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
	UPDATE runtimes
	SET provider = $1, permission_mode = $2, allowed_tools_json = $3, disallowed_tools_json = $4,
	    mcp_servers_json = $5, max_turns = $6, max_thinking_tokens = $7, setting_sources_json = $8, updated_at = now()
	WHERE agent_id = $9`,
		nullIfEmpty(record.Provider),
		nullIfEmpty(record.PermissionMode),
		record.AllowedToolsJSON,
		record.DisallowedToolsJSON,
		record.MCPServersJSON,
		record.MaxTurns,
		record.MaxThinkingTokens,
		record.SettingSourcesJSON,
		record.AgentID,
	); err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetAgent(ctx, record.AgentID, record.OwnerUserID)
}

// ArchiveAgent 软删除 Agent。
func (r *AgentRepository) ArchiveAgent(ctx context.Context, agentID string, ownerUserID string) error {
	query := `
UPDATE agents
SET status = 'archived', updated_at = now()
WHERE id = $1`
	args := []any{agentID}
	if ownerUserID != "" {
		query += ` AND owner_user_id = $2`
		args = append(args, ownerUserID)
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

// ExistsActiveAgentName 检查活跃名称是否已占用。
func (r *AgentRepository) ExistsActiveAgentName(ctx context.Context, ownerUserID string, name string, excludeAgentID string) (bool, error) {
	query := `SELECT COUNT(1) FROM agents WHERE status = 'active' AND owner_user_id = $1 AND LOWER(name) = LOWER($2)`
	args := []any{ownerUserID, name}
	if excludeAgentID != "" {
		query += ` AND id <> $3`
		args = append(args, excludeAgentID)
	}

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// PromoteMainAgent 把指定 Agent 提升为主智能体。
func (r *AgentRepository) PromoteMainAgent(ctx context.Context, agentID string, ownerUserID string) (*protocol.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
UPDATE agents
SET is_main = FALSE, updated_at = now()
WHERE owner_user_id = $1`, ownerUserID); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `
UPDATE agents
SET is_main = TRUE, updated_at = now()
WHERE id = $1 AND owner_user_id = $2`, agentID, ownerUserID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetAgent(ctx, agentID, ownerUserID)
}

func scanAgent(scanner interface {
	Scan(dest ...any) error
}) (protocol.Agent, error) {
	var (
		item                protocol.Agent
		vibeTagsJSON        string
		allowedToolsJSON    string
		disallowedToolsJSON string
		mcpServersJSON      string
		settingSourcesJSON  string
		maxTurns            sql.NullInt64
		maxThinkingTokens   sql.NullInt64
		createdAt           time.Time
	)

	err := scanner.Scan(
		&item.AgentID,
		&item.Name,
		&item.OwnerUserID,
		&item.WorkspacePath,
		&item.Status,
		&item.IsMain,
		&item.Avatar,
		&item.Description,
		&vibeTagsJSON,
		&item.DisplayName,
		&item.Headline,
		&item.ProfileMarkdown,
		&createdAt,
		&item.Options.Provider,
		&item.Options.PermissionMode,
		&allowedToolsJSON,
		&disallowedToolsJSON,
		&mcpServersJSON,
		&maxTurns,
		&maxThinkingTokens,
		&settingSourcesJSON,
	)
	if err != nil {
		return protocol.Agent{}, err
	}

	item.CreatedAt = createdAt
	item.VibeTags = decodeStringSlice(vibeTagsJSON)
	item.Options.AllowedTools = protocol.ParseJSONStringSlice(allowedToolsJSON)
	item.Options.DisallowedTools = protocol.ParseJSONStringSlice(disallowedToolsJSON)
	item.Options.MCPServers = protocol.ParseJSONMap(mcpServersJSON)
	item.Options.SettingSources = protocol.ParseJSONStringSlice(settingSourcesJSON)
	if maxTurns.Valid {
		value := int(maxTurns.Int64)
		item.Options.MaxTurns = &value
	}
	if maxThinkingTokens.Valid {
		value := int(maxThinkingTokens.Int64)
		item.Options.MaxThinkingTokens = &value
	}
	return item, nil
}

func decodeStringSlice(raw string) []string {
	var result []string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil
	}
	return result
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
