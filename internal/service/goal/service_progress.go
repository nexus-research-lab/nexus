package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	goalCompletionToolRetryMetadataKey = "completion_tool_retry_count"
	goalCompletionToolMaxRetries       = 1
)

// RecordContinuationProgress 记录上一轮 Goal 续跑是否产生了可计入的自主进展。
func (s *Service) RecordContinuationProgress(ctx context.Context, goalID string, roundID string, progressed bool) (*protocol.Goal, error) {
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
	return s.recordContinuationProgressForGoal(ctx, item, strings.TrimSpace(roundID), progressed)
}

// RecordContinuationFailure 记录 Goal 续跑的 runtime 失败原因，并暂停后续空转续跑。
func (s *Service) RecordContinuationFailure(ctx context.Context, goalID string, roundID string, reason string) (*protocol.Goal, error) {
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
	return s.recordContinuationFailureForGoal(ctx, item, strings.TrimSpace(roundID), reason)
}

// RecordCompletionToolMiss 记录模型已声称目标完成但漏调 Goal 完成工具，并安排一次收尾重试。
func (s *Service) RecordCompletionToolMiss(ctx context.Context, goalID string, roundID string, reason string) (*protocol.Goal, error) {
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
	return s.recordCompletionToolMissForGoal(ctx, item, strings.TrimSpace(roundID), reason)
}

// RecordGoalActivity 记录显式用户/外部活动，让自动续跑 run 从当前轮重新开始计数。
func (s *Service) RecordGoalActivity(ctx context.Context, goalID string, roundID string) (*protocol.Goal, error) {
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
	return s.recordGoalActivityForGoal(ctx, item, strings.TrimSpace(roundID))
}

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
func (s *Service) RecordRoomGoalCollaborationEvidence(ctx context.Context, goalID string, roundID string, agentID string) (*protocol.Goal, error) {
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
	return s.recordRoomGoalCollaborationEvidenceForGoal(ctx, item, strings.TrimSpace(roundID), strings.TrimSpace(agentID))
}

func (s *Service) recordContinuationProgressForGoal(ctx context.Context, item *protocol.Goal, roundID string, progressed bool) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := s.recordContinuationProgressForLoadedGoal(ctx, current, roundID, progressed)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordContinuationFailureForGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := s.recordContinuationFailureForLoadedGoal(ctx, current, roundID, reason)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordCompletionToolMissForGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := s.recordCompletionToolMissForLoadedGoal(ctx, current, roundID, reason)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordGoalActivityForGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := s.recordGoalActivityForLoadedGoal(ctx, current, roundID)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordRoomGoalCollaborationRequiredForGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := s.recordRoomGoalCollaborationRequiredForLoadedGoal(ctx, current, roundID)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordRoomGoalCollaborationEvidenceForGoal(ctx context.Context, item *protocol.Goal, roundID string, agentID string) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := s.recordRoomGoalCollaborationEvidenceForLoadedGoal(ctx, current, roundID, agentID)
		if !errors.Is(err, ErrGoalVersionStale) {
			return updated, err
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, current.ID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		current = reloaded
	}
	return nil, ErrGoalVersionStale
}

func (s *Service) recordContinuationProgressForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, progressed bool) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	if progressed {
		return s.resetContinuationProgress(ctx, item)
	}
	return s.noteEmptyContinuationProgress(ctx, item, roundID)
}

func (s *Service) recordContinuationFailureForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Goal continuation runtime failed"
	}
	expectedVersion := item.Version
	item.EmptyProgressCount++
	item.LastError = reason
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"empty_progress_count": updated.EmptyProgressCount,
		"reason":               reason,
	}
	if err := s.appendEvent(ctx, *updated, "continuation_failed", protocol.GoalUpdateSourceSystem, roundID, payload); err != nil {
		return nil, err
	}
	s.clearWallClockGoal(*updated)
	return updated, nil
}

func (s *Service) recordCompletionToolMissForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Goal completion was claimed, but the Goal update tool was not called"
	}
	retryCount := goalCompletionToolRetryCount(item.Metadata)
	if retryCount >= goalCompletionToolMaxRetries {
		return s.completeAfterCompletionToolMissRetry(ctx, item, roundID, reason)
	}
	expectedVersion := item.Version
	item.Metadata = cloneMap(item.Metadata)
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	item.Metadata[goalCompletionToolRetryMetadataKey] = goalCompletionToolRetryCount(item.Metadata) + 1
	item.EmptyProgressCount = 0
	item.LastError = ""
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"retry_count": updated.Metadata[goalCompletionToolRetryMetadataKey],
		"reason":      reason,
	}
	if err := s.appendEvent(ctx, *updated, "completion_tool_retry", protocol.GoalUpdateSourceSystem, roundID, payload); err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) completeAfterCompletionToolMissRetry(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	if roomGoalCompletionRequiresCollaboration(*item) {
		return s.noteEmptyContinuationProgress(ctx, item, roundID, "Room Goal completion requires room-visible non-lead collaboration")
	}
	retryCount := goalCompletionToolRetryCount(item.Metadata)
	item.Metadata = clearCompletionToolRetryMetadata(item.Metadata)
	item.EmptyProgressCount = 0
	item.LastError = ""
	return s.persistTransition(ctx, *item, protocol.GoalStatusComplete, protocol.GoalUpdateSourceSystem, "completed", roundID, map[string]any{
		"reason":      strings.TrimSpace(reason),
		"retry_count": retryCount,
		"source":      "completion_tool_miss",
	})
}

func (s *Service) recordGoalActivityForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	if item.EmptyProgressCount == 0 &&
		item.ContinuationCount == 0 &&
		strings.TrimSpace(item.LastError) == "" &&
		goalCompletionToolRetryCount(item.Metadata) == 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.EmptyProgressCount = 0
	item.ContinuationCount = 0
	item.LastError = ""
	item.Metadata = clearCompletionToolRetryMetadata(item.Metadata)
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"continuation_count":   updated.ContinuationCount,
		"empty_progress_count": updated.EmptyProgressCount,
		"reason":               "explicit goal activity reset continuation run",
	}
	if err := s.appendEvent(ctx, *updated, "continuation_reset", protocol.GoalUpdateSourceSystem, roundID, payload); err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) resetContinuationProgress(ctx context.Context, item *protocol.Goal) (*protocol.Goal, error) {
	if item.EmptyProgressCount == 0 &&
		strings.TrimSpace(item.LastError) == "" &&
		goalCompletionToolRetryCount(item.Metadata) == 0 {
		return item, nil
	}
	expectedVersion := item.Version
	item.EmptyProgressCount = 0
	item.LastError = ""
	item.Metadata = clearCompletionToolRetryMetadata(item.Metadata)
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) noteEmptyContinuationProgress(ctx context.Context, item *protocol.Goal, roundID string, reasonOverride ...string) (*protocol.Goal, error) {
	reason := "goal continuation produced no counted tool progress"
	if len(reasonOverride) > 0 && strings.TrimSpace(reasonOverride[0]) != "" {
		reason = strings.TrimSpace(reasonOverride[0])
	}
	expectedVersion := item.Version
	item.EmptyProgressCount++
	item.Version++
	item.UpdatedAt = s.nowFn()
	updated, err := s.repo.UpdateGoal(ctx, *item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"empty_progress_count": updated.EmptyProgressCount,
		"reason":               reason,
	}
	if err := s.appendEvent(ctx, *updated, "continuation_suppressed", protocol.GoalUpdateSourceSystem, roundID, payload); err != nil {
		return nil, err
	}
	s.clearWallClockGoal(*updated)
	return updated, nil
}

func (s *Service) recordRoomGoalCollaborationRequiredForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive || !protocol.IsRoomSharedSessionKey(item.SessionKey) {
		return item, nil
	}
	if protocol.GoalRoomCollaborationRequired(*item) {
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
		agentID == protocol.GoalRoomLeadAgentID(*item) {
		return item, nil
	}
	if protocol.GoalRoomCollaborationObserved(*item) {
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

func goalCompletionToolRetryCount(metadata map[string]any) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[goalCompletionToolRetryMetadataKey].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func roomGoalCompletionRequiresCollaboration(item protocol.Goal) bool {
	return protocol.IsRoomSharedSessionKey(item.SessionKey) &&
		protocol.GoalRoomCollaborationRequired(item) &&
		!protocol.GoalRoomCollaborationObserved(item)
}

func clearCompletionToolRetryMetadata(metadata map[string]any) map[string]any {
	if goalCompletionToolRetryCount(metadata) == 0 {
		return metadata
	}
	copied := cloneMap(metadata)
	delete(copied, goalCompletionToolRetryMetadataKey)
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func resetEmptyProgressForTransition(source protocol.GoalUpdateSource, status protocol.GoalStatus) bool {
	if protocol.NormalizeGoalStatus(status) != protocol.GoalStatusActive {
		return false
	}
	return source == protocol.GoalUpdateSourceUser || source == protocol.GoalUpdateSourceExternal
}

func resetContinuationCountForTransition(source protocol.GoalUpdateSource, status protocol.GoalStatus) bool {
	if protocol.NormalizeGoalStatus(status) != protocol.GoalStatusActive {
		return false
	}
	return source == protocol.GoalUpdateSourceUser || source == protocol.GoalUpdateSourceExternal
}

func resetCompletionToolRetryForTransition(source protocol.GoalUpdateSource, status protocol.GoalStatus) bool {
	if protocol.NormalizeGoalStatus(status) != protocol.GoalStatusActive {
		return false
	}
	return source == protocol.GoalUpdateSourceUser || source == protocol.GoalUpdateSourceExternal
}
