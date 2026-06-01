package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) changeStatus(
	ctx context.Context,
	goalID string,
	status protocol.GoalStatus,
	source protocol.GoalUpdateSource,
	eventType string,
	roundID string,
	payload map[string]any,
) (*protocol.Goal, error) {
	if source == protocol.GoalUpdateSourceModel {
		ctx = withBudgetLimitSteeringSuppressed(ctx)
	}
	s.prepareExternalMutation(ctx, strings.TrimSpace(goalID))
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	return s.persistTransition(ctx, *item, status, source, eventType, roundID, payload)
}

func (s *Service) loadMutableGoal(ctx context.Context, goalID string) (*protocol.Goal, error) {
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
	if !protocol.IsCurrentGoalStatus(item.Status) {
		return nil, ErrGoalInvalidState
	}
	return item, nil
}

func (s *Service) persistTransition(
	ctx context.Context,
	item protocol.Goal,
	status protocol.GoalStatus,
	source protocol.GoalUpdateSource,
	eventType string,
	roundID string,
	payload map[string]any,
) (*protocol.Goal, error) {
	return s.persistTransitionWithOptions(ctx, item, status, source, eventType, roundID, payload, transitionOptions{})
}

type transitionOptions struct {
	persistBudgetLimitedStopRequest bool
}

func (s *Service) persistTransitionWithOptions(
	ctx context.Context,
	item protocol.Goal,
	status protocol.GoalStatus,
	source protocol.GoalUpdateSource,
	eventType string,
	roundID string,
	payload map[string]any,
	options transitionOptions,
) (*protocol.Goal, error) {
	status = protocol.NormalizeGoalStatus(status)
	if shouldPreserveBudgetLimitedStopRequest(item.Status, status) {
		if !options.persistBudgetLimitedStopRequest {
			s.clearWallClockGoal(item)
			if shouldClearAccountingAfterMutation(source) {
				s.clearExternalGoalAccounting(item)
			}
			return &item, nil
		}
		status = protocol.NormalizeGoalStatus(item.Status)
	}
	if !canTransition(source, item.Status, status) {
		return nil, ErrGoalInvalidState
	}
	expectedVersion := item.Version
	now := s.nowFn()
	item.Status = status
	if resetEmptyProgressForTransition(source, status) {
		item.EmptyProgressCount = 0
	}
	if resetContinuationCountForTransition(source, status) {
		item.ContinuationCount = 0
	}
	item.Version++
	item.UpdatedAt = now
	switch status {
	case protocol.GoalStatusActive:
		item.LastError = ""
		item.CompletedAt = nil
		item.BlockedAt = nil
	case protocol.GoalStatusComplete:
		item.CompletedAt = &now
	case protocol.GoalStatusBlocked:
		item.BlockedAt = &now
	}
	updated, err := s.repo.UpdateGoal(ctx, item, expectedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGoalVersionStale
	}
	if err != nil {
		return nil, err
	}
	if err := s.appendEvent(ctx, *updated, eventType, source, roundID, payload); err != nil {
		return nil, err
	}
	if protocol.NormalizeGoalStatus(updated.Status) == protocol.GoalStatusActive {
		if source == protocol.GoalUpdateSourceModel {
			s.markWallClockGoalActive(*updated)
		} else {
			s.activateExternalGoalAccounting(ctx, *updated)
		}
	} else {
		s.clearWallClockGoal(*updated)
		if shouldClearAccountingAfterMutation(source) {
			s.clearExternalGoalAccounting(*updated)
		}
	}
	return updated, nil
}

func statusAfterUserGoalUpdate(status protocol.GoalStatus, objectiveUpdated bool) protocol.GoalStatus {
	normalized := protocol.NormalizeGoalStatus(status)
	if !objectiveUpdated {
		return normalized
	}
	switch normalized {
	case protocol.GoalStatusBudgetLimited, protocol.GoalStatusComplete:
		return protocol.GoalStatusActive
	default:
		return normalized
	}
}
