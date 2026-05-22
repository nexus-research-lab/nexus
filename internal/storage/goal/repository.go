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
    time_used_seconds,
    continuation_count,
    empty_progress_count,
    version,
    created_by,
    created_at,
    updated_at,
    completed_at,
    blocked_at,
    cleared_at,
    last_error,
    metadata_json
) VALUES (%s)`, r.bindList(23))
	_, err := r.db.ExecContext(
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
		goal.TimeUsedSeconds,
		goal.ContinuationCount,
		goal.EmptyProgressCount,
		goal.Version,
		nullString(goal.CreatedBy),
		goal.CreatedAt.UTC(),
		goal.UpdatedAt.UTC(),
		nullableTime(goal.CompletedAt),
		nullableTime(goal.BlockedAt),
		nullableTime(goal.ClearedAt),
		nullString(goal.LastError),
		marshalMap(goal.Metadata),
	)
	if err != nil {
		return nil, err
	}
	return r.GetGoal(ctx, goal.ID)
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
	query := goalSelectQuery("session_key = " + r.bind(1) + " AND status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited', 'complete')")
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

// ListRunnableGoals 返回需要系统继续推进的 active Goal。
func (r *Repository) ListRunnableGoals(ctx context.Context, limit int) ([]protocol.Goal, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := goalSelectQuery("status = " + r.bind(1) + " ORDER BY updated_at ASC, goal_id ASC LIMIT " + r.bind(2))
	rows, err := r.db.QueryContext(ctx, query, protocol.GoalStatusActive, limit)
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
    time_used_seconds = %s,
    continuation_count = %s,
    empty_progress_count = %s,
    version = %s,
    updated_at = %s,
    completed_at = %s,
    blocked_at = %s,
    cleared_at = %s,
    last_error = %s,
    metadata_json = %s
WHERE goal_id = %s AND version = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8), r.bind(9), r.bind(10),
		r.bind(11), r.bind(12), r.bind(13), r.bind(14), r.bind(15), r.bind(16), r.bind(17), r.bind(18), r.bind(19), r.bind(20),
		r.bind(21),
	)
	result, err := r.db.ExecContext(
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
		goal.TimeUsedSeconds,
		goal.ContinuationCount,
		goal.EmptyProgressCount,
		goal.Version,
		goal.UpdatedAt.UTC(),
		nullableTime(goal.CompletedAt),
		nullableTime(goal.BlockedAt),
		nullableTime(goal.ClearedAt),
		nullString(goal.LastError),
		marshalMap(goal.Metadata),
		goal.ID,
		expectedVersion,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return nil, sql.ErrNoRows
	}
	return r.GetGoal(ctx, goal.ID)
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
