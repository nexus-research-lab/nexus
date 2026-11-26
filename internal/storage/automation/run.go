// INPUT: task-owned run 身份、初始 attempt/request 身份、状态转换、投递快照与 exact permission retry request。
// OUTPUT: owner-scoped run ledger 查询和带条件领取、幂等重放、恢复、完成写入。
// POS: Automation run 持久状态机；首条 run 可与 task claim 同事务写入，只有 exact request 领取成功后才清理 retry 绑定。
package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// RunPendingInput 表示创建 run ledger 的输入。
type RunPendingInput struct {
	RunID                    string
	JobID                    string
	OwnerUserID              string
	ScheduledFor             *time.Time
	TriggerKind              string
	SessionKey               string
	RoundID                  string
	DeliveryMode             string
	DeliveryTo               string
	DeliveryTarget           *automationdomain.DeliveryTarget
	DeliveryStatus           string
	Status                   string
	StartedAt                *time.Time
	FinishedAt               *time.Time
	Attempts                 int
	ErrorMessage             *string
	PermissionPolicyRevision int
	ClientRequestID          string
	IntentDigest             string
}

const scheduledTaskRunSelectColumns = `
    run_id,
    job_id,
    owner_user_id,
    client_request_id,
    status,
    trigger_kind,
    session_key,
    round_id,
    session_id,
    message_count,
    delivery_mode,
    delivery_to,
    delivery_target_json,
    delivery_status,
    delivery_error,
    delivered_at,
    delivery_attempts,
    delivery_next_attempt_at,
    delivery_dead_letter_at,
    scheduled_for,
    started_at,
    finished_at,
    attempts,
    error_message,
    result_summary,
    assistant_text,
    result_text,
    artifact_path,
    permission_policy_revision,
    block_state,
    blocked_request_id,
    effect_started,
    created_at,
    updated_at`

// RunFinishInput 表示结束 run ledger 的输入。
type RunFinishInput struct {
	RunID                 string
	Status                string
	FinishedAt            time.Time
	ErrorMessage          *string
	SessionID             *string
	MessageCount          int
	ResultSummary         *string
	AssistantText         *string
	ResultText            *string
	ArtifactPath          *string
	DeliveryTo            string
	DeliveryStatus        string
	DeliveryError         *string
	DeliveredAt           *time.Time
	DeliveryAttempted     bool
	DeliveryNextAttemptAt *time.Time
	DeliveryDeadLetterAt  *time.Time
}

// ListRunsByJob 列出任务运行历史。ownerUserID 为空时表示全局作用域。
func (r *Repository) ListRunsByJob(ctx context.Context, ownerUserID string, jobID string) ([]automationdomain.ScheduledTaskRun, error) {
	query := `SELECT` + scheduledTaskRunSelectColumns + `
FROM automation_task_runs
WHERE job_id = ` + r.bind(1)
	args := []any{strings.TrimSpace(jobID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	query += `
ORDER BY created_at DESC, run_id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]automationdomain.ScheduledTaskRun, 0)
	for rows.Next() {
		item, scanErr := scanScheduledTaskRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetRun 读取一条任务运行历史。ownerUserID 为空时表示全局作用域。
func (r *Repository) GetRun(ctx context.Context, ownerUserID string, jobID string, runID string) (*automationdomain.ScheduledTaskRun, error) {
	query := `SELECT` + scheduledTaskRunSelectColumns + `
FROM automation_task_runs
WHERE job_id = ` + r.bind(1) + `
  AND run_id = ` + r.bind(2)
	args := []any{strings.TrimSpace(jobID), strings.TrimSpace(runID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	item, err := scanScheduledTaskRun(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetRunByClientRequest 按人工命令身份读取 exact run；同一 request 被复用于
// 其他任务或意图时返回 typed conflict。未找到不是错误。
func (r *Repository) GetRunByClientRequest(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	requestID string,
	intentDigest string,
) (*automationdomain.ScheduledTaskRun, bool, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	jobID = strings.TrimSpace(jobID)
	requestID = strings.TrimSpace(requestID)
	intentDigest = strings.TrimSpace(intentDigest)
	if requestID == "" {
		return nil, false, nil
	}
	var storedJobID string
	var runID string
	var storedIntentDigest string
	err := r.db.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`SELECT job_id, run_id, client_intent_digest
FROM automation_task_runs
WHERE owner_user_id = %s AND client_request_id = %s`,
			r.bind(1), r.bind(2),
		),
		ownerUserID,
		requestID,
	).Scan(&storedJobID, &runID, &storedIntentDigest)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	if (jobID != "" && strings.TrimSpace(storedJobID) != jobID) ||
		strings.TrimSpace(storedIntentDigest) != intentDigest {
		return nil, true, automationdomain.ErrRuntimeCommandConflict
	}
	run, err := r.GetRun(ctx, ownerUserID, strings.TrimSpace(storedJobID), strings.TrimSpace(runID))
	return run, true, err
}

// InsertRunPending 新建一条待执行 run。
func (r *Repository) InsertRunPending(ctx context.Context, input RunPendingInput) error {
	args, err := r.initialRunInsertArgs(input)
	if err != nil {
		return err
	}
	result, err := r.execWithRetry(ctx, r.insertRunPendingQuery, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 1 {
		return nil
	}
	task, err := r.GetScheduledTask(ctx, strings.TrimSpace(input.OwnerUserID), strings.TrimSpace(input.JobID))
	if err != nil {
		return err
	}
	if task == nil {
		return automationdomain.ErrJobNotFound
	}
	return automationdomain.ErrTaskDeleting
}

func (r *Repository) initialRunInsertArgs(input RunPendingInput) ([]any, error) {
	values, err := r.initialRunValueArgs(input)
	if err != nil {
		return nil, err
	}
	return append(
		values,
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
	), nil
}

func (r *Repository) initialRunValueArgs(input RunPendingInput) ([]any, error) {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = automationdomain.RunStatusPending
	}
	deliveryTargetJSON, err := marshalRunDeliveryTarget(input.DeliveryTarget)
	if err != nil {
		return nil, err
	}
	return []any{
		strings.TrimSpace(input.RunID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
		status,
		strings.TrimSpace(input.TriggerKind),
		nullString(strings.TrimSpace(input.SessionKey)),
		nullString(strings.TrimSpace(input.RoundID)),
		nullString(strings.TrimSpace(input.DeliveryMode)),
		nullString(strings.TrimSpace(input.DeliveryTo)),
		nullString(deliveryTargetJSON),
		nullString(initialRunDeliveryStatus(input)),
		input.ScheduledFor,
		nullableTime(input.StartedAt),
		nullableTime(input.FinishedAt),
		input.Attempts,
		nullableString(input.ErrorMessage),
		input.PermissionPolicyRevision,
		nullString(strings.TrimSpace(input.ClientRequestID)),
		nullString(strings.TrimSpace(input.IntentDigest)),
	}, nil
}

func marshalRunDeliveryTarget(target *automationdomain.DeliveryTarget) (string, error) {
	if target == nil {
		return "", nil
	}
	normalized := target.Normalized()
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// MarkRunRunning 标记 run 开始执行。
func (r *Repository) MarkRunRunning(ctx context.Context, runID string, startedAt time.Time) error {
	result, err := r.execWithRetry(ctx, r.markRunRunningQuery, automationdomain.RunStatusRunning, startedAt.UTC(), runID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrTaskDeleting
	}
	return nil
}

// MarkRunFinished 标记 run 结束状态。
func (r *Repository) MarkRunFinished(ctx context.Context, input RunFinishInput) error {
	_, err := r.execWithRetry(
		ctx,
		r.markRunFinishedQuery,
		strings.TrimSpace(input.Status),
		input.FinishedAt.UTC(),
		nullableString(input.ErrorMessage),
		nullableString(input.SessionID),
		input.MessageCount,
		nullableString(input.ResultSummary),
		nullableString(input.AssistantText),
		nullableString(input.ResultText),
		nullableString(input.ArtifactPath),
		nullString(strings.TrimSpace(input.DeliveryTo)),
		nullString(finishedRunDeliveryStatus(input)),
		nullableString(input.DeliveryError),
		nullableTime(input.DeliveredAt),
		input.DeliveryAttempted,
		nullableTime(input.DeliveryNextAttemptAt),
		nullableTime(input.DeliveryDeadLetterAt),
		strings.TrimSpace(input.RunID),
	)
	return err
}

// MarkRunFinishedIfActive 仅在 run 仍处于未完成状态时写入结束结果。
func (r *Repository) MarkRunFinishedIfActive(ctx context.Context, input RunFinishInput) (bool, error) {
	query := fmt.Sprintf(
		`UPDATE automation_task_runs
SET status = %s,
    finished_at = %s,
    error_message = %s,
    session_id = %s,
    message_count = %s,
    result_summary = %s,
    assistant_text = %s,
    result_text = %s,
    artifact_path = %s,
    delivery_to = COALESCE(%s, delivery_to),
    delivery_status = %s,
    delivery_error = %s,
    delivered_at = %s,
    delivery_attempts = delivery_attempts + CASE WHEN %s THEN 1 ELSE 0 END,
    delivery_next_attempt_at = %s,
    delivery_dead_letter_at = %s,
    block_state = '',
    blocked_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND status IN (%s, %s)`,
		r.bind(1),
		r.bind(2),
		r.bind(3),
		r.bind(4),
		r.bind(5),
		r.bind(6),
		r.bind(7),
		r.bind(8),
		r.bind(9),
		r.bind(10),
		r.bind(11),
		r.bind(12),
		r.bind(13),
		r.bind(14),
		r.bind(15),
		r.bind(16),
		r.bind(17),
		r.bind(18),
		r.bind(19),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		strings.TrimSpace(input.Status),
		input.FinishedAt.UTC(),
		nullableString(input.ErrorMessage),
		nullableString(input.SessionID),
		input.MessageCount,
		nullableString(input.ResultSummary),
		nullableString(input.AssistantText),
		nullableString(input.ResultText),
		nullableString(input.ArtifactPath),
		nullString(strings.TrimSpace(input.DeliveryTo)),
		nullString(finishedRunDeliveryStatus(input)),
		nullableString(input.DeliveryError),
		nullableTime(input.DeliveredAt),
		input.DeliveryAttempted,
		nullableTime(input.DeliveryNextAttemptAt),
		nullableTime(input.DeliveryDeadLetterAt),
		strings.TrimSpace(input.RunID),
		automationdomain.RunStatusPending,
		automationdomain.RunStatusRunning,
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
