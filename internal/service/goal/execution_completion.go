// INPUT: Goal 与其绑定 Execution 的 WorkGraph readiness。
// OUTPUT: 所有 Goal complete 路径共用的未完成工作 gate。
// POS: Goal 状态机与 Execution Orchestration 之间的窄完成审计边界。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type executionGoalCompletionReadiness interface {
	ExecutionGoalCompletionBlocker(context.Context, protocol.Goal) (string, error)
}

type executionGoalBindingResolver interface {
	ResolveGoalExecutionBinding(
		context.Context,
		protocol.Goal,
	) (protocol.GoalExecutionBindingResolution, error)
}

// SetExecutionGoalCompletionReadiness 注入 WorkGraph 审计，防止模型或系统旁路提前完成 Goal。
func (s *Service) SetExecutionGoalCompletionReadiness(readiness executionGoalCompletionReadiness) {
	s.executionCompletion = readiness
}

func (s *Service) ensureExecutionGoalCompletionReady(
	ctx context.Context,
	item protocol.Goal,
) error {
	if GoalObjectiveTransitionPending(item) {
		transition, valid := ObjectiveTransitionFromGoal(item)
		if !valid {
			return fmt.Errorf(
				"%w: Goal objective transition metadata is malformed",
				ErrGoalInvalidState,
			)
		}
		return fmt.Errorf(
			"%w: Goal objective transition %s is %s and has not bound its successor WorkGraph",
			ErrGoalInvalidState,
			transition.ID,
			transition.Phase,
		)
	}
	resolution, err := s.resolveGoalExecutionBinding(ctx, item)
	if err != nil {
		return fmt.Errorf("resolve Goal Execution binding: %w", err)
	}
	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return nil
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		return fmt.Errorf(
			"%w: Goal Execution binding is %s",
			ErrGoalInvalidState,
			resolution.State,
		)
	case protocol.GoalExecutionBindingStateConfirmed:
	default:
		return fmt.Errorf("%w: Goal Execution binding state is unknown", ErrGoalInvalidState)
	}
	if s.executionCompletion == nil {
		return fmt.Errorf(
			"%w: Execution completion audit is unavailable for a Goal with a confirmed managed WorkGraph binding",
			ErrGoalInvalidState,
		)
	}
	blocker, err := s.executionCompletion.ExecutionGoalCompletionBlocker(ctx, item)
	if err != nil {
		return fmt.Errorf("check Execution Goal completion readiness: %w", err)
	}
	if blocker = strings.TrimSpace(blocker); blocker != "" {
		return fmt.Errorf("%w: Goal still has outstanding Execution work: %s", ErrGoalInvalidState, blocker)
	}
	return nil
}

func (s *Service) resolveGoalExecutionBinding(
	ctx context.Context,
	item protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	if resolver, ok := s.executionCompletion.(executionGoalBindingResolver); ok {
		return resolver.ResolveGoalExecutionBinding(ctx, item)
	}
	resolution := protocol.GoalExecutionBindingResolution{
		State:               protocol.GoalExecutionBindingStateStandalone,
		ReservedExecutionID: protocol.GoalReservedExecutionID(item),
	}
	switch protocol.GoalExecutionBindingStateFromGoal(item) {
	case protocol.GoalExecutionBindingStateConfirmed:
		resolution.State = protocol.GoalExecutionBindingStateConfirmed
		resolution.ExecutionID = resolution.ReservedExecutionID
	case protocol.GoalExecutionBindingStatePending:
		resolution.State = protocol.GoalExecutionBindingStatePending
	case protocol.GoalExecutionBindingStateReserved:
		resolution.State = protocol.GoalExecutionBindingStateReserved
	case protocol.GoalExecutionBindingStateStandalone:
		// Compatibility for focused service tests and pre-phase data. Production
		// wiring always uses the database-backed resolver above.
		if resolution.ReservedExecutionID != "" {
			if len(goalMetadataStrings(item.Metadata, protocol.GoalMetadataCompletionCriteria)) > 0 {
				resolution.State = protocol.GoalExecutionBindingStateConfirmed
				resolution.ExecutionID = resolution.ReservedExecutionID
			} else {
				resolution.State = protocol.GoalExecutionBindingStateReserved
			}
		}
	case protocol.GoalExecutionBindingStateConflict:
		resolution.State = protocol.GoalExecutionBindingStateConflict
	default:
		resolution.State = protocol.GoalExecutionBindingStateConflict
	}
	return resolution, nil
}
