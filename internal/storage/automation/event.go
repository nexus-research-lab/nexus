// INPUT: scheduled task 写入的主会话触发事件与派发器的领取/收口请求。
// OUTPUT: 按 Agent 有序列出 new 事件、单行原子领取、终态标记与待恢复 Agent 集合。
// POS: automation_system_events 的唯一仓储入口；领取以 status CAS 保证跨实例只消费一次。
package automation

import (
	"context"
	"encoding/json"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
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

// ListNewSystemEventsByAgent 按写入顺序列出指定 Agent 尚未领取的系统事件。
func (r *Repository) ListNewSystemEventsByAgent(ctx context.Context, agentID string) ([]automationdomain.SystemEvent, error) {
	query := `SELECT event_id, event_type, source_type, source_id, payload, status, created_at
FROM automation_system_events
WHERE status = 'new'`
	if r.isPostgres {
		query += ` AND (source_id = ` + r.bind(1) + ` OR payload::jsonb->>'agent_id' = ` + r.bind(2) + `)`
	} else {
		query += ` AND (source_id = ` + r.bind(1) + ` OR json_extract(payload, '$.agent_id') = ` + r.bind(2) + `)`
	}
	query += ` ORDER BY created_at ASC, event_id ASC`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(agentID), strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]automationdomain.SystemEvent, 0)
	for rows.Next() {
		var item automationdomain.SystemEvent
		if err = rows.Scan(
			&item.EventID, &item.EventType, &item.SourceType, &item.SourceID,
			&item.Payload, &item.Status, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListAgentIDsWithNewSystemEvents 返回仍有未领取事件的 Agent，供重启或漏唤醒后恢复派发。
func (r *Repository) ListAgentIDsWithNewSystemEvents(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT source_id
FROM automation_system_events
WHERE status = 'new'
ORDER BY source_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var agentID string
		if err = rows.Scan(&agentID); err != nil {
			return nil, err
		}
		if agentID = strings.TrimSpace(agentID); agentID != "" {
			items = append(items, agentID)
		}
	}
	return items, rows.Err()
}

// ClaimSystemEvent 原子领取 new 事件，避免两个实例同时消费同一行。
func (r *Repository) ClaimSystemEvent(ctx context.Context, eventID string) (bool, error) {
	result, err := r.execWithRetry(ctx, `UPDATE automation_system_events
SET status = 'processing', updated_at = CURRENT_TIMESTAMP
WHERE event_id = `+r.bind(1)+` AND status = 'new'`, strings.TrimSpace(eventID))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// MarkSystemEventStatus 更新系统事件状态。
func (r *Repository) MarkSystemEventStatus(ctx context.Context, eventID string, status string) error {
	_, err := r.execWithRetry(ctx, r.markSystemEventStatusQuery, status, status, eventID)
	return err
}
