package automation

import (
	"context"
	"database/sql"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// GetHeartbeatState 读取 heartbeat 配置。
func (r *Repository) GetHeartbeatState(ctx context.Context, agentID string) (*automationdomain.HeartbeatConfig, *time.Time, *time.Time, error) {
	query := `
SELECT
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars,
    configuration_version,
    last_heartbeat_at,
    last_ack_at
FROM automation_heartbeat_states
WHERE agent_id = ` + r.bind(1)
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(agentID))
	var (
		item          automationdomain.HeartbeatConfig
		lastHeartbeat sql.NullTime
		lastAck       sql.NullTime
	)
	err := row.Scan(
		&item.AgentID,
		&item.Enabled,
		&item.EverySeconds,
		&item.TargetMode,
		&item.AckMaxChars,
		&item.ConfigurationVersion,
		&lastHeartbeat,
		&lastAck,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, err
	}
	return &item, nullTimePointer(lastHeartbeat), nullTimePointer(lastAck), nil
}

// ListEnabledHeartbeatStates 列出已启用 heartbeat。
func (r *Repository) ListEnabledHeartbeatStates(ctx context.Context) ([]automationdomain.HeartbeatConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars,
    configuration_version
FROM automation_heartbeat_states
WHERE enabled = TRUE
ORDER BY agent_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]automationdomain.HeartbeatConfig, 0)
	for rows.Next() {
		var item automationdomain.HeartbeatConfig
		if scanErr := rows.Scan(
			&item.AgentID,
			&item.Enabled,
			&item.EverySeconds,
			&item.TargetMode,
			&item.AckMaxChars,
			&item.ConfigurationVersion,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpsertHeartbeatState 创建或更新 heartbeat 配置。
func (r *Repository) UpsertHeartbeatState(ctx context.Context, stateID string, config automationdomain.HeartbeatConfig, lastHeartbeatAt *time.Time, lastAckAt *time.Time) error {
	_, err := r.execWithRetry(
		ctx,
		r.upsertHeartbeatStateQuery,
		stateID,
		config.AgentID,
		config.Enabled,
		config.EverySeconds,
		config.TargetMode,
		config.AckMaxChars,
		lastHeartbeatAt,
		lastAckAt,
	)
	return err
}

// UpsertHeartbeatStateAtVersion 以持久版本创建或更新 heartbeat 配置。
// expectedVersion=0 只允许首次插入；已有配置必须精确匹配并推进一版。
func (r *Repository) UpsertHeartbeatStateAtVersion(
	ctx context.Context,
	stateID string,
	config automationdomain.HeartbeatConfig,
	lastHeartbeatAt *time.Time,
	lastAckAt *time.Time,
	expectedVersion int64,
) error {
	if expectedVersion < 0 {
		return automationdomain.ErrConfigurationVersionConflict
	}
	var (
		result sql.Result
		err    error
	)
	if expectedVersion == 0 {
		result, err = r.execWithRetry(
			ctx,
			`INSERT INTO automation_heartbeat_states (
    state_id,
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars,
    last_heartbeat_at,
    last_ack_at,
    created_at,
    updated_at
) VALUES (`+r.bindList(8)+`,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT(agent_id) DO NOTHING`,
			stateID,
			config.AgentID,
			config.Enabled,
			config.EverySeconds,
			config.TargetMode,
			config.AckMaxChars,
			lastHeartbeatAt,
			lastAckAt,
		)
	} else {
		result, err = r.execWithRetry(
			ctx,
			`UPDATE automation_heartbeat_states
SET enabled = `+r.bind(1)+`,
    every_seconds = `+r.bind(2)+`,
    target_mode = `+r.bind(3)+`,
    ack_max_chars = `+r.bind(4)+`,
    configuration_version = configuration_version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE agent_id = `+r.bind(5)+`
  AND configuration_version = `+r.bind(6),
			config.Enabled,
			config.EverySeconds,
			config.TargetMode,
			config.AckMaxChars,
			config.AgentID,
			expectedVersion,
		)
	}
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return automationdomain.ErrConfigurationVersionConflict
	}
	return nil
}

// PersistHeartbeatRuntimeState 只刷新运行时间，不推进或覆盖配置版本。
func (r *Repository) PersistHeartbeatRuntimeState(
	ctx context.Context,
	stateID string,
	config automationdomain.HeartbeatConfig,
	lastHeartbeatAt *time.Time,
	lastAckAt *time.Time,
) error {
	_, err := r.execWithRetry(
		ctx,
		r.persistHeartbeatRuntimeQuery,
		stateID,
		config.AgentID,
		config.Enabled,
		config.EverySeconds,
		config.TargetMode,
		config.AckMaxChars,
		lastHeartbeatAt,
		lastAckAt,
	)
	return err
}
