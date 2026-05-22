package goal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateCheckpoint 写入 Goal 长程执行 checkpoint。
func (r *Repository) CreateCheckpoint(ctx context.Context, checkpoint protocol.GoalCheckpoint) (*protocol.GoalCheckpoint, error) {
	query := fmt.Sprintf(`INSERT INTO goal_checkpoints (
    checkpoint_id,
    goal_id,
    session_key,
    summary,
    continuation_count,
    usage_json,
    created_at
) VALUES (%s)`, r.bindList(7))
	_, err := r.db.ExecContext(
		ctx,
		query,
		checkpoint.ID,
		checkpoint.GoalID,
		checkpoint.SessionKey,
		checkpoint.Summary,
		checkpoint.ContinuationCount,
		marshalUsage(checkpoint.Usage),
		checkpoint.CreatedAt.UTC(),
	)
	if err != nil {
		return nil, err
	}
	return r.LatestCheckpoint(ctx, checkpoint.GoalID)
}

// LatestCheckpoint 返回 Goal 最新 checkpoint。
func (r *Repository) LatestCheckpoint(ctx context.Context, goalID string) (*protocol.GoalCheckpoint, error) {
	query := `SELECT
    checkpoint_id,
    goal_id,
    session_key,
    summary,
    continuation_count,
    usage_json,
    created_at
FROM goal_checkpoints
WHERE goal_id = ` + r.bind(1) + `
ORDER BY created_at DESC, checkpoint_id DESC
LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, strings.TrimSpace(goalID))
	item, err := scanGoalCheckpoint(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
