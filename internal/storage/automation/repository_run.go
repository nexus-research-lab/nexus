package automation

import (
	"context"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
)

// RunPendingInput 表示创建 run ledger 的输入。
type RunPendingInput struct {
	RunID          string
	JobID          string
	OwnerUserID    string
	ScheduledFor   *time.Time
	TriggerKind    string
	SessionKey     string
	RoundID        string
	DeliveryMode   string
	DeliveryTo     string
	DeliveryStatus string
	Status         string
}

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
func (r *Repository) ListRunsByJob(ctx context.Context, ownerUserID string, jobID string) ([]automationdomain.CronRun, error) {
	query := `
SELECT
    run_id,
    job_id,
    owner_user_id,
    status,
    trigger_kind,
    session_key,
    round_id,
    session_id,
    message_count,
    delivery_mode,
    delivery_to,
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
    created_at,
    updated_at
FROM automation_cron_runs
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

	items := make([]automationdomain.CronRun, 0)
	for rows.Next() {
		item, scanErr := scanCronRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetRun 读取一条任务运行历史。ownerUserID 为空时表示全局作用域。
func (r *Repository) GetRun(ctx context.Context, ownerUserID string, jobID string, runID string) (*automationdomain.CronRun, error) {
	query := `
SELECT
    run_id,
    job_id,
    owner_user_id,
    status,
    trigger_kind,
    session_key,
    round_id,
    session_id,
    message_count,
    delivery_mode,
    delivery_to,
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
    created_at,
    updated_at
FROM automation_cron_runs
WHERE job_id = ` + r.bind(1) + `
  AND run_id = ` + r.bind(2)
	args := []any{strings.TrimSpace(jobID), strings.TrimSpace(runID)}
	if strings.TrimSpace(ownerUserID) != "" {
		args = append(args, strings.TrimSpace(ownerUserID))
		query += " AND owner_user_id = " + r.bind(len(args))
	}
	item, err := scanCronRun(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// InsertRunPending 新建一条待执行 run。
func (r *Repository) InsertRunPending(ctx context.Context, input RunPendingInput) error {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = automationdomain.RunStatusPending
	}
	_, err := r.execWithRetry(
		ctx,
		r.insertRunPendingQuery,
		strings.TrimSpace(input.RunID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
		status,
		strings.TrimSpace(input.TriggerKind),
		nullString(strings.TrimSpace(input.SessionKey)),
		nullString(strings.TrimSpace(input.RoundID)),
		nullString(strings.TrimSpace(input.DeliveryMode)),
		nullString(strings.TrimSpace(input.DeliveryTo)),
		nullString(initialRunDeliveryStatus(input)),
		input.ScheduledFor,
		0,
	)
	return err
}

// MarkRunRunning 标记 run 开始执行。
func (r *Repository) MarkRunRunning(ctx context.Context, runID string, startedAt time.Time) error {
	_, err := r.execWithRetry(ctx, r.markRunRunningQuery, automationdomain.RunStatusRunning, startedAt.UTC(), runID)
	return err
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
		`UPDATE automation_cron_runs
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
