// INPUT: structured Work/Review actor、runtime Execution snapshot、当前 Goal 与中央 binding resolver。
// OUTPUT: 仅 confirmed 且三方 identity 完全一致时生成的 Goal mutation authority。
// POS: Room Work/Review 不得仅凭 SQL snapshot 或 ambient Goal 推断 Goal capability。
package realtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionGoalBindingResolver interface {
	ResolveGoalExecutionBinding(
		context.Context,
		protocol.Goal,
	) (protocol.GoalExecutionBindingResolution, error)
}

var _ executionGoalBindingResolver = (*orchestrationsvc.Service)(nil)

func (s *Service) resolveExecutionGoalMutationAuthority(
	ctx context.Context,
	actor orchestrationsvc.ActorContext,
	rootRoundID string,
) (string, roomGoalMutationAuthority, bool, error) {
	binding, err := s.executionGoalBinding(ctx, actor)
	if err != nil {
		return "", roomGoalMutationAuthority{}, false, err
	}
	if strings.TrimSpace(binding.GoalID) == "" {
		return "", roomGoalMutationAuthority{}, false, nil
	}

	goalContext, goal, ok := s.goalRuntimeSnapshot(ctx, binding.SessionKey)
	if !ok || goal == nil ||
		strings.TrimSpace(goal.ID) != strings.TrimSpace(binding.GoalID) ||
		goal.ObjectiveRevision() != binding.GoalObjectiveRevision ||
		strings.TrimSpace(goal.SessionKey) != strings.TrimSpace(binding.SessionKey) {
		return "", roomGoalMutationAuthority{}, false, goalsvc.ErrGoalRevisionStale
	}

	resolver, ok := s.executionContext.(executionGoalBindingResolver)
	if !ok || resolver == nil {
		return "", roomGoalMutationAuthority{}, false, fmt.Errorf(
			"%w: central Goal Execution binding resolver is unavailable",
			goalsvc.ErrGoalExecutionBindingConflict,
		)
	}
	resolution, err := resolver.ResolveGoalExecutionBinding(ctx, *goal)
	if err != nil {
		return "", roomGoalMutationAuthority{}, false, fmt.Errorf(
			"resolve Goal Execution binding: %w",
			err,
		)
	}

	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return "", roomGoalMutationAuthority{}, false, nil
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		return "", roomGoalMutationAuthority{}, false, fmt.Errorf(
			"%w: Goal Execution binding is %s",
			goalsvc.ErrGoalExecutionBindingConflict,
			resolution.State,
		)
	case protocol.GoalExecutionBindingStateConfirmed:
	default:
		return "", roomGoalMutationAuthority{}, false, fmt.Errorf(
			"%w: Goal Execution binding state is unknown",
			goalsvc.ErrGoalExecutionBindingConflict,
		)
	}

	actorExecutionID := executionIDFromRoomBindings(actor.WorkBinding, actor.ReviewBinding)
	if actorExecutionID == "" ||
		strings.TrimSpace(binding.ExecutionID) != actorExecutionID ||
		strings.TrimSpace(resolution.ExecutionID) != actorExecutionID {
		return "", roomGoalMutationAuthority{}, false, fmt.Errorf(
			"%w: confirmed Goal Execution differs from the Work/Review binding",
			goalsvc.ErrGoalExecutionBindingConflict,
		)
	}

	return goalContext, roomGoalMutationAuthority{
		SessionKey:        strings.TrimSpace(binding.SessionKey),
		GoalID:            strings.TrimSpace(binding.GoalID),
		ObjectiveRevision: binding.GoalObjectiveRevision,
		ExecutionID:       actorExecutionID,
		RootRoundID:       strings.TrimSpace(rootRoundID),
		Source:            roomGoalAuthorityExecutionBinding,
	}, true, nil
}
