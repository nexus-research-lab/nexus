// INPUT: Room Goal 协作要求、可见证据与 objective revision。
// OUTPUT: revision 安全的协作 metadata 和审计事件。
// POS: Room Goal 协作完成条件的唯一状态入口。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RecordRoomGoalCollaborationRequired 标记多成员 Room Goal 完成前必须具备非负责人可见协作证据。
func (s *Service) RecordRoomGoalCollaborationRequired(ctx context.Context, goalID string, roundID string) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordRoomGoalCollaborationRequiredForGoal(ctx, item, strings.TrimSpace(roundID))
}

// RecordRoomGoalCollaborationEvidence 记录非负责人在房间可见回复中参与了 Room Goal。
func (s *Service) RecordRoomGoalCollaborationEvidence(ctx context.Context, goalID string, roundID string, agentID string, expectedRevision ...int64) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	item, err := s.repo.GetGoal(ctx, strings.TrimSpace(goalID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	return s.recordRoomGoalCollaborationEvidenceForGoal(ctx, item, strings.TrimSpace(roundID), strings.TrimSpace(agentID), firstExpectedObjectiveRevision(expectedRevision))
}

func (s *Service) recordRoomGoalCollaborationRequiredForGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		return s.recordRoomGoalCollaborationRequiredForLoadedGoal(ctx, current, roundID)
	})
}

func (s *Service) recordRoomGoalCollaborationEvidenceForGoal(ctx context.Context, item *protocol.Goal, roundID string, agentID string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordRoomGoalCollaborationEvidenceForLoadedGoal(ctx, current, roundID, agentID)
	})
}

func (s *Service) recordRoomGoalCollaborationRequiredForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive || !protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return item, nil
	}
	if RoomCollaborationRequired(*item) {
		return item, nil
	}
	expectedVersion := item.Version
	item.Metadata = cloneMap(item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequired] = true
	if roundID != "" {
		item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequirementRound] = roundID
	}
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *updated, "room_collaboration_required", protocol.GoalUpdateSourceSystem, roundID, map[string]any{
		"reason": "multi-member Room Goal requires room-visible non-lead collaboration before completion",
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) recordRoomGoalCollaborationEvidenceForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, agentID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive ||
		!protocol.IsRoomSharedSessionKey(item.SessionKey) ||
		agentID == "" ||
		agentID == RoomLeadAgentID(*item) {
		return item, nil
	}
	if RoomCollaborationObserved(*item) {
		return item, nil
	}
	expectedVersion := item.Version
	item.Metadata = cloneMap(item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRequired] = true
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationObserved] = true
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationAgentID] = agentID
	if roundID != "" {
		item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRoundID] = roundID
	}
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationObservedAt] = s.nowFn().UTC().Format(time.RFC3339)
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *updated, "room_collaboration_observed", protocol.GoalUpdateSourceSystem, roundID, map[string]any{
		"agent_id": agentID,
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

func roomGoalCompletionRequiresCollaboration(item protocol.Goal) bool {
	return protocol.IsRoomSharedSessionKey(item.SessionKey) &&
		RoomCollaborationRequired(item) &&
		!RoomCollaborationObserved(item)
}
