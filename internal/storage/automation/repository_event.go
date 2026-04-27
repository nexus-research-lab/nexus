package automation

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// InsertSystemEvent 写入系统事件。
func (r *Repository) InsertSystemEvent(ctx context.Context, eventID string, eventType string, sourceType string, sourceID string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = r.execWithRetry(ctx, r.insertSystemEventQuery, eventID, eventType, sourceType, sourceID, string(body), "new")
	return err
}

// ListNewSystemEventsByAgent 列出指定 agent 尚未消费的系统事件。
func (r *Repository) ListNewSystemEventsByAgent(ctx context.Context, agentID string) ([]protocol.SystemEvent, error) {
	query := `
SELECT
    event_id,
    event_type,
    source_type,
    source_id,
    payload,
    status,
    created_at
FROM automation_system_events
WHERE status = 'new'`
	if r.isPostgres {
		query += ` AND (source_id = ` + r.bind(1) + ` OR payload::jsonb->>'agent_id' = ` + r.bind(2) + `)`
	} else {
		query += ` AND (source_id = ` + r.bind(1) + ` OR json_extract(payload, '$.agent_id') = ` + r.bind(2) + `)`
	}
	query += `
ORDER BY created_at ASC, event_id ASC`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(agentID), strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]protocol.SystemEvent, 0)
	for rows.Next() {
		var item protocol.SystemEvent
		if scanErr := rows.Scan(
			&item.EventID,
			&item.EventType,
			&item.SourceType,
			&item.SourceID,
			&item.Payload,
			&item.Status,
			&item.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// MarkSystemEventStatus 更新系统事件状态。
func (r *Repository) MarkSystemEventStatus(ctx context.Context, eventID string, status string) error {
	_, err := r.execWithRetry(ctx, r.markSystemEventStatusQuery, status, status, eventID)
	return err
}
