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
	if s.executionCompletion == nil {
		if goalRequiresExecutionCompletionAudit(item) {
			return fmt.Errorf(
				"%w: Execution completion audit is unavailable for a bound Goal",
				ErrGoalInvalidState,
			)
		}
		return nil
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

func goalRequiresExecutionCompletionAudit(item protocol.Goal) bool {
	if protocol.GoalMetadataString(
		item.Metadata,
		protocol.GoalMetadataExecutionID,
	) != "" {
		return true
	}
	switch protocol.GoalActivationOrigin(protocol.GoalMetadataString(
		item.Metadata,
		protocol.GoalMetadataActivationOrigin,
	)) {
	case protocol.GoalActivationOriginUserExplicit,
		protocol.GoalActivationOriginAdaptiveInitial,
		protocol.GoalActivationOriginAdaptivePromoted:
		return true
	default:
		return false
	}
}
