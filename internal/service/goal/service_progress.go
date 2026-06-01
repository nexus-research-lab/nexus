package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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

func (s *Service) recordGoalActivityForLoadedGoal(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
	if protocol.NormalizeGoalStatus(item.Status) != protocol.GoalStatusActive {
		return item, nil
	}
	if item.EmptyProgressCount == 0 &&
		item.ContinuationCount == 0 &&
		strings.TrimSpace(item.LastError) == "" {
		return item, nil
	}
	expectedVersion := item.Version
	item.EmptyProgressCount = 0
	item.ContinuationCount = 0
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
	if item.EmptyProgressCount == 0 && strings.TrimSpace(item.LastError) == "" {
		return item, nil
	}
	expectedVersion := item.Version
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
	s.markWallClockGoalActive(*updated)
	return updated, nil
}

func (s *Service) noteEmptyContinuationProgress(ctx context.Context, item *protocol.Goal, roundID string) (*protocol.Goal, error) {
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
		"reason":               "goal continuation produced no counted tool progress",
	}
	if err := s.appendEvent(ctx, *updated, "continuation_suppressed", protocol.GoalUpdateSourceSystem, roundID, payload); err != nil {
		return nil, err
	}
	s.clearWallClockGoal(*updated)
	return updated, nil
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
