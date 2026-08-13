// INPUT: 已完成 owner authorization 的当前 Goal、中央 Goal/Execution binding resolver 与清理来源。
// OUTPUT: 仅 standalone/reserved Goal 可删除；pending/confirmed/conflict binding 一律 fail closed。
// POS: REST 与 app-server HTTP/WS Goal clear 共用的最终生命周期边界。
package goal

import (
	"context"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) clearGoal(
	ctx context.Context,
	item protocol.Goal,
	source protocol.GoalUpdateSource,
) (bool, error) {
	if err := s.ensureGoalClearAllowed(ctx, item); err != nil {
		return false, err
	}
	return s.deleteGoal(ctx, item, source)
}

func (s *Service) ensureGoalClearAllowed(ctx context.Context, item protocol.Goal) error {
	resolution, err := s.resolveGoalExecutionBinding(ctx, item)
	if err != nil {
		return fmt.Errorf("resolve Goal Execution binding before clear: %w", err)
	}
	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return nil
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConfirmed,
		protocol.GoalExecutionBindingStateConflict:
		return fmt.Errorf(
			"%w: cannot clear Goal while Execution binding is %s",
			ErrGoalInvalidState,
			resolution.State,
		)
	default:
		return fmt.Errorf(
			"%w: cannot clear Goal with unknown Execution binding state",
			ErrGoalInvalidState,
		)
	}
}
