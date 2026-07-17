// INPUT: Room Goal、当前模型 Agent/root round 与 Room 运行中工作快照。
// OUTPUT: complete 前的 outstanding-work gate 与稳定 Goal 状态错误。
// POS: Goal 状态机与 Room 实时编排之间的窄完成条件边界。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type roomGoalCompletionReadiness interface {
	RoomGoalCompletionBlocker(context.Context, protocol.Goal, string, string) (string, error)
}

// SetRoomGoalCompletionReadiness 注入 Room 运行中工作检查，防止模型过早终结共享 Goal。
func (s *Service) SetRoomGoalCompletionReadiness(readiness roomGoalCompletionReadiness) {
	s.roomCompletion = readiness
}

func (s *Service) ensureRoomGoalCompletionReady(
	ctx context.Context,
	item protocol.Goal,
	agentID string,
	roundID string,
) error {
	if !protocol.IsRoomSharedSessionKey(item.SessionKey) || s.roomCompletion == nil {
		return nil
	}
	blocker, err := s.roomCompletion.RoomGoalCompletionBlocker(
		ctx,
		item,
		strings.TrimSpace(agentID),
		strings.TrimSpace(roundID),
	)
	if err != nil {
		return fmt.Errorf("check Room Goal completion readiness: %w", err)
	}
	if blocker = strings.TrimSpace(blocker); blocker != "" {
		return fmt.Errorf("%w: Room Goal still has outstanding work: %s", ErrGoalInvalidState, blocker)
	}
	return nil
}
