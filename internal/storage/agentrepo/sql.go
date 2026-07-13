package agentrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// SQLRepository 提供 Agent 仓储的跨方言 SQL 实现。
type SQLRepository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

// NewSQLRepository 创建 Agent 仓储。
func NewSQLRepository(driver string, db *sql.DB) *SQLRepository {
	return &SQLRepository{
		db:      db,
		dialect: storage.NewSQLDialect(driver),
	}
}

// ListActiveAgents 返回所有活跃 Agent。
func (r *SQLRepository) ListActiveAgents(ctx context.Context, ownerUserID string) ([]protocol.Agent, error) {
	query := r.agentSelect() + `
WHERE a.status = 'active'`
	args := []any{}
	if ownerUserID != "" {
		query += ` AND a.owner_user_id = ` + r.dialect.Bind(1)
		args = append(args, ownerUserID)
	}
	query += `
ORDER BY a.is_main DESC, a.created_at ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAgents(rows, 0)
}

// ListAgentsByIDs 批量返回指定 ID 列表的活跃 Agent。
func (r *SQLRepository) ListAgentsByIDs(ctx context.Context, ownerUserID string, agentIDs []string) ([]protocol.Agent, error) {
	if len(agentIDs) == 0 {
		return nil, nil
	}
	query := r.agentSelect() + `
WHERE a.status = 'active' AND a.id IN (` + r.dialect.BindList(len(agentIDs)) + `)`
	args := make([]any, 0, len(agentIDs)+1)
	for _, id := range agentIDs {
		args = append(args, id)
	}
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		query += ` AND a.owner_user_id = ` + r.dialect.Bind(len(args))
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAgents(rows, len(agentIDs))
}

// GetAgent 返回指定 Agent。
func (r *SQLRepository) GetAgent(ctx context.Context, agentID string, ownerUserID string) (*protocol.Agent, error) {
	query := r.agentSelect() + `
WHERE a.id = ` + r.dialect.Bind(1)
	args := []any{agentID}
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		query += ` AND a.owner_user_id = ` + r.dialect.Bind(2)
	}
	return r.getAgent(ctx, query, args...)
}

// GetMainAgent 返回指定用户的主智能体。
func (r *SQLRepository) GetMainAgent(ctx context.Context, ownerUserID string) (*protocol.Agent, error) {
	if ownerUserID == "" {
		return nil, nil
	}
	return r.getAgent(ctx, r.agentSelect()+`
WHERE a.owner_user_id = `+r.dialect.Bind(1)+` AND a.status = 'active' AND a.is_main = `+r.dialect.TrueValue()+`
LIMIT 1`, ownerUserID)
}

// CreateAgent 创建 Agent、Profile 与 Runtime。
func (r *SQLRepository) CreateAgent(ctx context.Context, record CreateRecord) (*protocol.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO agents (
    id, owner_user_id, slug, name, description, definition, status, workspace_path, is_main, avatar, vibe_tags
) VALUES (%s, %s, %s, %s, %s, '', %s, %s, %s, %s, %s)`,
		r.dialect.Bind(1),
		r.dialect.Bind(2),
		r.dialect.Bind(3),
		r.dialect.Bind(4),
		r.dialect.Bind(5),
		r.dialect.Bind(6),
		r.dialect.Bind(7),
		r.dialect.Bind(8),
		r.dialect.Bind(9),
		r.dialect.JSONValue(10),
	),
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
VALUES (`+r.dialect.BindList(3)+`, NULL, `+r.dialect.Bind(4)+`, `+r.dialect.Bind(5)+`)`,
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
) VALUES (`+r.dialect.BindList(12)+`)`,
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
func (r *SQLRepository) UpdateAgent(ctx context.Context, record UpdateRecord) (*protocol.Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE agents
SET name = %s, workspace_path = %s, avatar = %s, description = %s, vibe_tags = %s, updated_at = %s
WHERE id = %s AND owner_user_id = %s`,
		r.dialect.Bind(1),
		r.dialect.Bind(2),
		r.dialect.Bind(3),
		r.dialect.Bind(4),
		r.dialect.JSONValue(5),
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(6),
		r.dialect.Bind(7),
	),
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
SET display_name = `+r.dialect.Bind(1)+`, updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE agent_id = `+r.dialect.Bind(2),
		record.Name,
		record.AgentID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE runtimes
SET provider = %s, model = %s, permission_mode = %s, allowed_tools_json = %s, disallowed_tools_json = %s,
    mcp_servers_json = %s, max_turns = %s, max_thinking_tokens = %s, setting_sources_json = %s, updated_at = %s
WHERE agent_id = %s`,
		r.dialect.Bind(1),
		r.dialect.Bind(2),
		r.dialect.Bind(3),
		r.dialect.Bind(4),
		r.dialect.Bind(5),
		r.dialect.Bind(6),
		r.dialect.Bind(7),
		r.dialect.Bind(8),
		r.dialect.Bind(9),
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(10),
	),
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
func (r *SQLRepository) DeleteAgent(ctx context.Context, agentID string, ownerUserID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err = r.deleteAgentDependents(ctx, tx, agentID); err != nil {
		return err
	}

	query := `DELETE FROM agents WHERE id = ` + r.dialect.Bind(1)
	args := []any{agentID}
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		query += ` AND owner_user_id = ` + r.dialect.Bind(2)
	}
	if _, err = tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLRepository) getAgent(ctx context.Context, query string, args ...any) (*protocol.Agent, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	item, err := ScanAgent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *SQLRepository) agentSelect() string {
	return fmt.Sprintf(`
SELECT
    a.id,
    a.name,
    a.owner_user_id,
    a.workspace_path,
    a.status,
    a.is_main,
    COALESCE(a.avatar, ''),
    COALESCE(a.description, ''),
    COALESCE(%s, '[]'),
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
LEFT JOIN runtimes rt ON rt.agent_id = a.id`, r.dialect.JSONText("a.vibe_tags"))
}

func scanAgents(rows *sql.Rows, capacity int) ([]protocol.Agent, error) {
	result := make([]protocol.Agent, 0, capacity)
	for rows.Next() {
		item, err := ScanAgent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLRepository) deleteAgentDependents(ctx context.Context, tx *sql.Tx, agentID string) error {
	statements := []struct {
		query string
		args  []any
	}{
		{query: `
DELETE FROM automation_task_events
WHERE agent_id = ` + r.dialect.Bind(1) + `
   OR job_id IN (SELECT job_id FROM automation_scheduled_tasks WHERE agent_id = ` + r.dialect.Bind(2) + `)`, args: []any{agentID, agentID}},
		{query: `UPDATE automation_task_events SET actor_agent_id = NULL WHERE actor_agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `
DELETE FROM automation_task_runs
WHERE job_id IN (SELECT job_id FROM automation_scheduled_tasks WHERE agent_id = ` + r.dialect.Bind(1) + `)`, args: []any{agentID}},
		{query: `UPDATE automation_scheduled_tasks SET source_creator_agent_id = NULL WHERE source_creator_agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM automation_scheduled_tasks WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM automation_delivery_routes WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM automation_heartbeat_states WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM im_ingress_messages WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM im_pairings WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM im_channel_configs WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM contacts WHERE owner_agent_id = ` + r.dialect.Bind(1) + ` OR contact_agent_id = ` + r.dialect.Bind(2), args: []any{agentID, agentID}},
		{query: `DELETE FROM members WHERE member_type = 'agent' AND member_agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `
UPDATE rooms
SET host_agent_id = NULL,
    host_auto_reply_enabled = ` + r.dialect.FalseValue() + `,
    updated_at = ` + r.dialect.CurrentTimestamp() + `
WHERE host_agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM rounds WHERE session_id IN (SELECT id FROM sessions WHERE agent_id = ` + r.dialect.Bind(1) + `)`, args: []any{agentID}},
		{query: `
UPDATE messages
SET session_id = NULL
WHERE session_id IN (SELECT id FROM sessions WHERE agent_id = ` + r.dialect.Bind(1) + `)`, args: []any{agentID}},
		{query: `UPDATE messages SET sender_agent_id = NULL WHERE sender_agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM sessions WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM profiles WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
		{query: `DELETE FROM runtimes WHERE agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
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
