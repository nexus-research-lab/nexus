// INPUT: Goal 领域对象、optimistic version 与 SQL 方言。
// OUTPUT: session_goals 的持久化读写结果。
// POS: Goal service 与关系数据库之间的仓储边界。
package goal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Repository 封装 Goal 领域 SQL 读写。
type Repository struct {
	db         *sql.DB
	isPostgres bool
}

// NewRepository 创建 Goal SQL 仓储。
func NewRepository(cfg config.Config, db *sql.DB) *Repository {
	return &Repository{
		db:         db,
		isPostgres: storage.NormalizeSQLDriver(cfg.DatabaseDriver) == "pgx",
	}
}

// CreateGoal 创建 Goal。
func (r *Repository) CreateGoal(ctx context.Context, goal protocol.Goal) (*protocol.Goal, error) {
	goal.Usage = goal.Usage.NormalizeTotals()
	if err := r.insertGoal(ctx, r.db, goal); err != nil {
		return nil, err
	}
	return r.GetGoal(ctx, goal.ID)
}

func (r *Repository) insertGoal(
	ctx context.Context,
	executor goalEventExecutor,
	goal protocol.Goal,
) error {
	goal.Usage = goal.Usage.NormalizeTotals()
	query := fmt.Sprintf(`INSERT INTO session_goals (
    goal_id,
    session_key,
    objective,
    status,
    token_budget,
    token_used_input,
    token_used_output,
    token_used_cache_creation,
    token_used_cache_read,
    token_used_reasoning,
    token_used_total,
    token_used_actual_total,
    token_used_actual_estimated,
    usage_finalized,
    usage_finalized_at,
    time_used_seconds,
    continuation_count,
    empty_progress_count,
    version,
    created_by,
    created_at,
    updated_at,
    completed_at,
    blocked_at,
    last_error,
    metadata_json
) VALUES (%s)`, r.bindList(26))
	_, err := executor.ExecContext(
		ctx,
		query,
		goal.ID,
		goal.SessionKey,
		goal.Objective,
		goal.Status,
		nullInt64Pointer(goal.TokenBudget),
		goal.Usage.InputTokens,
		goal.Usage.OutputTokens,
		goal.Usage.CacheCreationInputTokens,
		goal.Usage.CacheReadInputTokens,
		goal.Usage.ReasoningTokens,
		goal.Usage.TotalTokens,
		goal.Usage.ActualTotalTokens,
		goal.Usage.ActualTokensEstimated,
		goal.UsageFinalized,
		nullableTime(goal.UsageFinalizedAt),
		goal.TimeUsedSeconds,
		goal.ContinuationCount,
		goal.EmptyProgressCount,
		goal.Version,
		nullString(goal.CreatedBy),
		goal.CreatedAt.UTC(),
		goal.UpdatedAt.UTC(),
		nullableTime(goal.CompletedAt),
		nullableTime(goal.BlockedAt),
		nullString(goal.LastError),
		marshalMap(goal.Metadata),
	)
	return err
}

// GetGoal 读取指定 Goal。
func (r *Repository) GetGoal(ctx context.Context, goalID string) (*protocol.Goal, error) {
	row := r.db.QueryRowContext(ctx, goalSelectQuery("goal_id = "+r.bind(1)), strings.TrimSpace(goalID))
	goal, err := scanGoal(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

// GetCurrentGoal 读取 session 当前 Goal。
func (r *Repository) GetCurrentGoal(ctx context.Context, sessionKey string) (*protocol.Goal, error) {
	query := goalSelectQuery("session_key = " + r.bind(1) + " AND status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited')")
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(sessionKey))
	goal, err := scanGoal(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &goal, nil
}

// ListGoals 返回全部 Goal，用于资源删除时按结构化 session_key 做级联清理。
func (r *Repository) ListGoals(ctx context.Context) ([]protocol.Goal, error) {
	return r.listGoalsWhere(ctx, "1 = 1")
}

// ListCurrentGoals 返回每个 session 当前未终结的 Goal，用于启动投影恢复。
func (r *Repository) ListCurrentGoals(ctx context.Context) ([]protocol.Goal, error) {
	return r.listGoalsWhere(ctx, "status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited')")
}

func (r *Repository) listGoalsWhere(ctx context.Context, predicate string) ([]protocol.Goal, error) {
	rows, err := r.db.QueryContext(ctx, goalSelectQuery(predicate+" ORDER BY updated_at ASC, goal_id ASC"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]protocol.Goal, 0)
	for rows.Next() {
		item, scanErr := scanGoal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ListRunnableGoals 只返回 continuation controller 可运行的 active Goal。
// suspended Goal 必须留给显式 activity/resume 解锁；若先 LIMIT 再在 service
// 过滤，它们会永久占满旧记录窗口并饿死较新的 ready Goal。
func (r *Repository) ListRunnableGoals(ctx context.Context, limit int) ([]protocol.Goal, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := goalSelectQuery(
		"status = " + r.bind(1) +
			" AND empty_progress_count < " + r.bind(2) +
			" AND TRIM(COALESCE(last_error, '')) = ''" +
			" ORDER BY updated_at ASC, goal_id ASC LIMIT " + r.bind(3),
	)
	rows, err := r.db.QueryContext(
		ctx,
		query,
		protocol.GoalStatusActive,
		protocol.GoalContinuationSuppressionThreshold,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]protocol.Goal, 0)
	for rows.Next() {
		item, scanErr := scanGoal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// UpdateGoal 以 optimistic version 更新 Goal。
func (r *Repository) UpdateGoal(ctx context.Context, goal protocol.Goal, expectedVersion int64) (*protocol.Goal, error) {
	goal.Usage = goal.Usage.NormalizeTotals()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = r.updateGoal(ctx, tx, goal, expectedVersion); err != nil {
		return nil, err
	}
	keepRevision := int64(0)
	if protocol.NormalizeGoalStatus(goal.Status) == protocol.GoalStatusActive {
		keepRevision = goal.ObjectiveRevision()
	}
	if err = r.cancelGoalContinuations(ctx, tx, goal.ID, keepRevision, "Goal lifecycle or objective revision advanced", goal.UpdatedAt); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &goal, nil
}

func (r *Repository) updateGoal(
	ctx context.Context,
	executor goalEventExecutor,
	goal protocol.Goal,
	expectedVersion int64,
) error {
	goal.Usage = goal.Usage.NormalizeTotals()
	query := fmt.Sprintf(`UPDATE session_goals
SET objective = %s,
    status = %s,
    token_budget = %s,
    token_used_input = %s,
    token_used_output = %s,
    token_used_cache_creation = %s,
    token_used_cache_read = %s,
    token_used_reasoning = %s,
    token_used_total = %s,
    token_used_actual_total = %s,
    token_used_actual_estimated = %s,
    usage_finalized = %s,
    usage_finalized_at = %s,
    time_used_seconds = %s,
    continuation_count = %s,
    empty_progress_count = %s,
    version = %s,
    updated_at = %s,
    completed_at = %s,
    blocked_at = %s,
    last_error = %s,
    metadata_json = %s
WHERE goal_id = %s AND version = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8), r.bind(9), r.bind(10),
		r.bind(11), r.bind(12), r.bind(13), r.bind(14), r.bind(15), r.bind(16), r.bind(17), r.bind(18), r.bind(19),
		r.bind(20), r.bind(21), r.bind(22), r.bind(23), r.bind(24),
	)
	result, err := executor.ExecContext(
		ctx,
		query,
		goal.Objective,
		goal.Status,
		nullInt64Pointer(goal.TokenBudget),
		goal.Usage.InputTokens,
		goal.Usage.OutputTokens,
		goal.Usage.CacheCreationInputTokens,
		goal.Usage.CacheReadInputTokens,
		goal.Usage.ReasoningTokens,
		goal.Usage.TotalTokens,
		goal.Usage.ActualTotalTokens,
		goal.Usage.ActualTokensEstimated,
		goal.UsageFinalized,
		nullableTime(goal.UsageFinalizedAt),
		goal.TimeUsedSeconds,
		goal.ContinuationCount,
		goal.EmptyProgressCount,
		goal.Version,
		goal.UpdatedAt.UTC(),
		nullableTime(goal.CompletedAt),
		nullableTime(goal.BlockedAt),
		nullString(goal.LastError),
		marshalMap(goal.Metadata),
		goal.ID,
		expectedVersion,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return err
}

// DeleteGoal 删除指定 Goal。
func (r *Repository) DeleteGoal(ctx context.Context, goalID string) (bool, error) {
	goalID = strings.TrimSpace(goalID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := r.lockUsageSourceGoal(ctx, tx, goalID); err == sql.ErrNoRows {
		if commitErr := tx.Commit(); commitErr != nil {
			return false, commitErr
		}
		return false, nil
	} else if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM goal_usage_source_pending
WHERE EXISTS (
    SELECT 1
    FROM goal_usage_scope_bindings
    WHERE goal_usage_scope_bindings.owner_user_id = goal_usage_source_pending.owner_user_id
      AND goal_usage_scope_bindings.goal_session_key = goal_usage_source_pending.goal_session_key
      AND goal_usage_scope_bindings.source_kind = goal_usage_source_pending.source_kind
      AND goal_usage_scope_bindings.scope_round_id = goal_usage_source_pending.scope_round_id
      AND goal_usage_scope_bindings.goal_id = `+r.bind(1)+`
)`,
		goalID,
	); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM goal_usage_source_evidence
WHERE EXISTS (
    SELECT 1
    FROM goal_usage_scope_bindings
    WHERE goal_usage_scope_bindings.owner_user_id = goal_usage_source_evidence.owner_user_id
      AND goal_usage_scope_bindings.goal_session_key = goal_usage_source_evidence.goal_session_key
      AND goal_usage_scope_bindings.source_kind = goal_usage_source_evidence.source_kind
      AND goal_usage_scope_bindings.scope_round_id = goal_usage_source_evidence.scope_round_id
      AND goal_usage_scope_bindings.goal_id = `+r.bind(1)+`
)`,
		goalID,
	); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM goal_usage_parent_ledger
WHERE attributed_goal_id = `+r.bind(1)+`
   OR EXISTS (
    SELECT 1
    FROM goal_usage_scope_bindings
    WHERE goal_usage_scope_bindings.owner_user_id = goal_usage_parent_ledger.owner_user_id
      AND goal_usage_scope_bindings.goal_session_key = goal_usage_parent_ledger.goal_session_key
      AND goal_usage_scope_bindings.scope_round_id = goal_usage_parent_ledger.scope_round_id
      AND goal_usage_scope_bindings.goal_id = `+r.bind(2)+`
)`,
		goalID,
		goalID,
	); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE goal_usage_scope_bindings
SET state = 'closed',
    goal_id = NULL,
    closed_at = CURRENT_TIMESTAMP
WHERE goal_id = `+r.bind(1),
		goalID,
	); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(
		ctx,
		"DELETE FROM goal_events WHERE goal_id = "+r.bind(1),
		goalID,
	); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(
		ctx,
		"DELETE FROM session_goals WHERE goal_id = "+r.bind(1),
		goalID,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *Repository) bind(position int) string {
	if r.isPostgres {
		return fmt.Sprintf("$%d", position)
	}
	return "?"
}

func (r *Repository) bindList(count int) string {
	values := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		values = append(values, r.bind(i))
	}
	return strings.Join(values, ",")
}
