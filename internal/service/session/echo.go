// INPUT: WebSocket DM session key 与 Echo 覆盖模式。
// OUTPUT: owner-scoped workspace Session options 更新结果。
// POS: 用户级 Echo 策略之下的 Session 覆盖持久化入口。
package session

import (
	"context"
	"strings"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetEchoOverride 读取单个 DM 的 Echo 覆盖。
func (s *Service) GetEchoOverride(ctx context.Context, rawSessionKey string) (echodomain.SessionOverride, error) {
	item, _, _, err := s.loadMutableWorkspaceSession(ctx, rawSessionKey)
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	mode, err := echodomain.NormalizeSessionMode(protocol.SessionEchoModeFromOptions(item.Options))
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	return echodomain.SessionOverride{Mode: mode}, nil
}

// UpdateEchoOverride 更新单个 WebSocket DM 的 Echo 覆盖。
func (s *Service) UpdateEchoOverride(
	ctx context.Context,
	rawSessionKey string,
	override echodomain.SessionOverride,
) (echodomain.SessionOverride, error) {
	mode, err := echodomain.NormalizeSessionMode(override.Mode)
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	item, workspacePath, parsed, err := s.loadMutableWorkspaceSession(ctx, rawSessionKey)
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	if protocol.NormalizeSessionKeyChannelSegment(parsed.Channel) != protocol.SessionChannelWebSocketSegment ||
		strings.TrimSpace(parsed.ChatType) != protocol.RoomTypeDM {
		return echodomain.SessionOverride{}, echodomain.ErrUnsupportedSession
	}
	item.Options = protocol.WithSessionEchoMode(item.Options, mode)
	updated, err := s.ownerFiles(ctx).UpsertSession(workspacePath, *item)
	if err != nil {
		return echodomain.SessionOverride{}, err
	}
	s.notifyDirectoryChanged(ctx, "session_echo_updated", *updated)
	return echodomain.SessionOverride{Mode: mode}, nil
}
