// INPUT: owner、Room 与可信 Agent 标识。
// OUTPUT: 单条 SQL 快照中的 host、成员关系、configuration_version 与 authority_epoch。
// POS: Room 配置权限判定的持久化事实入口；禁止用分步聚合查询替代。
package roomrepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetRoomAuthorizationSnapshot 在一个数据库语句快照内读取 Room 授权事实。
func (r *SQLRepository) GetRoomAuthorizationSnapshot(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	agentID string,
) (*protocol.RoomAuthorizationSnapshot, error) {
	query := `
SELECT
    r.id,
    COALESCE(r.host_agent_id, ''),
    EXISTS (
        SELECT 1
        FROM members m
        WHERE m.room_id = r.id
          AND m.member_type = 'agent'
          AND m.member_agent_id = ` + r.dialect.Bind(1) + `
    ),
    r.configuration_version,
    r.authority_epoch
FROM rooms r
WHERE r.id = ` + r.dialect.Bind(2)
	args := []any{agentID, roomID}
	if ownerUserID != "" {
		query += ` AND r.owner_user_id = ` + r.dialect.Bind(3)
		args = append(args, ownerUserID)
	}

	snapshot := protocol.RoomAuthorizationSnapshot{
		AgentID: agentID,
	}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&snapshot.RoomID,
		&snapshot.HostAgentID,
		&snapshot.AgentIsMember,
		&snapshot.ConfigurationVersion,
		&snapshot.AuthorityEpoch,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}
