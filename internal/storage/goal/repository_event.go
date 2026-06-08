package goal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// AppendEvent 写入 Goal 审计事件。
func (r *Repository) AppendEvent(ctx context.Context, event protocol.GoalEvent) error {
	query := fmt.Sprintf(`INSERT INTO goal_events (
    event_id,
    goal_id,
    session_key,
    event_type,
    source,
    round_id,
    payload_json,
    created_at
) VALUES (%s)`, r.bindList(8))
	_, err := r.db.ExecContext(
		ctx,
		query,
		event.ID,
		event.GoalID,
		event.SessionKey,
		event.EventType,
		event.Source,
		nullString(event.RoundID),
		marshalMap(event.Payload),
		event.CreatedAt.UTC(),
	)
	return err
}

// ListEvents 读取 Goal 审计事件。
func (r *Repository) ListEvents(ctx context.Context, goalID string, limit int) ([]protocol.GoalEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT
    event_id,
    goal_id,
    session_key,
    event_type,
    source,
    round_id,
    payload_json,
    created_at
FROM goal_events
WHERE goal_id = ` + r.bind(1) + `
ORDER BY created_at DESC, event_id DESC
LIMIT ` + r.bind(2)
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(goalID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]protocol.GoalEvent, 0)
	for rows.Next() {
		item, scanErr := scanGoalEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return items, nil
}
