// INPUT: Goal 当前状态、目标状态、来源与可选 objective revision。
// OUTPUT: CAS 持久化后的状态迁移和审计事件。
// POS: Goal 状态机持久化的唯一入口。
package goal

import (
	"context"
	"strings"
	"time"

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
	expectedRevision ...int64,
) (*protocol.Goal, error) {
	if source == protocol.GoalUpdateSourceModel {
		ctx = withBudgetLimitSteeringSuppressed(ctx)
	}
	prepare := s.prepareExternalMutation
	if externalStatusUsesSettlementBoundary(source, status) {
		prepare = s.prepareExternalMutationAtSettlementBoundary
	}
	if err := prepare(ctx, strings.TrimSpace(goalID)); err != nil {
		return nil, err
	}
	item, err := s.loadMutableGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if !objectiveRevisionMatches(*item, firstExpectedObjectiveRevision(expectedRevision)) {
		return nil, ErrGoalRevisionStale
	}
	return s.persistTransition(ctx, *item, status, source, eventType, roundID, payload)
}

func externalStatusUsesSettlementBoundary(
	source protocol.GoalUpdateSource,
	status protocol.GoalStatus,
) bool {
	if source != protocol.GoalUpdateSourceUser &&
		source != protocol.GoalUpdateSourceExternal {
		return false
	}
	switch protocol.NormalizeGoalStatus(status) {
	case protocol.GoalStatusBlocked,
		protocol.GoalStatusUsageLimited:
		return true
	default:
		// complete 保留绑定到 parent terminal，以 provider 累计真值和 child drain
		// 建立最终 fence；active 变更仍属于同一个 Goal。
		return false
	}
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
			if source == protocol.GoalUpdateSourceUser ||
				source == protocol.GoalUpdateSourceExternal {
				if err := s.prepareExternalMutationAtSettlementBoundary(ctx, item.ID); err != nil {
					return nil, err
				}
				if err := s.clearExternalGoalAccounting(ctx, item); err != nil {
					return nil, err
				}
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
	if resetSuppressionForActiveTransition(source, status) {
		item.EmptyProgressCount = 0
		item.Metadata = clearContinuationReservations(clearCompletionCommandRetryMetadata(item.Metadata))
	}
	item.Version++
	item.UpdatedAt = now
	switch status {
	case protocol.GoalStatusActive:
		item.LastError = ""
		item.CompletedAt = nil
		item.BlockedAt = nil
		item.Metadata = clearGoalBlockerMetadata(item.Metadata)
	case protocol.GoalStatusComplete:
		item.CompletedAt = &now
	case protocol.GoalStatusBlocked:
		item.BlockedAt = &now
		item.Metadata = storeGoalBlockerMetadata(
			item.Metadata,
			payload,
			item.ObjectiveRevision(),
			now,
		)
	}
	updated, err := s.persistGoalUpdateWithEvent(
		ctx,
		item,
		expectedVersion,
		eventType,
		source,
		roundID,
		payload,
	)
	if err != nil {
		return nil, err
	}
	if protocol.NormalizeGoalStatus(updated.Status) == protocol.GoalStatusActive {
		if source == protocol.GoalUpdateSourceModel {
			s.markWallClockGoalActive(*updated)
		} else {
			if err := s.activateExternalGoalAccounting(ctx, *updated); err != nil {
				return nil, err
			}
		}
	} else {
		s.clearWallClockGoal(*updated)
		if source == protocol.GoalUpdateSourceUser ||
			source == protocol.GoalUpdateSourceExternal {
			if err := s.clearExternalGoalAccounting(ctx, *updated); err != nil {
				return nil, err
			}
			if protocol.NormalizeGoalStatus(updated.Status) == protocol.GoalStatusComplete {
				refreshed, refreshErr := s.repo.GetGoal(ctx, updated.ID)
				if refreshErr != nil {
					return nil, refreshErr
				}
				if refreshed != nil {
					updated = refreshed
				}
			}
		}
	}
	return updated, nil
}

func storeGoalBlockerMetadata(
	metadata map[string]any,
	payload map[string]any,
	objectiveRevision int64,
	at time.Time,
) map[string]any {
	metadata = cloneMap(metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata[protocol.GoalMetadataBlocker] = map[string]any{
		"id":             strings.TrimSpace(protocol.GoalMetadataString(payload, "blocker_id")),
		"reason":         strings.TrimSpace(protocol.GoalMetadataString(payload, "reason")),
		"needed_input":   strings.TrimSpace(protocol.GoalMetadataString(payload, "needed_input")),
		"since_revision": objectiveRevision,
		"blocked_at":     at.UTC().Format(time.RFC3339Nano),
	}
	return metadata
}

func clearGoalBlockerMetadata(metadata map[string]any) map[string]any {
	metadata = cloneMap(metadata)
	delete(metadata, protocol.GoalMetadataBlocker)
	return metadata
}

func statusAfterUserGoalUpdate(status protocol.GoalStatus, objectiveUpdated bool) protocol.GoalStatus {
	normalized := protocol.NormalizeGoalStatus(status)
	if !objectiveUpdated {
		return normalized
	}
	return protocol.GoalStatusActive
}

func canTransition(source protocol.GoalUpdateSource, from protocol.GoalStatus, to protocol.GoalStatus) bool {
	from = protocol.NormalizeGoalStatus(from)
	to = protocol.NormalizeGoalStatus(to)
	if from == to {
		return true
	}
	switch source {
	case protocol.GoalUpdateSourceModel:
		if to == protocol.GoalStatusActive {
			return from == protocol.GoalStatusPaused ||
				from == protocol.GoalStatusBlocked ||
				from == protocol.GoalStatusUsageLimited
		}
		return (from == protocol.GoalStatusActive || from == protocol.GoalStatusBudgetLimited) &&
			(to == protocol.GoalStatusComplete || to == protocol.GoalStatusBlocked)
	case protocol.GoalUpdateSourceSystem:
		if from == protocol.GoalStatusBudgetLimited && to == protocol.GoalStatusUsageLimited {
			return true
		}
		if from != protocol.GoalStatusActive {
			return false
		}
		return to == protocol.GoalStatusBlocked ||
			to == protocol.GoalStatusComplete ||
			to == protocol.GoalStatusBudgetLimited ||
			to == protocol.GoalStatusUsageLimited
	case protocol.GoalUpdateSourceExternal:
		return canExternalTransition(from, to)
	default:
		return canUserTransition(from, to)
	}
}

func shouldPreserveBudgetLimitedStopRequest(from protocol.GoalStatus, to protocol.GoalStatus) bool {
	from = protocol.NormalizeGoalStatus(from)
	to = protocol.NormalizeGoalStatus(to)
	return from == protocol.GoalStatusBudgetLimited &&
		(to == protocol.GoalStatusPaused || to == protocol.GoalStatusBlocked)
}

func canExternalTransition(_ protocol.GoalStatus, to protocol.GoalStatus) bool {
	return to == protocol.GoalStatusActive ||
		to == protocol.GoalStatusPaused ||
		to == protocol.GoalStatusBlocked ||
		to == protocol.GoalStatusBudgetLimited ||
		to == protocol.GoalStatusUsageLimited ||
		to == protocol.GoalStatusComplete
}

func canUserTransition(from protocol.GoalStatus, to protocol.GoalStatus) bool {
	switch from {
	case protocol.GoalStatusActive:
		return to == protocol.GoalStatusPaused || to == protocol.GoalStatusComplete || to == protocol.GoalStatusBlocked
	case protocol.GoalStatusPaused, protocol.GoalStatusBlocked:
		return to == protocol.GoalStatusActive
	case protocol.GoalStatusBudgetLimited, protocol.GoalStatusUsageLimited:
		return to == protocol.GoalStatusActive ||
			to == protocol.GoalStatusPaused ||
			to == protocol.GoalStatusComplete ||
			to == protocol.GoalStatusBlocked
	case protocol.GoalStatusComplete:
		return to == protocol.GoalStatusActive
	default:
		return false
	}
}
