package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
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
		    COALESCE(rt.model, ''),
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

	result := make([]protocol.Agent, 0)
	for rows.Next() {
		item, err := agentrepo.ScanAgent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ListAgentsByIDs 批量返回指定 ID 列表的活跃 Agent。
func (r *AgentRepository) ListAgentsByIDs(ctx context.Context, ownerUserID string, agentIDs []string) ([]protocol.Agent, error) {
	if len(agentIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(agentIDs))
	args := make([]any, len(agentIDs))
	for i, id := range agentIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
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
		    COALESCE(rt.model, ''),
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
WHERE a.status = 'active' AND a.id IN (` + strings.Join(placeholders, ", ") + `)`
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		query += fmt.Sprintf(` AND a.owner_user_id = $%d`, len(args))
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]protocol.Agent, 0, len(agentIDs))
	for rows.Next() {
		item, err := agentrepo.ScanAgent(rows)
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
		    COALESCE(rt.model, ''),
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

	item, err := agentrepo.ScanAgent(row)
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
	    COALESCE(rt.model, ''),
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

	item, err := agentrepo.ScanAgent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// CreateAgent 创建 Agent、Profile 与 Runtime。
func (r *AgentRepository) CreateAgent(ctx context.Context, record agentrepo.CreateRecord) (*protocol.Agent, error) {
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
	    id, agent_id, provider, model, permission_mode, allowed_tools_json, disallowed_tools_json,
	    mcp_servers_json, max_turns, max_thinking_tokens, setting_sources_json, runtime_version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		record.RuntimeID,
		record.AgentID,
		nullIfEmpty(record.Provider),
		nullIfEmpty(record.Model),
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
func (r *AgentRepository) UpdateAgent(ctx context.Context, record agentrepo.UpdateRecord) (*protocol.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
UPDATE agents
SET name = $1, workspace_path = $2, avatar = $3, description = $4, vibe_tags = $5::json, updated_at = now()
WHERE id = $6 AND owner_user_id = $7`,
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
		SET provider = $1, model = $2, permission_mode = $3, allowed_tools_json = $4, disallowed_tools_json = $5,
		    mcp_servers_json = $6, max_turns = $7, max_thinking_tokens = $8, setting_sources_json = $9, updated_at = now()
		WHERE agent_id = $10`,
		nullIfEmpty(record.Provider),
		nullIfEmpty(record.Model),
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

// DeleteAgent 删除 Agent 及其数据库依赖记录。
func (r *AgentRepository) DeleteAgent(ctx context.Context, agentID string, ownerUserID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = deleteAgentDependents(ctx, tx, agentID); err != nil {
		return err
	}

	query := `DELETE FROM agents WHERE id = $1`
	args := []any{agentID}
	if ownerUserID != "" {
		query += ` AND owner_user_id = $2`
		args = append(args, ownerUserID)
	}
	if _, err = tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteAgentDependents(ctx context.Context, tx *sql.Tx, agentID string) error {
	statements := []struct {
		query string
		args  []any
	}{
		{query: `
DELETE FROM automation_task_events
WHERE agent_id = $1
   OR job_id IN (SELECT job_id FROM automation_cron_jobs WHERE agent_id = $2)`, args: []any{agentID, agentID}},
		{query: `UPDATE automation_task_events SET actor_agent_id = NULL WHERE actor_agent_id = $1`, args: []any{agentID}},
		{query: `
DELETE FROM automation_cron_runs
WHERE job_id IN (SELECT job_id FROM automation_cron_jobs WHERE agent_id = $1)`, args: []any{agentID}},
		{query: `UPDATE automation_cron_jobs SET source_creator_agent_id = NULL WHERE source_creator_agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM automation_cron_jobs WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM automation_delivery_routes WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM automation_heartbeat_states WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM im_ingress_messages WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM im_pairings WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM im_channel_configs WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM contacts WHERE owner_agent_id = $1 OR contact_agent_id = $2`, args: []any{agentID, agentID}},
		{query: `DELETE FROM members WHERE member_type = 'agent' AND member_agent_id = $1`, args: []any{agentID}},
		{query: `
UPDATE rooms
SET host_agent_id = NULL,
    host_auto_reply_enabled = FALSE,
    updated_at = now()
WHERE host_agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM rounds WHERE session_id IN (SELECT id FROM sessions WHERE agent_id = $1)`, args: []any{agentID}},
		{query: `
UPDATE messages
SET session_id = NULL
WHERE session_id IN (SELECT id FROM sessions WHERE agent_id = $1)`, args: []any{agentID}},
		{query: `UPDATE messages SET sender_agent_id = NULL WHERE sender_agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM sessions WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM profiles WHERE agent_id = $1`, args: []any{agentID}},
		{query: `DELETE FROM runtimes WHERE agent_id = $1`, args: []any{agentID}},
	}

	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
