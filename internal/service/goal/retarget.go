// INPUT: 当前 session、用户明确纠正后的 objective、消费该纠正的 round 与工具入口的 objective revision。
// OUTPUT: 保留身份和累计用量的 active Goal、刷新后的预览，以及 model 来源的 objective_updated 审计事件。
// POS: 模型重定向当前 Goal 的唯一服务入口；不承担用户面板的通用 Goal 编辑。
package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RetargetByModel 按用户明确纠正重定向当前 session 的 active Goal。
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
	if protocol.NormalizeGoalStatus(current.Status) != protocol.GoalStatusActive {
		return nil, ErrGoalInvalidState
	}
	if !objectiveRevisionMatches(*current, request.ExpectedObjectiveRevision) {
		return nil, ErrGoalRevisionStale
	}
	if current.Objective == objective {
		return current, nil
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

func (s *Service) retargetLoadedGoal(
	ctx context.Context,
	current *protocol.Goal,
	objective string,
	request protocol.RetargetGoalRequest,
) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(current.Status) != protocol.GoalStatusActive {
		return nil, ErrGoalInvalidState
	}
	if current.Objective == objective {
		return current, nil
	}

	current.Objective = objective
	advanceObjectiveRevision(current)
	current.ContinuationCount = 0
	current.EmptyProgressCount = 0
	current.Metadata = clearContinuationReservations(clearCompletionToolRetryMetadata(current.Metadata))
	if protocol.IsRoomSharedSessionKey(current.SessionKey) {
		current.Metadata = cloneMap(current.Metadata)
		delete(current.Metadata, protocol.GoalMetadataRoomGoalCollaborationObserved)
		delete(current.Metadata, protocol.GoalMetadataRoomGoalCollaborationAgentID)
		delete(current.Metadata, protocol.GoalMetadataRoomGoalCollaborationRoundID)
		delete(current.Metadata, protocol.GoalMetadataRoomGoalCollaborationObservedAt)
	}
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
