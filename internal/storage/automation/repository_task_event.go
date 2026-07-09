package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// TaskEventInput 表示一条定时任务管理动作审计输入。
type TaskEventInput struct {
	EventID      string
	JobID        string
	OwnerUserID  string
	AgentID      string
	Action       string
	ActorUserID  string
	ActorAgentID string
	RunID        string
	Detail       map[string]any
}

// InsertTaskEvent 写入定时任务管理动作审计记录。
func (r *Repository) InsertTaskEvent(ctx context.Context, input TaskEventInput) error {
	detail := input.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	body, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	query := `INSERT INTO automation_task_events (
    event_id,
    job_id,
    owner_user_id,
    agent_id,
    action,
    actor_user_id,
    actor_agent_id,
    run_id,
    detail_json,
    created_at
) VALUES (` + r.bindList(9) + `, CURRENT_TIMESTAMP)`
	_, err = r.execWithRetry(
		ctx,
		query,
		strings.TrimSpace(input.EventID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.AgentID),
		strings.TrimSpace(input.Action),
		nullString(strings.TrimSpace(input.ActorUserID)),
		nullString(strings.TrimSpace(input.ActorAgentID)),
		nullString(strings.TrimSpace(input.RunID)),
		string(body),
	)
	return err
}

// ListTaskEventsByJob 返回指定任务的最近管理动作。
func (r *Repository) ListTaskEventsByJob(ctx context.Context, ownerUserID string, jobID string, limit int) ([]automationdomain.CronTaskEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
SELECT
    event_id,
    job_id,
    owner_user_id,
    agent_id,
    action,
    actor_user_id,
    actor_agent_id,
    run_id,
    detail_json,
    created_at
FROM automation_task_events
WHERE job_id = ` + r.bind(1)
	args := []any{strings.TrimSpace(jobID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	args = append(args, limit)
	query += " ORDER BY created_at DESC, event_id DESC LIMIT " + r.bind(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]automationdomain.CronTaskEvent, 0)
	for rows.Next() {
		item, scanErr := scanTaskEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// SearchTaskEvents 返回最近的任务管理事件，用于按名称/关键词定位历史任务。
func (r *Repository) SearchTaskEvents(ctx context.Context, ownerUserID string, agentID string, queryText string, limit int) ([]automationdomain.CronTaskEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
SELECT
    event_id,
    job_id,
    owner_user_id,
    agent_id,
    action,
    actor_user_id,
    actor_agent_id,
    run_id,
    detail_json,
    created_at
FROM automation_task_events`
	args := []any{}
	conditions := make([]string, 0, 3)
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		conditions = append(conditions, "owner_user_id = "+r.bind(len(args)))
	}
	if strings.TrimSpace(agentID) != "" {
		args = append(args, strings.TrimSpace(agentID))
		conditions = append(conditions, "agent_id = "+r.bind(len(args)))
	}
	if strings.TrimSpace(queryText) != "" {
		like := "%" + strings.ToLower(strings.TrimSpace(queryText)) + "%"
		queryConditions := make([]string, 0, 4)
		for _, column := range []string{"job_id", "agent_id", "action", "detail_json"} {
			args = append(args, like)
			queryConditions = append(queryConditions, "LOWER("+column+") LIKE "+r.bind(len(args)))
		}
		conditions = append(conditions, "("+strings.Join(queryConditions, " OR ")+")")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	args = append(args, limit)
	query += " ORDER BY created_at DESC, event_id DESC LIMIT " + r.bind(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]automationdomain.CronTaskEvent, 0)
	for rows.Next() {
		item, scanErr := scanTaskEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanTaskEvent(scanner interface {
	Scan(dest ...any) error
}) (automationdomain.CronTaskEvent, error) {
	var item automationdomain.CronTaskEvent
	var actorUserID sql.NullString
	var actorAgentID sql.NullString
	var runID sql.NullString
	var detailJSON string
	if err := scanner.Scan(
		&item.EventID,
		&item.JobID,
		&item.OwnerUserID,
		&item.AgentID,
		&item.Action,
		&actorUserID,
		&actorAgentID,
		&runID,
		&detailJSON,
		&item.CreatedAt,
	); err != nil {
		return automationdomain.CronTaskEvent{}, err
	}
	item.ActorUserID = nullStringValue(actorUserID)
	item.ActorAgentID = nullStringValue(actorAgentID)
	item.RunID = nullStringValue(runID)
	if strings.TrimSpace(detailJSON) != "" {
		var detail map[string]any
		if err := json.Unmarshal([]byte(detailJSON), &detail); err != nil {
			return automationdomain.CronTaskEvent{}, err
		}
		item.Detail = detail
	}
	return item, nil
}
