// INPUT: Room Goal、当前模型 Agent/root round 与 Room 运行中工作快照。
// OUTPUT: complete 前的当前 Room 成员门槛、Goal 生命周期协作证据、outstanding-work gate 与稳定 Goal 状态错误。
// POS: Goal 状态机与 Room 实时编排之间的窄完成条件边界。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type roomGoalCompletionReadiness interface {
	RoomGoalCompletionReport(context.Context, protocol.Goal, string, string) (RoomGoalCompletionReport, error)
}

// RoomGoalCompletionReport 是 Room 当前事实对 Goal complete 的单次一致读取。
type RoomGoalCompletionReport struct {
	CollaborationRequired bool
	Blocker               string
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
) (RoomGoalCompletionReport, error) {
	if !protocol.IsRoomSharedSessionKey(item.SessionKey) || s.roomCompletion == nil {
		return RoomGoalCompletionReport{
			CollaborationRequired: RoomCollaborationRequired(item),
		}, nil
	}
	readiness, err := s.roomCompletion.RoomGoalCompletionReport(
		ctx,
		item,
		strings.TrimSpace(agentID),
		strings.TrimSpace(roundID),
	)
	if err != nil {
		return RoomGoalCompletionReport{}, fmt.Errorf("check Room Goal completion readiness: %w", err)
	}
	readiness.CollaborationRequired = readiness.CollaborationRequired || RoomCollaborationRequired(item)
	if blocker := strings.TrimSpace(readiness.Blocker); blocker != "" {
		return readiness, fmt.Errorf("%w: Room Goal still has outstanding work: %s", ErrGoalInvalidState, blocker)
	}
	return readiness, nil
}

func (s *Service) ensureRoomGoalCollaborationReady(
	ctx context.Context,
	item protocol.Goal,
	agentID string,
	roundID string,
) error {
	readiness, err := s.ensureRoomGoalCompletionReady(ctx, item, agentID, roundID)
	if err != nil {
		return err
	}
	if readiness.CollaborationRequired && !RoomCollaborationObserved(item) {
		return fmt.Errorf("%w: multi-member Room Goal requires a room-visible non-lead collaboration reply before completion", ErrGoalInvalidState)
	}
	return nil
}
