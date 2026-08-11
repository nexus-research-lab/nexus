// INPUT: 已完成 terminal drain 的 Goal ID、尚未持久化的最终 usage 增量与 round ID。
// OUTPUT: 按 ID 查询的聚合 usage，以及单事务冻结后的 Goal 和 usage_finalized 事件。
// POS: runtime terminal settlement 到 Goal 持久/event fence 的服务边界。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type usageFinalizationErrorClassifier interface {
	IsGoalUsageUnavailable(error) bool
}

// UsageByGoalID 返回指定 Goal 的聚合 usage；完成后不依赖 current Goal 查询。
func (s *Service) UsageByGoalID(ctx context.Context, goalID string) (*protocol.GoalUsageReport, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		return nil, ErrGoalInvalidInput
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	report := item.UsageReport()
	return &report, nil
}

// UsageByGoalIDForOwner returns final usage only to the Goal's authenticated
// owner. The unscoped variant remains reserved for trusted in-process flows.
func (s *Service) UsageByGoalIDForOwner(
	ctx context.Context,
	goalID string,
	ownerUserID string,
) (*protocol.GoalUsageReport, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		return nil, ErrGoalInvalidInput
	}
	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	item, err = s.authorizeOwnerScopedGoal(ctx, item, ownerUserID)
	if err != nil {
		return nil, err
	}
	report := item.UsageReport()
	return &report, nil
}

// FinalizeUsageForGoal 在 parent terminal 与 child drain 全部完成后，
// 原子写入尚未持久化的最终增量，并冻结该 Goal 的 usage。
func (s *Service) FinalizeUsageForGoal(
	ctx context.Context,
	goalID string,
	finalDelta protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		return nil, ErrGoalInvalidInput
	}
	finalDelta = finalDelta.NormalizeTotals()

	item, err := s.repo.GetGoal(ctx, goalID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrGoalNotFound
	}
	for attempt := 0; attempt < goalUpdateMaxAttempts; attempt++ {
		if item.UsageFinalized {
			if isGoalUsageZero(finalDelta) {
				return item, nil
			}
			return nil, ErrGoalInvalidState
		}
		if !protocol.IsGoalUsageFinalizableStatus(item.Status) {
			return nil, ErrGoalInvalidState
		}

		expectedVersion := item.Version
		finalizedAt := s.nowFn().UTC()
		item.Usage = item.Usage.Add(finalDelta)
		item.TimeUsedSeconds += finalDelta.RuntimeSeconds
		item.UsageFinalized = true
		item.UsageFinalizedAt = &finalizedAt
		item.Version++
		item.UpdatedAt = finalizedAt

		event := s.newGoalEvent(
			*item,
			"usage_finalized",
			protocol.GoalUpdateSourceSystem,
			roundID,
			map[string]any{
				"final_usage":        item.Usage,
				"usage_delta":        finalDelta,
				"usage_finalized":    true,
				"usage_finalized_at": finalizedAt,
			},
			finalizedAt,
		)
		updated, finalizeErr := s.repo.FinalizeGoalUsage(ctx, *item, expectedVersion, event)
		if !errors.Is(finalizeErr, sql.ErrNoRows) {
			if finalizeErr != nil {
				if classifier, ok := s.repo.(usageFinalizationErrorClassifier); ok &&
					classifier.IsGoalUsageUnavailable(finalizeErr) {
					return nil, fmt.Errorf("%w: %v", ErrGoalUsageUnavailable, finalizeErr)
				}
				return nil, finalizeErr
			}
			s.publishGoalEvent(ctx, *updated, event)
			return updated, nil
		}
		reloaded, reloadErr := s.repo.GetGoal(ctx, goalID)
		if reloadErr != nil {
			return nil, reloadErr
		}
		if reloaded == nil {
			return nil, ErrGoalNotFound
		}
		item = reloaded
	}
	return nil, ErrGoalVersionStale
}
