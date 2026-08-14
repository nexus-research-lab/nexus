// INPUT: Room Goal 可见协作事实、稳定 Goal identity 与事件归因 revision。
// OUTPUT: Goal 生命周期内单调累计、记录时 revision 安全的审计 metadata 和事件。
// POS: Room Goal 协作事实的唯一状态入口；证据不参与 complete 判定，retarget 不会抹除已成立事实。
package goal

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func (s *Service) recordRoomGoalCollaborationEvidenceForGoal(ctx context.Context, item *protocol.Goal, roundID string, agentID string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record Room collaboration evidence"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordRoomGoalCollaborationEvidenceForLoadedGoal(ctx, current, roundID, agentID)
	})
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
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationObserved] = true
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationAgentID] = agentID
	if roundID != "" {
		item.Metadata[protocol.GoalMetadataRoomGoalCollaborationRoundID] = roundID
	}
	item.Metadata[protocol.GoalMetadataRoomGoalCollaborationObservedAt] = s.nowFn().UTC().Format(time.RFC3339)
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "room_collaboration_observed", protocol.GoalUpdateSourceSystem, roundID, map[string]any{
		"agent_id": agentID,
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}
