// INPUT: 当前 owner 上下文、Room ID 与可信 Agent ID。
// OUTPUT: 同一数据库语句内形成的 Room 授权快照。
// POS: configuration 等调用方判定 Room host/member 权限的唯一服务入口。
package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetRoomAuthorizationSnapshot 读取当前 owner 下指定 Agent 的 Room 授权事实。
func (s *Service) GetRoomAuthorizationSnapshot(
	ctx context.Context,
	roomID string,
	agentID string,
) (*protocol.RoomAuthorizationSnapshot, error) {
	roomID = strings.TrimSpace(roomID)
	agentID = strings.TrimSpace(agentID)
	if roomID == "" {
		return nil, errors.New("Room 授权快照缺少 room_id")
	}
	if agentID == "" {
		return nil, errors.New("Room 授权快照缺少 agent_id")
	}
	snapshot, err := s.repository.GetRoomAuthorizationSnapshot(
		ctx,
		authctx.OwnerUserID(ctx),
		roomID,
		agentID,
	)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, ErrRoomNotFound
	}
	return snapshot, nil
}
