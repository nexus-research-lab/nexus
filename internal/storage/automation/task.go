// INPUT: owner-scoped Automation 任务查询、配置版本写入与领域目标字段。
// OUTPUT: 完整 ScheduledTask 快照及带 CAS 的配置持久化结果。
// POS: Automation task repository；SELECT/UPDATE 字段顺序与 scan_automation.go 同构。
package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// ListScheduledTasks 列出定时任务。ownerUserID 为空时表示全局作用域。
func (r *Repository) ListScheduledTasks(ctx context.Context, ownerUserID string, agentID string) ([]automationdomain.ScheduledTask, error) {
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
    execution_kind,
    permission_mode,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    delivery_session_key,
    session_binding_state,
    invalidated_session_keys_json,
    source_kind,
    source_creator_agent_id,
    source_context_type,
    source_context_id,
    source_context_label,
    source_session_key,
    source_session_label,
    delivery_grant_json,
    overlap_policy,
    expires_at,
    enabled,
    next_run_at,
    running_run_id,
    running_started_at,
    last_run_at,
    last_run_status,
    failure_streak,
    last_error,
    last_delivery_status,
    configuration_version,
    permission_policy_json,
    permission_policy_revision,
    permission_state,
    pending_permission_request_id
FROM automation_scheduled_tasks`
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

	items := make([]automationdomain.ScheduledTask, 0)
	for rows.Next() {
		item, scanErr := scanScheduledTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CountEnabledScheduledTasks 统计启用中的定时任务数量。ownerUserID 为空时表示全局作用域。
func (r *Repository) CountEnabledScheduledTasks(ctx context.Context, ownerUserID string, agentID string) (int, error) {
	query := "SELECT COUNT(1) FROM automation_scheduled_tasks WHERE enabled = " + r.bind(1)
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

// GetScheduledTask 读取单个任务。ownerUserID 为空时表示全局作用域。
func (r *Repository) GetScheduledTask(ctx context.Context, ownerUserID string, jobID string) (*automationdomain.ScheduledTask, error) {
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
    execution_kind,
    permission_mode,
    session_target_kind,
    bound_session_key,
    named_session_key,
    wake_mode,
    delivery_mode,
    delivery_channel,
    delivery_to,
    delivery_account_id,
    delivery_thread_id,
    delivery_session_key,
    session_binding_state,
    invalidated_session_keys_json,
    source_kind,
    source_creator_agent_id,
    source_context_type,
    source_context_id,
    source_context_label,
    source_session_key,
    source_session_label,
    delivery_grant_json,
    overlap_policy,
    expires_at,
    enabled,
    next_run_at,
    running_run_id,
    running_started_at,
    last_run_at,
    last_run_status,
    failure_streak,
    last_error,
    last_delivery_status,
    configuration_version,
    permission_policy_json,
    permission_policy_revision,
    permission_state,
    pending_permission_request_id
FROM automation_scheduled_tasks
WHERE job_id = ` + r.bind(1)

	args := []any{strings.TrimSpace(jobID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}

	row := r.db.QueryRowContext(ctx, query, args...)
	item, err := scanScheduledTaskRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

// UpsertScheduledTask 创建或更新任务。
func (r *Repository) UpsertScheduledTask(ctx context.Context, job automationdomain.ScheduledTask) (*automationdomain.ScheduledTask, error) {
	args, err := scheduledTaskUpsertArgs(job)
	if err != nil {
		return nil, err
	}
	_, err = r.execWithRetry(
		ctx,
		r.upsertScheduledTaskQuery,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return r.GetScheduledTask(ctx, "", job.JobID)
}

// CreateScheduledTaskIdempotent 以 owner/request_id 原子认领一次创建意图。
// 相同意图重试返回首次任务；相同 request_id 的不同意图会稳定冲突。
func (r *Repository) CreateScheduledTaskIdempotent(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	requestID string,
	intentDigest string,
) (*automationdomain.ScheduledTask, bool, error) {
	attempts := automationWriteRetryAttempts
	if r.isPostgres {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		task, created, err := r.createScheduledTaskIdempotentOnce(
			ctx,
			job,
			requestID,
			intentDigest,
		)
		if err == nil || !isSQLiteLockedError(err) || attempt == attempts-1 {
			return task, created, err
		}
		timer := time.NewTimer(automationWriteRetryDelay * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, false, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, false, errors.New("scheduled task idempotent create retries exhausted")
}

func (r *Repository) createScheduledTaskIdempotentOnce(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	requestID string,
	intentDigest string,
) (*automationdomain.ScheduledTask, bool, error) {
	requestID = strings.TrimSpace(requestID)
	intentDigest = strings.TrimSpace(intentDigest)
	if requestID == "" || intentDigest == "" {
		return nil, false, errors.New("request_id and intent_digest are required")
	}
	upsertArgs, err := scheduledTaskUpsertArgs(job)
	if err != nil {
		return nil, false, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(
			`INSERT INTO automation_task_create_requests (
    owner_user_id, request_id, job_id, agent_id, intent_digest, created_at
) VALUES (%s,CURRENT_TIMESTAMP)
ON CONFLICT(owner_user_id, request_id) DO NOTHING`,
			r.bindList(5),
		),
		strings.TrimSpace(job.OwnerUserID),
		requestID,
		strings.TrimSpace(job.JobID),
		strings.TrimSpace(job.AgentID),
		intentDigest,
	)
	if err != nil {
		return nil, false, err
	}
	claimed, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	jobID := strings.TrimSpace(job.JobID)
	if claimed == 0 {
		var existingDigest string
		var existingAgentID string
		err = tx.QueryRowContext(
			ctx,
			fmt.Sprintf(
				`SELECT job_id, agent_id, intent_digest
FROM automation_task_create_requests
WHERE owner_user_id = %s AND request_id = %s`,
				r.bind(1),
				r.bind(2),
			),
			strings.TrimSpace(job.OwnerUserID),
			requestID,
		).Scan(&jobID, &existingAgentID, &existingDigest)
		if err != nil {
			return nil, false, err
		}
		if strings.TrimSpace(existingDigest) != intentDigest ||
			strings.TrimSpace(existingAgentID) != strings.TrimSpace(job.AgentID) {
			return nil, false, automationdomain.ErrCreateRequestConflict
		}
	} else {
		if _, err = tx.ExecContext(ctx, r.upsertScheduledTaskQuery, upsertArgs...); err != nil {
			return nil, false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	created, err := r.GetScheduledTask(ctx, strings.TrimSpace(job.OwnerUserID), jobID)
	if err != nil {
		return nil, false, err
	}
	if created == nil {
		return nil, false, automationdomain.ErrJobNotFound
	}
	return created, claimed == 1, nil
}

// GetScheduledTaskCreateReplay 检查 request_id 是否已经绑定到相同创建意图。
func (r *Repository) GetScheduledTaskCreateReplay(
	ctx context.Context,
	ownerUserID string,
	requestID string,
	agentID string,
	intentDigest string,
) (*automationdomain.ScheduledTask, bool, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, false, nil
	}
	var (
		jobID          string
		existingAgent  string
		existingDigest string
	)
	err := r.db.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`SELECT job_id, agent_id, intent_digest
FROM automation_task_create_requests
WHERE owner_user_id = %s AND request_id = %s`,
			r.bind(1),
			r.bind(2),
		),
		strings.TrimSpace(ownerUserID),
		requestID,
	).Scan(&jobID, &existingAgent, &existingDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(existingAgent) != strings.TrimSpace(agentID) ||
		strings.TrimSpace(existingDigest) != strings.TrimSpace(intentDigest) {
		return nil, true, automationdomain.ErrCreateRequestConflict
	}
	task, err := r.GetScheduledTask(ctx, strings.TrimSpace(ownerUserID), strings.TrimSpace(jobID))
	if err != nil {
		return nil, true, err
	}
	if task == nil {
		return nil, true, automationdomain.ErrJobNotFound
	}
	return task, true, nil
}

// UpdateScheduledTaskAtVersion 仅在版本仍与读取快照一致时覆盖配置并推进版本。
func (r *Repository) UpdateScheduledTaskAtVersion(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	expectedVersion int64,
) (*automationdomain.ScheduledTask, error) {
	return r.updateScheduledTaskAtVersion(ctx, job, expectedVersion, nil)
}

// UpdateScheduledTaskAtVersionAndRunningRun 把配置 revision 与当前运行身份放进
// 同一条条件写入，避免先提交配置后才发现 cancel_active_run 针对的是旧 run。
func (r *Repository) UpdateScheduledTaskAtVersionAndRunningRun(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	expectedVersion int64,
	expectedRunningRunID string,
) (*automationdomain.ScheduledTask, error) {
	expectedRunningRunID = strings.TrimSpace(expectedRunningRunID)
	return r.updateScheduledTaskAtVersion(ctx, job, expectedVersion, &expectedRunningRunID)
}

func (r *Repository) updateScheduledTaskAtVersion(
	ctx context.Context,
	job automationdomain.ScheduledTask,
	expectedVersion int64,
	expectedRunningRunID *string,
) (*automationdomain.ScheduledTask, error) {
	if expectedVersion < 1 {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	columns := []string{
		"owner_user_id",
		"name",
		"agent_id",
		"schedule_kind",
		"run_at",
		"interval_seconds",
		"cron_expression",
		"timezone",
		"instruction",
		"execution_kind",
		"permission_mode",
		"session_target_kind",
		"bound_session_key",
		"named_session_key",
		"wake_mode",
		"delivery_mode",
		"delivery_channel",
		"delivery_to",
		"delivery_account_id",
		"delivery_thread_id",
		"delivery_session_key",
		"session_binding_state",
		"invalidated_session_keys_json",
		"source_kind",
		"source_creator_agent_id",
		"source_context_type",
		"source_context_id",
		"source_context_label",
		"source_session_key",
		"source_session_label",
		"delivery_grant_json",
		"overlap_policy",
		"expires_at",
		"enabled",
		"permission_policy_json",
		"permission_policy_revision",
		"permission_state",
		"pending_permission_request_id",
	}
	assignments := make([]string, 0, len(columns)+2)
	for index, column := range columns {
		assignments = append(assignments, column+" = "+r.bind(index+1))
	}
	assignments = append(assignments,
		"configuration_version = configuration_version + 1",
		"updated_at = CURRENT_TIMESTAMP",
	)
	args, err := scheduledTaskDefinitionArgs(job)
	if err != nil {
		return nil, err
	}
	args = append(args, strings.TrimSpace(job.JobID), strings.TrimSpace(job.OwnerUserID), expectedVersion)
	query := "UPDATE automation_scheduled_tasks SET " + strings.Join(assignments, ", ") +
		" WHERE job_id = " + r.bind(len(columns)+1) +
		" AND owner_user_id = " + r.bind(len(columns)+2) +
		" AND configuration_version = " + r.bind(len(columns)+3)
	if expectedRunningRunID != nil {
		args = append(args, strings.TrimSpace(*expectedRunningRunID))
		query += " AND COALESCE(running_run_id, '') = " + r.bind(len(columns)+4)
	}
	result, err := r.execWithRetry(
		ctx,
		query,
		args...,
	)
	if err != nil {
		return nil, err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if updated != 1 {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	return r.GetScheduledTask(ctx, strings.TrimSpace(job.OwnerUserID), strings.TrimSpace(job.JobID))
}

// DeleteScheduledTask 删除任务。ownerUserID 为空时表示全局作用域。
func (r *Repository) DeleteScheduledTask(ctx context.Context, ownerUserID string, jobID string) error {
	query := "DELETE FROM automation_scheduled_tasks WHERE job_id = " + r.bind(1)
	args := []any{strings.TrimSpace(jobID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	_, err := r.execWithRetry(ctx, query, args...)
	return err
}

// DeleteScheduledTaskAtVersion 仅删除调用方实际检查过的版本。
func (r *Repository) DeleteScheduledTaskAtVersion(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	expectedVersion int64,
) error {
	if expectedVersion < 1 {
		return automationdomain.ErrConfigurationVersionConflict
	}
	result, err := r.execWithRetry(
		ctx,
		"DELETE FROM automation_scheduled_tasks WHERE job_id = "+r.bind(1)+
			" AND owner_user_id = "+r.bind(2)+
			" AND configuration_version = "+r.bind(3),
		strings.TrimSpace(jobID),
		strings.TrimSpace(ownerUserID),
		expectedVersion,
	)
	if err != nil {
		return err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted != 1 {
		return automationdomain.ErrConfigurationVersionConflict
	}
	return nil
}

func scheduledTaskUpsertArgs(job automationdomain.ScheduledTask) ([]any, error) {
	definitionArgs, err := scheduledTaskDefinitionArgs(job)
	if err != nil {
		return nil, err
	}
	return append([]any{strings.TrimSpace(job.JobID)}, definitionArgs...), nil
}

func scheduledTaskDefinitionArgs(job automationdomain.ScheduledTask) ([]any, error) {
	job = automationdomain.NormalizeScheduledTaskCompatibility(job)
	permissionPolicyJSON, err := json.Marshal(job.PermissionPolicy)
	if err != nil {
		return nil, err
	}
	invalidatedSessionKeysJSON, err := json.Marshal(job.InvalidatedSessionKeys)
	if err != nil {
		return nil, err
	}
	deliveryGrantJSON, err := json.Marshal(job.DeliveryGrant)
	if err != nil {
		return nil, err
	}
	return []any{
		strings.TrimSpace(job.OwnerUserID),
		job.Name,
		job.AgentID,
		job.Schedule.Kind,
		nullStringPointer(job.Schedule.RunAt),
		nullIntPointer(job.Schedule.IntervalSeconds),
		nullStringPointer(job.Schedule.CronExpression),
		job.Schedule.Timezone,
		job.Instruction,
		automationdomain.NormalizeExecutionKind(job.ExecutionKind),
		automationdomain.NormalizePermissionMode(job.PermissionMode),
		job.SessionTarget.Kind,
		nullString(job.SessionTarget.BoundSessionKey),
		nullString(job.SessionTarget.NamedSessionKey),
		job.SessionTarget.WakeMode,
		job.Delivery.Mode,
		nullString(job.Delivery.Channel),
		nullString(job.Delivery.To),
		nullString(job.Delivery.AccountID),
		nullString(job.Delivery.ThreadID),
		nullString(job.Delivery.SessionKey),
		job.SessionBindingState,
		string(invalidatedSessionKeysJSON),
		job.Source.Kind,
		nullString(job.Source.CreatorAgentID),
		nullString(job.Source.ContextType),
		nullString(job.Source.ContextID),
		nullString(job.Source.ContextLabel),
		nullString(job.Source.SessionKey),
		nullString(job.Source.SessionLabel),
		string(deliveryGrantJSON),
		automationdomain.NormalizeOverlapPolicy(job.OverlapPolicy),
		nullableTime(job.ExpiresAt),
		job.Enabled,
		string(permissionPolicyJSON),
		job.PermissionPolicy.Revision,
		strings.TrimSpace(job.PermissionState),
		nullString(strings.TrimSpace(job.PendingPermissionRequestID)),
	}, nil
}
