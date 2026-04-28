package automation

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListCronJobs 列出定时任务。ownerUserID 为空时表示全局作用域。
func (r *Repository) ListCronJobs(ctx context.Context, ownerUserID string, agentID string) ([]protocol.CronJob, error) {
	query := `
SELECT
    job_id,
    owner_user_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    source_kind,
    source_creator_agent_id,
    source_context_type,
    source_context_id,
    source_context_label,
    source_session_key,
    source_session_label,
    overlap_policy,
    enabled
FROM automation_cron_jobs`
	args := []any{}
	conditions := make([]string, 0, 2)
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		conditions = append(conditions, "owner_user_id = "+r.bind(len(args)))
	}
	if strings.TrimSpace(agentID) != "" {
		args = append(args, strings.TrimSpace(agentID))
		conditions = append(conditions, "agent_id = "+r.bind(len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC, job_id DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]protocol.CronJob, 0)
	for rows.Next() {
		item, scanErr := scanCronJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CountEnabledCronJobs 统计启用中的定时任务数量。ownerUserID 为空时表示全局作用域。
func (r *Repository) CountEnabledCronJobs(ctx context.Context, ownerUserID string, agentID string) (int, error) {
	query := "SELECT COUNT(1) FROM automation_cron_jobs WHERE enabled = " + r.bind(1)
	args := []any{true}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	if strings.TrimSpace(agentID) != "" {
		args = append(args, strings.TrimSpace(agentID))
		query += " AND agent_id = " + r.bind(len(args))
	}
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetCronJob 读取单个任务。ownerUserID 为空时表示全局作用域。
func (r *Repository) GetCronJob(ctx context.Context, ownerUserID string, jobID string) (*protocol.CronJob, error) {
	query := `
SELECT
    job_id,
    owner_user_id,
    name,
    agent_id,
    schedule_kind,
    run_at,
    interval_seconds,
    cron_expression,
    timezone,
    instruction,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    source_kind,
    source_creator_agent_id,
    source_context_type,
    source_context_id,
    source_context_label,
    source_session_key,
    source_session_label,
    overlap_policy,
    enabled
FROM automation_cron_jobs
WHERE job_id = ` + r.bind(1)

	args := []any{strings.TrimSpace(jobID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}

	row := r.db.QueryRowContext(ctx, query, args...)
	item, err := scanCronJobRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

// UpsertCronJob 创建或更新任务。
func (r *Repository) UpsertCronJob(ctx context.Context, job protocol.CronJob) (*protocol.CronJob, error) {
	_, err := r.execWithRetry(
		ctx,
		r.upsertCronJobQuery,
		job.JobID,
		job.OwnerUserID,
		job.Name,
		job.AgentID,
		job.Schedule.Kind,
		nullStringPointer(job.Schedule.RunAt),
		nullIntPointer(job.Schedule.IntervalSeconds),
		nullStringPointer(job.Schedule.CronExpression),
		job.Schedule.Timezone,
		job.Instruction,
		job.SessionTarget.Kind,
		nullString(job.SessionTarget.BoundSessionKey),
		nullString(job.SessionTarget.NamedSessionKey),
		job.SessionTarget.WakeMode,
		job.Delivery.Mode,
		nullString(job.Delivery.Channel),
		nullString(job.Delivery.To),
		nullString(job.Delivery.AccountID),
		nullString(job.Delivery.ThreadID),
		job.Source.Kind,
		nullString(job.Source.CreatorAgentID),
		nullString(job.Source.ContextType),
		nullString(job.Source.ContextID),
		nullString(job.Source.ContextLabel),
		nullString(job.Source.SessionKey),
		nullString(job.Source.SessionLabel),
		protocol.NormalizeOverlapPolicy(job.OverlapPolicy),
		job.Enabled,
	)
	if err != nil {
		return nil, err
	}
	return r.GetCronJob(ctx, "", job.JobID)
}

// DeleteCronJob 删除任务。ownerUserID 为空时表示全局作用域。
func (r *Repository) DeleteCronJob(ctx context.Context, ownerUserID string, jobID string) error {
	query := "DELETE FROM automation_cron_jobs WHERE job_id = " + r.bind(1)
	args := []any{strings.TrimSpace(jobID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	_, err := r.execWithRetry(ctx, query, args...)
	return err
}
