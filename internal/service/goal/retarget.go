// INPUT: 当前 session、用户明确替换后的 objective、当前 Agent 身份、round 与 objective revision。
// OUTPUT: 经 Room lead 授权、保留身份和累计用量并直接恢复 active 的 Goal 与审计事件。
// POS: 模型重定向当前 Goal 的唯一服务入口；不需要先恢复 blocked/paused/limited Goal。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RetargetByModel 按用户明确纠正替换当前 session 的 Goal objective。
func (s *Service) RetargetByModel(ctx context.Context, sessionKey string, request protocol.RetargetGoalRequest) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	normalizedSessionKey, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGoalInvalidInput, err)
	}
	objective, err := normalizeObjective(request.Objective)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetCurrentGoal(ctx, normalizedSessionKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrGoalNotFound
	}
	if expectedGoalID := strings.TrimSpace(request.ExpectedGoalID); expectedGoalID != "" &&
		current.ID != expectedGoalID {
		return nil, ErrGoalRevisionStale
	}
	if err := authorizeRoomGoalModelMutation(*current, request.AgentID); err != nil {
		return nil, err
	}
	if !canRetargetGoalStatus(current.Status) {
		return nil, ErrGoalInvalidState
	}
	if current.Objective == objective {
		if transition, ok := ObjectiveTransitionFromGoal(*current); ok &&
			transition.TargetObjective == objective &&
			(request.ExpectedObjectiveRevision <= 0 ||
				request.ExpectedObjectiveRevision == transition.OldRevision ||
				request.ExpectedObjectiveRevision == transition.NewRevision) {
			return current, nil
		}
		if !objectiveRevisionMatches(*current, request.ExpectedObjectiveRevision) {
			return nil, ErrGoalRevisionStale
		}
		return current, nil
	}
	if !objectiveRevisionMatches(*current, request.ExpectedObjectiveRevision) {
		return nil, ErrGoalRevisionStale
	}
	managed, bindingErr := s.goalHasManagedExecutionBinding(ctx, *current)
	if bindingErr != nil {
		return nil, bindingErr
	}
	if managed {
		if s.objectiveRetarget == nil {
			return nil, fmt.Errorf(
				"%w: Goal objective retarget coordinator is unavailable for a managed Execution",
				ErrGoalInvalidState,
			)
		}
		updated, coordinateErr := s.objectiveRetarget.RetargetGoalObjective(ctx, ObjectiveRetargetCommand{
			Goal:                      *current,
			RequestedObjective:        objective,
			Objective:                 objective,
			Reason:                    "user explicitly changed the Goal objective",
			ExpectedObjectiveRevision: current.ObjectiveRevision(),
			Source:                    protocol.GoalUpdateSourceModel,
			RoundID:                   strings.TrimSpace(request.RoundID),
			AgentID:                   strings.TrimSpace(request.AgentID),
		})
		if coordinateErr != nil {
			return nil, coordinateErr
		}
		s.updatePreviewFromGoal(ctx, *updated, "")
		return updated, nil
	}
	updated, err := s.retryGoalMutation(ctx, current, func(latest *protocol.Goal) (*protocol.Goal, error) {
		if !objectiveRevisionMatches(*latest, request.ExpectedObjectiveRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.retargetLoadedGoal(ctx, latest, objective, request)
	})
	if err != nil {
		return nil, err
	}
	s.updatePreviewFromGoal(ctx, *updated, "")
	return updated, nil
}

func (s *Service) goalHasManagedExecutionBinding(
	ctx context.Context,
	item protocol.Goal,
) (bool, error) {
	resolution, err := s.resolveGoalExecutionBinding(ctx, item)
	if err != nil {
		return false, fmt.Errorf("resolve Goal Execution binding: %w", err)
	}
	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return false, nil
	case protocol.GoalExecutionBindingStateConfirmed:
		return true, nil
	case protocol.GoalExecutionBindingStatePending,
		protocol.GoalExecutionBindingStateConflict:
		return false, fmt.Errorf(
			"%w: Goal Execution binding is %s",
			ErrGoalInvalidState,
			resolution.State,
		)
	default:
		return false, fmt.Errorf("%w: Goal Execution binding state is unknown", ErrGoalInvalidState)
	}
}

func (s *Service) retargetLoadedGoal(
	ctx context.Context,
	current *protocol.Goal,
	objective string,
	request protocol.RetargetGoalRequest,
) (*protocol.Goal, error) {
	if err := authorizeRoomGoalModelMutation(*current, request.AgentID); err != nil {
		return nil, err
	}
	if !canRetargetGoalStatus(current.Status) {
		return nil, ErrGoalInvalidState
	}
	if current.Objective == objective {
		return current, nil
	}

	current.Objective = objective
	advanceObjectiveRevision(current)
	resetGoalContinuationForObjectiveReplacement(current)
	payload := map[string]any{
		"objective":          objective,
		"objective_updated":  true,
		"objective_revision": current.ObjectiveRevision(),
	}
	if agentID := strings.TrimSpace(request.AgentID); agentID != "" {
		payload["source_agent_id"] = agentID
	}
	return s.persistTransition(ctx, *current, protocol.GoalStatusActive, protocol.GoalUpdateSourceModel, "updated", strings.TrimSpace(request.RoundID), payload)
}
