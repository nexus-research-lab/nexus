// INPUT: 已进入可结算终态的 Goal、最终 usage 增量与 usage_finalized 审计事件。
// OUTPUT: 单事务提交的最终聚合 usage、持久 fence 和事件。
// POS: terminal drain 完成后冻结 Goal usage 的 SQL 事务边界。
package goal

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// FinalizeGoalUsage 原子写入最终 usage、持久 fence 与审计事件。
func (r *Repository) FinalizeGoalUsage(
	ctx context.Context,
	goal protocol.Goal,
	expectedVersion int64,
	event protocol.GoalEvent,
) (*protocol.Goal, error) {
	if !goal.UsageFinalized || goal.UsageFinalizedAt == nil {
		return nil, fmt.Errorf("goal usage finalization requires a finalized goal")
	}
	goal.Usage = goal.Usage.NormalizeTotals()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := r.lockUsageSourceGoal(ctx, tx, goal.ID); err != nil {
		return nil, err
	}
	if err := r.rejectGoalUsageFinalizationWithPending(ctx, tx, goal.ID); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`UPDATE session_goals
SET token_used_input = %s,
    token_used_output = %s,
    token_used_cache_creation = %s,
    token_used_cache_read = %s,
    token_used_reasoning = %s,
    token_used_total = %s,
    token_used_actual_total = %s,
    token_used_actual_estimated = %s,
    time_used_seconds = %s,
    usage_finalized = %s,
    usage_finalized_at = %s,
    version = %s,
    updated_at = %s
WHERE goal_id = %s
  AND version = %s
  AND usage_finalized = %s
  AND status = 'complete'`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8),
		r.bind(9), r.bind(10), r.bind(11), r.bind(12), r.bind(13), r.bind(14), r.bind(15), r.bind(16),
	)
	result, err := tx.ExecContext(
		ctx,
		query,
		goal.Usage.InputTokens,
		goal.Usage.OutputTokens,
		goal.Usage.CacheCreationInputTokens,
		goal.Usage.CacheReadInputTokens,
		goal.Usage.ReasoningTokens,
		goal.Usage.TotalTokens,
		goal.Usage.ActualTotalTokens,
		goal.Usage.ActualTokensEstimated,
		goal.TimeUsedSeconds,
		true,
		goal.UsageFinalizedAt.UTC(),
		goal.Version,
		goal.UpdatedAt.UTC(),
		goal.ID,
		expectedVersion,
		false,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return nil, sql.ErrNoRows
	}
	if err := r.insertGoalEvent(ctx, tx, event); err != nil {
		return nil, err
	}
	if err := r.cancelGoalContinuations(ctx, tx, goal.ID, 0, "Goal usage finalized", goal.UpdatedAt); err != nil {
		return nil, err
	}
	updated, err := scanGoal(tx.QueryRowContext(
		ctx,
		goalSelectQuery("goal_id = "+r.bind(1)),
		goal.ID,
	))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}
