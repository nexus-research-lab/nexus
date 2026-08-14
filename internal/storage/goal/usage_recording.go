// INPUT: optimistic Goal usage aggregate 与同一增量产生的审计事件。
// OUTPUT: Goal usage/status/version 与 usage_recorded/budget_limited 事件的单事务提交结果。
// POS: parent runtime 两阶段 emission 的原子 SQL 落点。
package goal

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// RecordGoalUsage 原子更新 Goal usage，并写入该增量对应的全部审计事件。
func (r *Repository) RecordGoalUsage(
	ctx context.Context,
	goal protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	goal.Usage = goal.Usage.NormalizeTotals()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	query := fmt.Sprintf(`UPDATE session_goals
SET status = %s,
    token_used_input = %s,
    token_used_output = %s,
    token_used_cache_creation = %s,
    token_used_cache_read = %s,
    token_used_reasoning = %s,
    token_used_total = %s,
    token_used_actual_total = %s,
    token_used_actual_estimated = %s,
    time_used_seconds = %s,
    version = %s,
    updated_at = %s,
    last_error = %s
WHERE goal_id = %s
  AND version = %s
  AND usage_finalized = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8),
		r.bind(9), r.bind(10), r.bind(11), r.bind(12), r.bind(13), r.bind(14), r.bind(15), r.bind(16),
	)
	result, err := tx.ExecContext(
		ctx,
		query,
		goal.Status,
		goal.Usage.InputTokens,
		goal.Usage.OutputTokens,
		goal.Usage.CacheCreationInputTokens,
		goal.Usage.CacheReadInputTokens,
		goal.Usage.ReasoningTokens,
		goal.Usage.TotalTokens,
		goal.Usage.ActualTotalTokens,
		goal.Usage.ActualTokensEstimated,
		goal.TimeUsedSeconds,
		goal.Version,
		goal.UpdatedAt.UTC(),
		nullString(goal.LastError),
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
	if err != nil {
		return nil, err
	}
	keepRevision := int64(0)
	if protocol.NormalizeGoalStatus(goal.Status) == protocol.GoalStatusActive {
		keepRevision = goal.ObjectiveRevision()
	}
	if err := r.cancelGoalContinuations(ctx, tx, goal.ID, keepRevision, "Goal lifecycle or objective revision advanced", goal.UpdatedAt); err != nil {
		return nil, err
	}
	for _, event := range events {
		if err := r.insertGoalEvent(ctx, tx, event); err != nil {
			return nil, err
		}
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
