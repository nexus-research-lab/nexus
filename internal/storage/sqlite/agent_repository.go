// =====================================================
// @File   ：agent_repository.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	agentmodel "github.com/nexus-research-lab/nexus/internal/model/agent"
)

// AgentRepository 提供 SQLite 的 Agent 仓储实现。
type AgentRepository struct {
	db *sql.DB
}

// NewAgentRepository 创建 Agent 仓储。
func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

// ListActiveAgents 返回所有活跃 Agent。
func (r *AgentRepository) ListActiveAgents(ctx context.Context) ([]agentmodel.Agent, error) {
	rows, err := r.db.QueryContext(ctx, `
	SELECT
	    a.id,
	    a.name,
    a.workspace_path,
    a.status,
    COALESCE(a.avatar, ''),
    COALESCE(a.description, ''),
	    COALESCE(a.vibe_tags, '[]'),
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
WHERE a.status = 'active'
ORDER BY a.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []agentmodel.Agent
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
func (r *AgentRepository) GetAgent(ctx context.Context, agentID string) (*agentmodel.Agent, error) {
	row := r.db.QueryRowContext(ctx, `
	SELECT
	    a.id,
	    a.name,
    a.workspace_path,
    a.status,
    COALESCE(a.avatar, ''),
    COALESCE(a.description, ''),
	    COALESCE(a.vibe_tags, '[]'),
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
WHERE a.id = ?`, agentID)

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
func (r *AgentRepository) CreateAgent(ctx context.Context, record agentmodel.CreateRecord) (*agentmodel.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO agents (id, slug, name, description, definition, status, workspace_path, avatar, vibe_tags)
VALUES (?, ?, ?, ?, '', ?, ?, ?, json(?))`,
		record.AgentID,
		record.Slug,
		record.Name,
		record.Description,
		record.Status,
		record.WorkspacePath,
		nullIfEmpty(record.Avatar),
		record.VibeTagsJSON,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO profiles (id, agent_id, display_name, avatar_url, headline, profile_markdown)
VALUES (?, ?, ?, NULL, ?, ?)`,
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
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
	return r.GetAgent(ctx, record.AgentID)
}

// UpdateAgent 更新 Agent 配置。
func (r *AgentRepository) UpdateAgent(ctx context.Context, record agentmodel.UpdateRecord) (*agentmodel.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
UPDATE agents
SET slug = ?, name = ?, workspace_path = ?, avatar = ?, description = ?, vibe_tags = json(?), updated_at = CURRENT_TIMESTAMP
WHERE id = ?`,
		record.Slug,
		record.Name,
		record.WorkspacePath,
		nullIfEmpty(record.Avatar),
		record.Description,
		record.VibeTagsJSON,
		record.AgentID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE profiles
SET display_name = ?, updated_at = CURRENT_TIMESTAMP
WHERE agent_id = ?`,
		record.Name,
		record.AgentID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
	UPDATE runtimes
	SET provider = ?, permission_mode = ?, allowed_tools_json = ?, disallowed_tools_json = ?,
	    mcp_servers_json = ?, max_turns = ?, max_thinking_tokens = ?, setting_sources_json = ?, updated_at = CURRENT_TIMESTAMP
	WHERE agent_id = ?`,
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
	return r.GetAgent(ctx, record.AgentID)
}

// ArchiveAgent 软删除 Agent。
func (r *AgentRepository) ArchiveAgent(ctx context.Context, agentID string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE agents
SET status = 'archived', updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, agentID)
	return err
}

// ExistsActiveAgentName 检查活跃名称是否已占用。
func (r *AgentRepository) ExistsActiveAgentName(ctx context.Context, name string, excludeAgentID string) (bool, error) {
	query := `SELECT COUNT(1) FROM agents WHERE status = 'active' AND LOWER(name) = LOWER(?)`
	args := []any{name}
	if excludeAgentID != "" {
		query += ` AND id <> ?`
		args = append(args, excludeAgentID)
	}

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func scanAgent(scanner interface {
	Scan(dest ...any) error
}) (agentmodel.Agent, error) {
	var (
		item                agentmodel.Agent
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
		&item.WorkspacePath,
		&item.Status,
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
		return agentmodel.Agent{}, err
	}

	item.CreatedAt = createdAt
	item.VibeTags = decodeStringSlice(vibeTagsJSON)
	item.Options.AllowedTools = agentmodel.ParseJSONStringSlice(allowedToolsJSON)
	item.Options.DisallowedTools = agentmodel.ParseJSONStringSlice(disallowedToolsJSON)
	item.Options.MCPServers = agentmodel.ParseJSONMap(mcpServersJSON)
	item.Options.SettingSources = agentmodel.ParseJSONStringSlice(settingSourcesJSON)
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
