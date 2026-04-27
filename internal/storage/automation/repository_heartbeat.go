package automation

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetHeartbeatState 读取 heartbeat 配置。
func (r *Repository) GetHeartbeatState(ctx context.Context, agentID string) (*protocol.HeartbeatConfig, *time.Time, *time.Time, error) {
	query := `
SELECT
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars,
    last_heartbeat_at,
    last_ack_at
FROM automation_heartbeat_states
WHERE agent_id = ` + r.bind(1)
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(agentID))
	var (
		item          protocol.HeartbeatConfig
		lastHeartbeat sql.NullTime
		lastAck       sql.NullTime
	)
	err := row.Scan(
		&item.AgentID,
		&item.Enabled,
		&item.EverySeconds,
		&item.TargetMode,
		&item.AckMaxChars,
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
func (r *Repository) ListEnabledHeartbeatStates(ctx context.Context) ([]protocol.HeartbeatConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    agent_id,
    enabled,
    every_seconds,
    target_mode,
    ack_max_chars
FROM automation_heartbeat_states
WHERE enabled = TRUE
ORDER BY agent_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]protocol.HeartbeatConfig, 0)
	for rows.Next() {
		var item protocol.HeartbeatConfig
		if scanErr := rows.Scan(
			&item.AgentID,
			&item.Enabled,
			&item.EverySeconds,
			&item.TargetMode,
			&item.AckMaxChars,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpsertHeartbeatState 创建或更新 heartbeat 配置。
func (r *Repository) UpsertHeartbeatState(ctx context.Context, stateID string, config protocol.HeartbeatConfig, lastHeartbeatAt *time.Time, lastAckAt *time.Time) error {
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
