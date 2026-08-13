// INPUT: owner-scoped current Goal、持久 Room lead 或 DM Agent identity。
// OUTPUT: 允许负责人新 round 使用的精确 Goal/objective revision 快照。
// POS: Goal MCP 跨 round 负责人权限的持久身份解析边界；不授予 Execution authority。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CurrentModelMutationAuthority 返回当前 Agent 负责的 current Goal。
//
// 返回值只是新物理 round 启动时使用的精确 revision 快照：调用方必须把它
// 固定在该 round，不能在工具调用时重新读取最新 revision。Room 只认持久化
// lead；DM 只认 session key 自带的 Agent identity。
func (s *Service) CurrentModelMutationAuthority(
	ctx context.Context,
	sessionKey string,
	ownerUserID string,
	agentID string,
) (*protocol.Goal, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf(
			"%w: Goal mutation authority requires the current agent identity",
			ErrGoalForbidden,
		)
	}
	item, err := s.CurrentOptionalForOwner(ctx, sessionKey, ownerUserID)
	if err != nil || item == nil {
		return item, err
	}
	parsed := protocol.ParseSessionKey(item.SessionKey)
	switch parsed.Kind {
	case protocol.SessionKeyKindRoom:
		if err = authorizeRoomGoalModelMutation(*item, agentID); err != nil {
			return nil, err
		}
	case protocol.SessionKeyKindAgent:
		if strings.TrimSpace(parsed.AgentID) != agentID {
			return nil, fmt.Errorf(
				"%w: only the Goal session Agent %s may mutate this Goal",
				ErrGoalForbidden,
				strings.TrimSpace(parsed.AgentID),
			)
		}
	default:
		return nil, fmt.Errorf(
			"%w: unsupported Goal session identity",
			ErrGoalForbidden,
		)
	}
	return item, nil
}
