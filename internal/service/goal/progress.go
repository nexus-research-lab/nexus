// INPUT: runtime round 进展、失败、Room collaboration handback 与 objective revision fence。
// OUTPUT: CAS 重试后的 Goal 进展状态和审计事件。
// POS: Goal round 结果回写的唯一入口。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	goalCompletionToolRetryMetadataKey = "completion_tool_retry_count"
	goalCompletionToolMaxRetries       = 1
)

// RecordContinuationProgress 记录上一轮 Goal 续跑是否产生了可计入的自主进展。
func (s *Service) RecordContinuationProgress(ctx context.Context, goalID string, roundID string, progressed bool, expectedRevision ...int64) (*protocol.Goal, error) {
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
	return s.recordContinuationProgressForGoal(ctx, item, strings.TrimSpace(roundID), progressed, firstExpectedObjectiveRevision(expectedRevision))
}

// RecordContinuationFailure 记录 Goal 续跑的 runtime 失败原因，并暂停后续空转续跑。
func (s *Service) RecordContinuationFailure(ctx context.Context, goalID string, roundID string, reason string, expectedRevision ...int64) (*protocol.Goal, error) {
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
	return s.recordContinuationFailureForGoal(ctx, item, strings.TrimSpace(roundID), reason, firstExpectedObjectiveRevision(expectedRevision))
}

// RecordCompletionToolMiss 记录模型已声称目标完成但漏调 Goal 完成工具，并安排一次收尾重试。
func (s *Service) RecordCompletionToolMiss(ctx context.Context, goalID string, roundID string, reason string, expectedRevision ...int64) (*protocol.Goal, error) {
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
	return s.recordCompletionToolMissForGoal(ctx, item, strings.TrimSpace(roundID), reason, firstExpectedObjectiveRevision(expectedRevision))
}

// RecordGoalActivity 记录显式用户/外部活动，让自动续跑 run 从当前轮重新开始计数。
func (s *Service) RecordGoalActivity(ctx context.Context, goalID string, roundID string, expectedRevision ...int64) (*protocol.Goal, error) {
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
	return s.recordGoalActivityForGoal(ctx, item, strings.TrimSpace(roundID), firstExpectedObjectiveRevision(expectedRevision))
}

// RecordRoomGoalCollaborationHandback 记录 target Agent 已将控制权归还 host。
// 它不把 target 输出冒充为新的模型工具进展，也不重置
// ContinuationCount；只清除源 round 在旧实现中可能抢先写入的
// empty-progress 抑制，让后续仍受正常续跑上限约束。
func (s *Service) RecordRoomGoalCollaborationHandback(
	ctx context.Context,
	goalID string,
	roundID string,
	expectedRevision ...int64,
) (*protocol.Goal, error) {
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
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record Room Goal collaboration handback"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, firstExpectedObjectiveRevision(expectedRevision)) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordRoomGoalCollaborationHandbackForLoadedGoal(
			ctx,
			current,
			strings.TrimSpace(roundID),
		)
	})
}

func (s *Service) recordContinuationProgressForGoal(ctx context.Context, item *protocol.Goal, roundID string, progressed bool, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if _, planning := objectiveTransitionAwaitingPlan(*current); !planning {
			if err := rejectPendingObjectiveTransition(*current, "record continuation progress"); err != nil {
				return nil, err
			}
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordContinuationProgressForLoadedGoal(ctx, current, roundID, progressed)
	})
}

func (s *Service) recordContinuationFailureForGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if _, planning := objectiveTransitionAwaitingPlan(*current); !planning {
			if err := rejectPendingObjectiveTransition(*current, "record continuation failure"); err != nil {
				return nil, err
			}
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordContinuationFailureForLoadedGoal(ctx, current, roundID, reason)
	})
}

func (s *Service) recordCompletionToolMissForGoal(ctx context.Context, item *protocol.Goal, roundID string, reason string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record completion-tool progress"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordCompletionToolMissForLoadedGoal(ctx, current, roundID, reason)
	})
}

func (s *Service) recordGoalActivityForGoal(ctx context.Context, item *protocol.Goal, roundID string, expectedRevision int64) (*protocol.Goal, error) {
	return s.retryGoalMutation(ctx, item, func(current *protocol.Goal) (*protocol.Goal, error) {
		if err := rejectPendingObjectiveTransition(*current, "record Goal activity"); err != nil {
			return nil, err
		}
		if !objectiveRevisionMatches(*current, expectedRevision) {
			return nil, ErrGoalRevisionStale
		}
		return s.recordGoalActivityForLoadedGoal(ctx, current, roundID)
	})
}

func (s *Service) retryGoalMutation(ctx context.Context, item *protocol.Goal, mutate func(*protocol.Goal) (*protocol.Goal, error)) (*protocol.Goal, error) {
	current := item
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		updated, err := mutate(current)
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
	payload := map[string]any{
		"empty_progress_count": item.EmptyProgressCount,
		"reason":               reason,
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "continuation_failed", protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
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
	payload := map[string]any{
		"retry_count": item.Metadata[goalCompletionToolRetryMetadataKey],
		"reason":      reason,
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "completion_tool_retry", protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) completeAfterCompletionToolMissRetry(ctx context.Context, item *protocol.Goal, roundID string, reason string) (*protocol.Goal, error) {
	if alignmentErr := s.ensureGoalObjectiveAlignmentReady(
		ctx,
		*item,
		RoomLeadAgentID(*item),
		roundID,
	); alignmentErr != nil {
		return s.noteEmptyContinuationProgress(ctx, item, roundID, alignmentErr.Error())
	}
	if readinessErr := s.ensureExecutionGoalCompletionReady(ctx, *item); readinessErr != nil {
		return s.noteEmptyContinuationProgress(ctx, item, roundID, readinessErr.Error())
	}
	if readinessErr := s.ensureRoomGoalCollaborationReady(
		ctx,
		*item,
		RoomLeadAgentID(*item),
		roundID,
	); readinessErr != nil {
		return s.noteEmptyContinuationProgress(ctx, item, roundID, readinessErr.Error())
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
	item.Metadata = clearContinuationReservations(clearCompletionToolRetryMetadata(item.Metadata))
	item.Version++
	item.UpdatedAt = s.nowFn()
	payload := map[string]any{
		"continuation_count":   item.ContinuationCount,
		"empty_progress_count": item.EmptyProgressCount,
		"reason":               "explicit goal activity reset continuation run",
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "continuation_reset", protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
		return nil, err
	}
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) recordRoomGoalCollaborationHandbackForLoadedGoal(
	ctx context.Context,
	item *protocol.Goal,
	roundID string,
) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
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
	updated, err := s.persistGoalUpdateWithEvent(
		ctx,
		*item,
		expectedVersion,
		"room_collaboration_handback",
		protocol.GoalUpdateSourceSystem,
		roundID,
		map[string]any{
			"continuation_count":   item.ContinuationCount,
			"empty_progress_count": item.EmptyProgressCount,
			"reason":               "Room collaboration target returned control to the Goal host",
		},
	)
	if err != nil {
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
	payload := map[string]any{
		"empty_progress_count": item.EmptyProgressCount,
		"reason":               reason,
	}
	updated, err := s.persistGoalUpdateWithEvent(ctx, *item, expectedVersion, "continuation_suppressed", protocol.GoalUpdateSourceSystem, roundID, payload)
	if err != nil {
		return nil, err
	}
	s.clearWallClockGoal(*updated)
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

func resetCountersForActiveTransition(source protocol.GoalUpdateSource, status protocol.GoalStatus) bool {
	if protocol.NormalizeGoalStatus(status) != protocol.GoalStatusActive {
		return false
	}
	return source == protocol.GoalUpdateSourceUser || source == protocol.GoalUpdateSourceExternal
}
