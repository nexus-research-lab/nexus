// INPUT: owner scope、Agent 仓储记录、可选期望 runtime 版本与 SQL 方言。
// OUTPUT: owner 隔离 CRUD、runtime_version CAS，以及含 Channel version/account/pairing 的原子删除。
// POS: Agent 持久化的跨方言事务边界；级联顺序在提交前保持可核验。
package agentrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// ErrRuntimeVersionConflict 表示 Agent runtime 已被其他写入更新。
var ErrRuntimeVersionConflict = errors.New("agent runtime version conflict")

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

// ListActiveAgents 返回主智能体优先、其余按新建优先排列的活跃 Agent。
func (r *SQLRepository) ListActiveAgents(ctx context.Context, ownerUserID string) ([]protocol.Agent, error) {
	query := r.agentSelect() + `
WHERE a.status = 'active'`
	args := []any{}
	if ownerUserID != "" {
		query += ` AND a.owner_user_id = ` + r.dialect.Bind(1)
		args = append(args, ownerUserID)
	}
	// ponytail: SQLite 创建时间精确到秒；同秒批量创建成为真实需求时再引入持久化序号。
	query += `
ORDER BY a.is_main DESC, a.created_at DESC`
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
    id, owner_user_id, slug, name, description, definition, status, workspace_path, is_main, avatar, vibe_tags, business_tags
) VALUES (%s, %s, %s, %s, %s, '', %s, %s, %s, %s, %s, %s)`,
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
		r.dialect.JSONValue(11),
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
		record.BusinessTagsJSON,
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
    mcp_servers_json, connector_ids_json, skill_ids_json, disabled_skill_ids_json,
    max_turns, max_thinking_tokens, setting_sources_json, runtime_version
) VALUES (`+r.dialect.BindList(15)+`)`,
		record.RuntimeID,
		record.AgentID,
		nullIfEmpty(record.Provider),
		nullIfEmpty(record.Model),
		nullIfEmpty(record.PermissionMode),
		record.AllowedToolsJSON,
		record.DisallowedToolsJSON,
		record.MCPServersJSON,
		record.ConnectorIDsJSON,
		record.SkillIDsJSON,
		record.DisabledSkillIDsJSON,
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

	agentResult, err := tx.ExecContext(ctx, fmt.Sprintf(`
UPDATE agents
SET name = %s, workspace_path = %s, avatar = %s, description = %s, vibe_tags = %s, business_tags = %s, updated_at = %s
WHERE id = %s AND owner_user_id = %s`,
		r.dialect.Bind(1),
		r.dialect.Bind(2),
		r.dialect.Bind(3),
		r.dialect.Bind(4),
		r.dialect.JSONValue(5),
		r.dialect.JSONValue(6),
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(7),
		r.dialect.Bind(8),
	),
		record.Name,
		record.WorkspacePath,
		nullIfEmpty(record.Avatar),
		record.Description,
		record.VibeTagsJSON,
		record.BusinessTagsJSON,
		record.AgentID,
		record.OwnerUserID,
	)
	if err != nil {
		return nil, err
	}
	affected, err := agentResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, nil
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

	runtimeQuery := fmt.Sprintf(`
UPDATE runtimes
SET provider = %s, model = %s, permission_mode = %s, allowed_tools_json = %s, disallowed_tools_json = %s,
    mcp_servers_json = %s, connector_ids_json = %s, skill_ids_json = %s, disabled_skill_ids_json = %s,
    max_turns = %s, max_thinking_tokens = %s, setting_sources_json = %s,
    runtime_version = runtime_version + 1, updated_at = %s
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
		r.dialect.Bind(10),
		r.dialect.Bind(11),
		r.dialect.Bind(12),
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(13),
	)
	runtimeArgs := []any{
		nullIfEmpty(record.Provider),
		nullIfEmpty(record.Model),
		nullIfEmpty(record.PermissionMode),
		record.AllowedToolsJSON,
		record.DisallowedToolsJSON,
		record.MCPServersJSON,
		record.ConnectorIDsJSON,
		record.SkillIDsJSON,
		record.DisabledSkillIDsJSON,
		record.MaxTurns,
		record.MaxThinkingTokens,
		record.SettingSourcesJSON,
		record.AgentID,
	}
	if record.ExpectedRuntimeVersion != nil {
		runtimeQuery += ` AND runtime_version = ` + r.dialect.Bind(14)
		runtimeArgs = append(runtimeArgs, *record.ExpectedRuntimeVersion)
	}
	runtimeResult, err := tx.ExecContext(ctx, runtimeQuery, runtimeArgs...)
	if err != nil {
		return nil, err
	}
	affected, err = runtimeResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		if record.ExpectedRuntimeVersion != nil {
			return nil, ErrRuntimeVersionConflict
		}
		return nil, sql.ErrNoRows
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetAgent(ctx, record.AgentID, record.OwnerUserID)
}

// UpdateAgentSkillSelection 只更新技能绑定列，避免覆盖并发保存的其它 Agent 配置。
func (r *SQLRepository) UpdateAgentSkillSelection(
	ctx context.Context,
	agentID string,
	ownerUserID string,
	skillIDsJSON string,
	disabledSkillIDsJSON string,
) (*protocol.Agent, error) {
	query := fmt.Sprintf(`
UPDATE runtimes
SET skill_ids_json = %s,
    disabled_skill_ids_json = %s,
    runtime_version = runtime_version + 1,
    updated_at = %s
WHERE agent_id = %s`,
		r.dialect.Bind(1),
		r.dialect.Bind(2),
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(3),
	)
	args := []any{skillIDsJSON, disabledSkillIDsJSON, agentID}
	if strings.TrimSpace(ownerUserID) != "" {
		query += ` AND agent_id IN (SELECT id FROM agents WHERE owner_user_id = ` + r.dialect.Bind(4) + `)`
		args = append(args, ownerUserID)
	}
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, nil
	}
	return r.GetAgent(ctx, agentID, ownerUserID)
}

// UpdateAgentSkillIDsAtVersion 仅在 runtime_version 匹配时更新全局 Skill 绑定列。
func (r *SQLRepository) UpdateAgentSkillIDsAtVersion(
	ctx context.Context,
	agentID string,
	ownerUserID string,
	skillIDsJSON string,
	expectedRuntimeVersion int64,
) (*protocol.Agent, error) {
	return r.updateAgentSkillColumnAtVersion(
		ctx,
		agentID,
		ownerUserID,
		"skill_ids_json",
		skillIDsJSON,
		expectedRuntimeVersion,
	)
}

// UpdateAgentDisabledSkillIDsAtVersion 仅在版本匹配时更新 workspace Skill 停用列。
func (r *SQLRepository) UpdateAgentDisabledSkillIDsAtVersion(
	ctx context.Context,
	agentID string,
	ownerUserID string,
	disabledSkillIDsJSON string,
	expectedRuntimeVersion int64,
) (*protocol.Agent, error) {
	return r.updateAgentSkillColumnAtVersion(
		ctx,
		agentID,
		ownerUserID,
		"disabled_skill_ids_json",
		disabledSkillIDsJSON,
		expectedRuntimeVersion,
	)
}

func (r *SQLRepository) updateAgentSkillColumnAtVersion(
	ctx context.Context,
	agentID string,
	ownerUserID string,
	column string,
	valueJSON string,
	expectedRuntimeVersion int64,
) (*protocol.Agent, error) {
	query := fmt.Sprintf(`
UPDATE runtimes
SET %s = %s,
    runtime_version = runtime_version + 1,
    updated_at = %s
WHERE agent_id = %s`,
		column,
		r.dialect.Bind(1),
		r.dialect.CurrentTimestamp(),
		r.dialect.Bind(2),
	)
	args := []any{valueJSON, agentID}
	if strings.TrimSpace(ownerUserID) != "" {
		query += ` AND agent_id IN (SELECT id FROM agents WHERE owner_user_id = ` + r.dialect.Bind(3) + `)`
		args = append(args, ownerUserID)
	}
	query += ` AND runtime_version = ` + r.dialect.Bind(len(args)+1)
	args = append(args, expectedRuntimeVersion)
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrRuntimeVersionConflict
	}
	return r.GetAgent(ctx, agentID, ownerUserID)
}

// DeleteAgent 删除 Agent 及其数据库依赖记录。
func (r *SQLRepository) DeleteAgent(ctx context.Context, agentID string, ownerUserID string) error {
	return r.deleteAgent(ctx, agentID, ownerUserID, nil)
}

// DeleteAgentAtVersion 仅在 runtime_version 仍等于计划版本时删除 Agent。
func (r *SQLRepository) DeleteAgentAtVersion(
	ctx context.Context,
	agentID string,
	ownerUserID string,
	expectedRuntimeVersion int64,
) error {
	return r.deleteAgent(ctx, agentID, ownerUserID, &expectedRuntimeVersion)
}

func (r *SQLRepository) deleteAgent(
	ctx context.Context,
	agentID string,
	ownerUserID string,
	expectedRuntimeVersion *int64,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if expectedRuntimeVersion != nil {
		query := `
UPDATE runtimes
SET runtime_version = runtime_version
WHERE agent_id = ` + r.dialect.Bind(1) + `
  AND runtime_version = ` + r.dialect.Bind(2)
		args := []any{agentID, *expectedRuntimeVersion}
		if ownerUserID != "" {
			query += `
  AND agent_id IN (
      SELECT id
      FROM agents
      WHERE owner_user_id = ` + r.dialect.Bind(3) + `
  )`
			args = append(args, ownerUserID)
		}
		result, lockErr := tx.ExecContext(ctx, query, args...)
		if lockErr != nil {
			return lockErr
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if affected != 1 {
			return ErrRuntimeVersionConflict
		}
	}
	if err = r.advanceChannelControlVersionForAgentDeletion(ctx, tx, ownerUserID); err != nil {
		return err
	}
	if err = r.deleteAgentDependents(ctx, tx, agentID, ownerUserID); err != nil {
		return err
	}

	query := `DELETE FROM agents WHERE id = ` + r.dialect.Bind(1)
	args := []any{agentID}
	if ownerUserID != "" {
		args = append(args, ownerUserID)
		query += ` AND owner_user_id = ` + r.dialect.Bind(2)
	}
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return sql.ErrNoRows
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
    COALESCE(rt.connector_ids_json, '[]'),
    COALESCE(rt.skill_ids_json, '[]'),
    COALESCE(rt.disabled_skill_ids_json, '[]'),
    rt.max_turns,
    rt.max_thinking_tokens,
    COALESCE(rt.setting_sources_json, '[]'),
    COALESCE(rt.runtime_version, 0)
FROM agents a
LEFT JOIN profiles p ON p.agent_id = a.id
LEFT JOIN runtimes rt ON rt.agent_id = a.id`,
		r.dialect.JSONText("a.vibe_tags"),
		r.dialect.JSONText("a.business_tags"),
	)
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

func (r *SQLRepository) advanceChannelControlVersionForAgentDeletion(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
) error {
	if ownerUserID == "" {
		return nil
	}
	insertQuery := `
INSERT INTO channel_control_versions (owner_user_id, version, updated_at)
VALUES (` + r.dialect.Bind(1) + `, 1, ` + r.dialect.CurrentTimestamp() + `)
ON CONFLICT (owner_user_id) DO NOTHING`
	if _, err := tx.ExecContext(ctx, insertQuery, ownerUserID); err != nil {
		return err
	}
	updateQuery := `
UPDATE channel_control_versions
SET version = version + 1, updated_at = ` + r.dialect.CurrentTimestamp() + `
WHERE owner_user_id = ` + r.dialect.Bind(1)
	_, err := tx.ExecContext(ctx, updateQuery, ownerUserID)
	return err
}

func (r *SQLRepository) deleteAgentDependents(
	ctx context.Context,
	tx *sql.Tx,
	agentID string,
	ownerUserID string,
) error {
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
		{query: `
DELETE FROM im_pairings
WHERE agent_id = ` + r.dialect.Bind(1) + `
  AND owner_user_id = ` + r.dialect.Bind(2), args: []any{agentID, ownerUserID}},
		{query: `
DELETE FROM im_channel_accounts
WHERE owner_user_id = ` + r.dialect.Bind(1) + `
  AND EXISTS (
      SELECT 1
      FROM im_channel_configs
      WHERE im_channel_configs.owner_user_id = im_channel_accounts.owner_user_id
        AND im_channel_configs.channel_type = im_channel_accounts.channel_type
        AND im_channel_configs.agent_id = ` + r.dialect.Bind(2) + `
  )`, args: []any{ownerUserID, agentID}},
		{query: `
DELETE FROM im_channel_configs
WHERE agent_id = ` + r.dialect.Bind(1) + `
  AND owner_user_id = ` + r.dialect.Bind(2), args: []any{agentID, ownerUserID}},
		{query: `DELETE FROM contacts WHERE owner_agent_id = ` + r.dialect.Bind(1) + ` OR contact_agent_id = ` + r.dialect.Bind(2), args: []any{agentID, agentID}},
		{query: `
UPDATE rooms
SET host_agent_id = CASE
        WHEN host_agent_id = ` + r.dialect.Bind(1) + ` THEN NULL
        ELSE host_agent_id
    END,
    host_auto_reply_enabled = CASE
        WHEN host_agent_id = ` + r.dialect.Bind(2) + ` THEN ` + r.dialect.FalseValue() + `
        ELSE host_auto_reply_enabled
    END,
    configuration_version = configuration_version + 1,
    authority_epoch = authority_epoch + 1,
    updated_at = ` + r.dialect.CurrentTimestamp() + `
WHERE host_agent_id = ` + r.dialect.Bind(3) + `
   OR id IN (
       SELECT room_id
       FROM members
       WHERE member_type = 'agent' AND member_agent_id = ` + r.dialect.Bind(4) + `
   )`, args: []any{agentID, agentID, agentID, agentID}},
		{query: `DELETE FROM members WHERE member_type = 'agent' AND member_agent_id = ` + r.dialect.Bind(1), args: []any{agentID}},
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
