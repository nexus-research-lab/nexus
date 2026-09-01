// INPUT: Automation run 的投递结果、retry/dead-letter 状态与 due 查询边界。
// OUTPUT: CAS-safe 投递观测写入、最新完成 run 的 task 摘要投影、到期 retry rows 与最早 deadline。
// POS: Automation delivery durable state；service timer 只消费其 deadline，不复制状态机。
package automation

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// NextDeliveryRetryAt 返回最早可领取的 terminal pending 首投或 failed retry。
// NULL retry deadline 使用 updated_at，保持崩溃恢复为 durable due work。
func (r *Repository) NextDeliveryRetryAt(
	ctx context.Context,
	maxAttempts int,
) (*time.Time, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var deadline any
	err := r.db.QueryRowContext(ctx, `
SELECT MIN(CASE
    WHEN delivery_status = `+r.bind(1)+` THEN updated_at
    ELSE COALESCE(delivery_next_attempt_at, updated_at)
END)
FROM automation_task_runs

WHERE delivery_dead_letter_at IS NULL
  AND (
      (delivery_status = `+r.bind(2)+`
       AND status = `+r.bind(3)+`
       AND delivery_attempts = 0)
      OR
      (delivery_status = `+r.bind(4)+`
       AND delivery_attempts < `+r.bind(5)+`)
  )`,
		automationdomain.DeliveryStatusPending,
		automationdomain.DeliveryStatusPending,
		automationdomain.RunStatusSucceeded,
		automationdomain.DeliveryStatusFailed,
		maxAttempts,
	).Scan(&deadline)
	if err != nil {
		return nil, err
	}
	return storage.NullableTime(deadline)
}

// RunDeliveryAttemptClaimInput 表示外投副作用发生前的 exact durable claim。
type RunDeliveryAttemptClaimInput struct {
	OwnerUserID                  string
	JobID                        string
	RunID                        string
	ExpectedDeliveryAttempts     int
	ExpectedConfigurationVersion *int64
	ExpectedStatus               string
	AttemptID                    string
	RequireEnabled               bool
	RequireNoDeadLetter          bool
	ConfirmUnverifiedAttempt     bool
}

// ClaimRunDeliveryAttempt 在外投前唯一领取一次 retry。attempt token 只在存储层
// 流转，不进入公开 run read model；领取后的 retrying 状态不会被自动 due 查询消费。
func (r *Repository) ClaimRunDeliveryAttempt(ctx context.Context, input RunDeliveryAttemptClaimInput) error {
	ownerUserID := strings.TrimSpace(input.OwnerUserID)
	jobID := strings.TrimSpace(input.JobID)
	runID := strings.TrimSpace(input.RunID)
	attemptID := strings.TrimSpace(input.AttemptID)
	if ownerUserID == "" || jobID == "" || runID == "" || attemptID == "" ||
		input.ExpectedDeliveryAttempts < 0 ||
		(input.ExpectedConfigurationVersion != nil && *input.ExpectedConfigurationVersion < 1) {
		return automationdomain.ErrDeliveryRetryConflict
	}
	expectedStatus := strings.TrimSpace(input.ExpectedStatus)
	if expectedStatus == "" {
		expectedStatus = automationdomain.DeliveryStatusFailed
	}
	if input.ConfirmUnverifiedAttempt {
		expectedStatus = automationdomain.DeliveryStatusRetrying
	}
	deadLetterClause := ""
	if input.RequireNoDeadLetter {
		deadLetterClause = " AND delivery_dead_letter_at IS NULL"
	}
	args := []any{
		automationdomain.DeliveryStatusRetrying,
		attemptID,
		ownerUserID,
		jobID,
		runID,
		expectedStatus,
		input.ExpectedDeliveryAttempts,
		ownerUserID,
		jobID,
	}
	versionClause := ""
	if input.ExpectedConfigurationVersion != nil {
		args = append(args, *input.ExpectedConfigurationVersion)
		versionClause = " AND task.configuration_version = " + r.bind(len(args))
	}
	enabledClause := ""
	if input.RequireEnabled {
		args = append(args, true)
		enabledClause = " AND task.enabled = " + r.bind(len(args))
	}
	query := fmt.Sprintf(`UPDATE automation_task_runs
SET delivery_status = %s,
    delivery_attempts = delivery_attempts + 1,
    delivery_attempt_id = %s,
    delivery_attempt_started_at = CURRENT_TIMESTAMP,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND delivery_status = %s
  AND delivery_attempts = %s%s
  AND EXISTS (
      SELECT 1
      FROM automation_scheduled_tasks task
      WHERE task.owner_user_id = %s
        AND task.job_id = %s
        AND COALESCE(task.deletion_state, '') = ''%s%s
  )`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), deadLetterClause, r.bind(8), r.bind(9), versionClause, enabledClause,
	)
	result, err := r.execWithRetry(ctx, query, args...)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrDeliveryRetryConflict
	}
	return nil
}

// RunDeliveryAttemptCompletionInput 表示 exact attempt 的最终 durable 结果。
type RunDeliveryAttemptCompletionInput struct {
	OwnerUserID           string
	JobID                 string
	RunID                 string
	AttemptID             string
	DeliveryMode          string
	DeliveryTo            string
	DeliveryStatus        string
	DeliveryError         *string
	DeliveredAt           *time.Time
	DeliveryNextAttemptAt *time.Time
	DeliveryDeadLetterAt  *time.Time
}

// CompleteRunDeliveryAttempt 只完成仍由 exact attempt token 持有的 retry。
// CAS 写失败时行保持 retrying，调用方不得据此自动再次外投。
func (r *Repository) CompleteRunDeliveryAttempt(ctx context.Context, input RunDeliveryAttemptCompletionInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	query := fmt.Sprintf(`UPDATE automation_task_runs
SET delivery_mode = COALESCE(%s, delivery_mode),
    delivery_to = COALESCE(%s, delivery_to),
    delivery_status = %s,
    delivery_error = %s,
    delivered_at = %s,
    delivery_next_attempt_at = %s,
    delivery_dead_letter_at = %s,
    delivery_attempt_id = NULL,
    delivery_attempt_started_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND delivery_status = %s
  AND delivery_attempt_id = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9), r.bind(10), r.bind(11), r.bind(12),
	)
	result, err := tx.ExecContext(
		ctx,
		query,
		nullString(strings.TrimSpace(input.DeliveryMode)),
		nullString(strings.TrimSpace(input.DeliveryTo)),
		nullString(strings.TrimSpace(input.DeliveryStatus)),
		nullableString(input.DeliveryError),
		nullableTime(input.DeliveredAt),
		nullableTime(input.DeliveryNextAttemptAt),
		nullableTime(input.DeliveryDeadLetterAt),
		strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.RunID),
		automationdomain.DeliveryStatusRetrying,
		strings.TrimSpace(input.AttemptID),
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrDeliveryRetryCompletionUnconfirmed
	}
	if err = r.updateExactTaskLastDeliveryStatus(
		ctx, tx, input.OwnerUserID, input.JobID, input.RunID, input.DeliveryStatus,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkRunDeliveryAttemptUnconfirmed 保存 router 已被调用但结果未知的诊断，保持
// retrying 与 exact token，使自动 worker 永远不会把它当 failed 重放。
func (r *Repository) MarkRunDeliveryAttemptUnconfirmed(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	runID string,
	attemptID string,
	deliveryMode string,
	deliveryTo string,
	deliveryError *string,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE automation_task_runs
SET delivery_mode = COALESCE(%s, delivery_mode),
    delivery_to = COALESCE(%s, delivery_to),
    delivery_error = %s,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND delivery_status = %s
  AND delivery_attempt_id = %s`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8),
		),
		nullString(strings.TrimSpace(deliveryMode)),
		nullString(strings.TrimSpace(deliveryTo)),
		nullableString(deliveryError),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(runID),
		automationdomain.DeliveryStatusRetrying,
		strings.TrimSpace(attemptID),
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrDeliveryRetryCompletionUnconfirmed
	}
	if err = r.updateExactTaskLastDeliveryStatus(
		ctx, tx, ownerUserID, jobID, runID, automationdomain.DeliveryStatusRetrying,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) updateExactTaskLastDeliveryStatus(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	jobID string,
	runID string,
	deliveryStatus string,
) error {
	// Migration 00122 backfills this identity from the latest authoritative
	// succeeded/failed/cancelled run (finished_at DESC, run_id DESC). New
	// completions maintain it transactionally, so an older historical retry
	// can update its own run without overwriting the task's current summary.
	_, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE automation_scheduled_tasks
SET last_delivery_status = %s, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND last_completed_run_id = %s
  AND deletion_state = ''`, r.bind(1), r.bind(2), r.bind(3), r.bind(4)),
		nullString(strings.TrimSpace(deliveryStatus)),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(runID),
	)
	return err
}

// DeadLetterFailedRunDelivery 只把调用方读取到的 exact failed attempt 标记为
// dead-letter；若人工 retry 已先领取，不能把 retrying 覆盖回 failed。
func (r *Repository) DeadLetterFailedRunDelivery(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	runID string,
	expectedDeliveryAttempts int,
	deliveryError *string,
	deadLetterAt time.Time,
) error {
	query := fmt.Sprintf(`UPDATE automation_task_runs
SET delivery_status = %s,
    delivery_error = %s,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND delivery_status = %s
  AND delivery_attempts = %s
  AND delivery_dead_letter_at IS NULL`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7), r.bind(8),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.DeliveryStatusFailed,
		nullableString(deliveryError),
		deadLetterAt.UTC(),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(runID),
		automationdomain.DeliveryStatusFailed,
		expectedDeliveryAttempts,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrDeliveryRetryConflict
	}
	return nil
}

// DeadLetterOrphanedPendingRunDelivery closes a terminal pending row whose task
// no longer exists. No delivery claim was taken, so the result is explicitly
// not attempted and excluded from future due scans.
func (r *Repository) DeadLetterOrphanedPendingRunDelivery(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	runID string,
	expectedDeliveryAttempts int,
	deliveryError *string,
	deadLetterAt time.Time,
) error {
	query := fmt.Sprintf(`UPDATE automation_task_runs
SET delivery_status = %s,
    delivery_error = %s,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND status = %s
  AND delivery_status = %s
  AND delivery_attempts = %s
  AND delivery_dead_letter_at IS NULL`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5),
		r.bind(6), r.bind(7), r.bind(8), r.bind(9),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.DeliveryStatusNotAttempted,
		nullableString(deliveryError),
		deadLetterAt.UTC(),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(runID),
		automationdomain.RunStatusSucceeded,
		automationdomain.DeliveryStatusPending,
		expectedDeliveryAttempts,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrDeliveryRetryConflict
	}
	return nil
}

// ListDueDeliveryRetries 列出到期的 terminal pending 首投与 failed retry run。
func (r *Repository) ListDueDeliveryRetries(ctx context.Context, now time.Time, maxAttempts int, limit int) ([]automationdomain.ScheduledTaskRun, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT` + scheduledTaskRunSelectColumns + `
FROM automation_task_runs
WHERE delivery_dead_letter_at IS NULL
  AND (
      (delivery_status = ` + r.bind(1) + `
       AND status = ` + r.bind(2) + `
       AND delivery_attempts = 0
       AND updated_at <= ` + r.bind(3) + `)
      OR
      (delivery_status = ` + r.bind(4) + `
       AND delivery_attempts < ` + r.bind(5) + `
       AND (delivery_next_attempt_at IS NULL OR delivery_next_attempt_at <= ` + r.bind(6) + `))
  )
ORDER BY COALESCE(delivery_next_attempt_at, updated_at), updated_at, run_id
LIMIT ` + r.bind(7)
	rows, err := r.db.QueryContext(
		ctx,
		query,
		automationdomain.DeliveryStatusPending,
		automationdomain.RunStatusSucceeded,
		now.UTC(),
		automationdomain.DeliveryStatusFailed,
		maxAttempts,
		now.UTC(),
		limit,
	)
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

func initialRunDeliveryStatus(input RunPendingInput) string {
	if deliveryStatus := strings.TrimSpace(input.DeliveryStatus); deliveryStatus != "" {
		return deliveryStatus
	}
	switch strings.TrimSpace(input.DeliveryMode) {
	case "", automationdomain.DeliveryModeNone:
		return automationdomain.DeliveryStatusNotRequired
	default:
		return automationdomain.DeliveryStatusPending
	}
}

func finishedRunDeliveryStatus(input RunFinishInput) string {
	if deliveryStatus := strings.TrimSpace(input.DeliveryStatus); deliveryStatus != "" {
		return deliveryStatus
	}
	switch strings.TrimSpace(input.Status) {
	case automationdomain.RunStatusPending, automationdomain.RunStatusRunning, automationdomain.RunStatusQueuedToMain:
		return automationdomain.DeliveryStatusPending
	case automationdomain.RunStatusSucceeded:
		return automationdomain.DeliveryStatusNotRequired
	case automationdomain.RunStatusFailed, automationdomain.RunStatusCancelled, automationdomain.RunStatusSkipped:
		return automationdomain.DeliveryStatusNotAttempted
	default:
		return automationdomain.DeliveryStatusNotAttempted
	}
}
