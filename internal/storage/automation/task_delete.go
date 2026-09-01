// INPUT: owner/task/version、durable deletion token、可选 review/version CAS 与删除审计快照。
// OUTPUT: 先 claim 删除意图，再以 exact token 和可选 fence 原子收尾 run、权限、投递、事件和任务定义。
// POS: Automation 删除的数据库真相；外部中断只能发生在 claim 提交之后。
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

// TaskDeletionClaimInput 描述一次不可逆删除 claim。
type TaskDeletionClaimInput struct {
	OwnerUserID     string
	JobID           string
	ExpectedVersion *int64
	DeletionToken   string
	ClaimedAt       time.Time
}

// TaskDeletionClaimResult 返回 claim 后的权威任务和 exact continuation token。
type TaskDeletionClaimResult struct {
	Task    automationdomain.ScheduledTask
	Token   string
	Claimed bool
}

// TaskDeleteFinalizationInput 描述 exact deletion claim 的最终数据库提交。
type TaskDeleteFinalizationInput struct {
	OwnerUserID                  string
	JobID                        string
	DeletionToken                string
	ExpectedDeletionState        string
	ExpectedConfigurationVersion *int64
	FinishedAt                   time.Time
	ActiveRunMessage             string
	DeliveryDeadLetter           time.Time
	DeliveryError                string
	UnconfirmedDeliveryError     string
	PendingDeliveryError         string
	DeleteEvent                  TaskEventInput
}

// TaskDeleteFinalizationResult 返回提交后需要通知或刷新投影的精确事实。
type TaskDeleteFinalizationResult struct {
	CancelledRuns                []automationdomain.ScheduledTaskRun
	SupersededPermissionRequests []automationdomain.AutomationPermissionRequest
	DeadLetteredDeliveryRunIDs   []string
	UnconfirmedDeliveryRunIDs    []string
	NotAttemptedDeliveryRunIDs   []string
}

// ClaimScheduledTaskDeletion 在任何外部清理前持久化删除意图。已经 deleting
// 的任务返回原 token，使崩溃恢复和跨实例重试继续同一个删除操作。
func (r *Repository) ClaimScheduledTaskDeletion(
	ctx context.Context,
	input TaskDeletionClaimInput,
) (TaskDeletionClaimResult, error) {
	if input.ExpectedVersion != nil && *input.ExpectedVersion < 1 {
		return TaskDeletionClaimResult{}, automationdomain.ErrConfigurationVersionConflict
	}
	ownerUserID := strings.TrimSpace(input.OwnerUserID)
	jobID := strings.TrimSpace(input.JobID)
	token := strings.TrimSpace(input.DeletionToken)
	if ownerUserID == "" || jobID == "" || token == "" {
		return TaskDeletionClaimResult{}, errors.New("task deletion claim identity is required")
	}
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET enabled = %s, next_run_at = NULL, deletion_state = %s, deletion_token = %s,
    deletion_claimed_at = %s, configuration_version = configuration_version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s AND job_id = %s AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
	)
	args := []any{false, automationdomain.TaskDeletionStateDeleting, token, input.ClaimedAt.UTC(), ownerUserID, jobID}
	if input.ExpectedVersion != nil {
		args = append(args, *input.ExpectedVersion)
		query += " AND configuration_version = " + r.bind(7)
	}
	result, err := r.execWithRetry(ctx, query, args...)
	if err != nil {
		return TaskDeletionClaimResult{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return TaskDeletionClaimResult{}, err
	}
	current, err := r.GetScheduledTask(ctx, ownerUserID, jobID)
	if err != nil {
		return TaskDeletionClaimResult{}, err
	}
	if current == nil {
		return TaskDeletionClaimResult{}, automationdomain.ErrJobNotFound
	}
	if count == 1 {
		return TaskDeletionClaimResult{Task: *current, Token: token, Claimed: true}, nil
	}
	if strings.TrimSpace(current.DeletionState) != "" &&
		strings.TrimSpace(current.DeletionToken) != "" {
		return TaskDeletionClaimResult{Task: *current, Token: strings.TrimSpace(current.DeletionToken)}, nil
	}
	if input.ExpectedVersion != nil {
		return TaskDeletionClaimResult{}, automationdomain.ErrConfigurationVersionConflict
	}
	return TaskDeletionClaimResult{}, automationdomain.ErrTaskDeleting
}

// MarkTaskDeletionReviewRequired 只按 exact token 持久化人工处理状态；它不撤销 claim。
func (r *Repository) MarkTaskDeletionReviewRequired(ctx context.Context, ownerUserID string, jobID string, token string) error {
	result, err := r.execWithRetry(ctx, `UPDATE automation_scheduled_tasks
SET deletion_state = `+r.bind(1)+`, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = `+r.bind(2)+` AND job_id = `+r.bind(3)+`
  AND deletion_state <> '' AND deletion_token = `+r.bind(4),
		automationdomain.TaskDeletionStateReviewRequired,
		strings.TrimSpace(ownerUserID), strings.TrimSpace(jobID), strings.TrimSpace(token))
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

// ListTaskDeletionCleanupRuns 只读取删除清理需要的非终态 run，不扫描历史。
func (r *Repository) ListTaskDeletionCleanupRuns(ctx context.Context, ownerUserID string, jobID string) ([]automationdomain.ScheduledTaskRun, error) {
	query := `SELECT` + scheduledTaskRunSelectColumns + `
FROM automation_task_runs
WHERE owner_user_id = ` + r.bind(1) + ` AND job_id = ` + r.bind(2) + `
	  AND status IN (` + r.bind(3) + `, ` + r.bind(4) + `, ` + r.bind(5) + `)
ORDER BY created_at ASC, run_id ASC`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(jobID),
		automationdomain.RunStatusPending, automationdomain.RunStatusRunning, automationdomain.RunStatusQueuedToMain)
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

// ListReviewRequiredTaskDeletionCleanupRuns batches the bounded scheduler audit
// for every review-required task. The scheduler groups the result by exact
// owner/job, avoiding one query per exceptional task as the catalog grows.
func (r *Repository) ListReviewRequiredTaskDeletionCleanupRuns(
	ctx context.Context,
) ([]automationdomain.ScheduledTaskRun, error) {
	query := `SELECT` + scheduledTaskRunSelectColumns + `
FROM automation_task_runs AS run
WHERE run.status IN (` + r.bind(1) + `, ` + r.bind(2) + `, ` + r.bind(3) + `)
  AND EXISTS (
      SELECT 1
      FROM automation_scheduled_tasks AS task
      WHERE task.owner_user_id = run.owner_user_id
        AND task.job_id = run.job_id
        AND task.deletion_state = ` + r.bind(4) + `
  )
ORDER BY run.owner_user_id ASC, run.job_id ASC, run.created_at ASC, run.run_id ASC`
	rows, err := r.db.QueryContext(
		ctx,
		query,
		automationdomain.RunStatusPending,
		automationdomain.RunStatusRunning,
		automationdomain.RunStatusQueuedToMain,
		automationdomain.TaskDeletionStateReviewRequired,
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

// FinalizeScheduledTaskDeletion 以 exact token 原子收尾所有关联事实并删除任务。
func (r *Repository) FinalizeScheduledTaskDeletion(ctx context.Context, input TaskDeleteFinalizationInput) (TaskDeleteFinalizationResult, error) {
	attempts := automationWriteRetryAttempts
	if r.isPostgres {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := r.finalizeScheduledTaskDeletionOnce(ctx, input)
		if err == nil || !isSQLiteLockedError(err) || attempt == attempts-1 {
			return result, err
		}
		timer := time.NewTimer(automationWriteRetryDelay * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return TaskDeleteFinalizationResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	return TaskDeleteFinalizationResult{}, errors.New("scheduled task deletion finalization retries exhausted")
}

func (r *Repository) finalizeScheduledTaskDeletionOnce(ctx context.Context, input TaskDeleteFinalizationInput) (TaskDeleteFinalizationResult, error) {
	ownerUserID := strings.TrimSpace(input.OwnerUserID)
	jobID := strings.TrimSpace(input.JobID)
	token := strings.TrimSpace(input.DeletionToken)
	if ownerUserID == "" || jobID == "" || token == "" {
		return TaskDeleteFinalizationResult{}, errors.New("task deletion finalization identity is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = r.requireExactDeletionClaimTx(
		ctx,
		tx,
		ownerUserID,
		jobID,
		token,
		strings.TrimSpace(input.ExpectedDeletionState),
		input.ExpectedConfigurationVersion,
	); err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	result := TaskDeleteFinalizationResult{}
	result.CancelledRuns, err = r.listDeletionRunsTx(ctx, tx, ownerUserID, jobID)
	if err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	result.SupersededPermissionRequests, err = r.listPendingPermissionRequestsForTaskTx(ctx, tx, ownerUserID, jobID)
	if err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	result.DeadLetteredDeliveryRunIDs, result.UnconfirmedDeliveryRunIDs,
		result.NotAttemptedDeliveryRunIDs, err = r.listDeletionDeliveryRunIDsTx(ctx, tx, ownerUserID, jobID)
	if err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	if err = r.cancelDeletionRunsTx(ctx, tx, input); err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	if err = r.supersedeDeletionPermissionsTx(ctx, tx, ownerUserID, jobID); err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	if err = r.deadLetterDeletionDeliveriesTx(ctx, tx, input); err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	if input.DeleteEvent.Detail == nil {
		input.DeleteEvent.Detail = map[string]any{}
	}
	if len(result.CancelledRuns) > 0 {
		cancelledRunIDs := make([]string, 0, len(result.CancelledRuns))
		for _, run := range result.CancelledRuns {
			cancelledRunIDs = append(cancelledRunIDs, strings.TrimSpace(run.RunID))
		}
		input.DeleteEvent.Detail["cancelled_run_ids"] = cancelledRunIDs
		input.DeleteEvent.Detail["cancelled_active_run"] = true
	}
	if len(result.DeadLetteredDeliveryRunIDs) > 0 {
		input.DeleteEvent.Detail["dead_lettered_delivery_run_ids"] = result.DeadLetteredDeliveryRunIDs
	}
	if len(result.UnconfirmedDeliveryRunIDs) > 0 {
		input.DeleteEvent.Detail["unconfirmed_delivery_run_ids"] = result.UnconfirmedDeliveryRunIDs
		input.DeleteEvent.Detail["delivery_outcome_unconfirmed"] = true
	}
	if len(result.NotAttemptedDeliveryRunIDs) > 0 {
		input.DeleteEvent.Detail["not_attempted_delivery_run_ids"] = result.NotAttemptedDeliveryRunIDs
	}
	if err = r.insertTaskEventTx(ctx, tx, input.DeleteEvent); err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	deleteQuery := `DELETE FROM automation_scheduled_tasks
WHERE owner_user_id = ` + r.bind(1) + ` AND job_id = ` + r.bind(2) + `
	  AND deletion_state <> '' AND deletion_token = ` + r.bind(3)
	deleteArgs := []any{ownerUserID, jobID, token}
	if expectedState := strings.TrimSpace(input.ExpectedDeletionState); expectedState != "" {
		deleteArgs = append(deleteArgs, expectedState)
		deleteQuery += ` AND deletion_state = ` + r.bind(len(deleteArgs))
	}
	if input.ExpectedConfigurationVersion != nil {
		deleteArgs = append(deleteArgs, *input.ExpectedConfigurationVersion)
		deleteQuery += ` AND configuration_version = ` + r.bind(len(deleteArgs))
	}
	deleted, err := tx.ExecContext(ctx, deleteQuery, deleteArgs...)
	if err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	count, err := deleted.RowsAffected()
	if err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	if count != 1 {
		return TaskDeleteFinalizationResult{}, automationdomain.ErrTaskDeleting
	}
	if err = tx.Commit(); err != nil {
		return TaskDeleteFinalizationResult{}, err
	}
	return result, nil
}

func (r *Repository) requireExactDeletionClaimTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	jobID string,
	token string,
	expectedState string,
	expectedVersion *int64,
) error {
	var storedToken, storedState string
	var storedVersion int64
	query := `SELECT COALESCE(deletion_token, ''), COALESCE(deletion_state, ''), configuration_version
FROM automation_scheduled_tasks
WHERE owner_user_id = ` + r.bind(1) + ` AND job_id = ` + r.bind(2) + ` AND deletion_state <> ''`
	if r.isPostgres {
		query += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, query, ownerUserID, jobID).Scan(&storedToken, &storedState, &storedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return automationdomain.ErrJobNotFound
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(storedToken) != token {
		return automationdomain.ErrTaskDeleting
	}
	if expectedState != "" && strings.TrimSpace(storedState) != expectedState {
		return automationdomain.ErrTaskDeletionReviewConflict
	}
	if expectedVersion != nil && (*expectedVersion < 1 || storedVersion != *expectedVersion) {
		return automationdomain.ErrConfigurationVersionConflict
	}
	return nil
}

func (r *Repository) listDeletionRunsTx(ctx context.Context, tx *sql.Tx, ownerUserID string, jobID string) ([]automationdomain.ScheduledTaskRun, error) {
	query := `SELECT` + scheduledTaskRunSelectColumns + ` FROM automation_task_runs
WHERE owner_user_id = ` + r.bind(1) + ` AND job_id = ` + r.bind(2) + `
	  AND status IN (` + r.bind(3) + `, ` + r.bind(4) + `, ` + r.bind(5) + `)
ORDER BY created_at ASC, run_id ASC`
	rows, err := tx.QueryContext(ctx, query, ownerUserID, jobID,
		automationdomain.RunStatusPending, automationdomain.RunStatusRunning, automationdomain.RunStatusQueuedToMain)
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

func (r *Repository) listDeletionDeliveryRunIDsTx(ctx context.Context, tx *sql.Tx, ownerUserID string, jobID string) ([]string, []string, []string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT run_id, delivery_status FROM automation_task_runs
WHERE owner_user_id = `+r.bind(1)+` AND job_id = `+r.bind(2)+`
  AND delivery_dead_letter_at IS NULL
  AND (delivery_status IN ('failed', 'retrying')
       OR (delivery_status = 'pending' AND status NOT IN ('pending', 'running', 'queued_to_main_session')))`, ownerUserID, jobID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	failed := make([]string, 0)
	unconfirmed := make([]string, 0)
	notAttempted := make([]string, 0)
	for rows.Next() {
		var runID, status string
		if err = rows.Scan(&runID, &status); err != nil {
			return nil, nil, nil, err
		}
		switch strings.TrimSpace(status) {
		case automationdomain.DeliveryStatusRetrying:
			unconfirmed = append(unconfirmed, strings.TrimSpace(runID))
		case automationdomain.DeliveryStatusPending:
			notAttempted = append(notAttempted, strings.TrimSpace(runID))
		default:
			failed = append(failed, strings.TrimSpace(runID))
		}
	}
	return failed, unconfirmed, notAttempted, rows.Err()
}

func (r *Repository) cancelDeletionRunsTx(ctx context.Context, tx *sql.Tx, input TaskDeleteFinalizationInput) error {
	_, err := tx.ExecContext(ctx, `UPDATE automation_task_runs
SET status = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') THEN `+r.bind(1)+` ELSE status END,
    finished_at = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') THEN `+r.bind(2)+` ELSE finished_at END,
    error_message = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') THEN `+r.bind(3)+` ELSE error_message END,
    delivery_status = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') AND delivery_status = 'pending' THEN `+r.bind(4)+` ELSE delivery_status END,
    delivery_error = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') AND delivery_status = 'pending' THEN `+r.bind(5)+` ELSE delivery_error END,
    delivery_next_attempt_at = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') AND delivery_status = 'pending' THEN NULL ELSE delivery_next_attempt_at END,
    delivery_dead_letter_at = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') AND delivery_status = 'pending' THEN `+r.bind(6)+` ELSE delivery_dead_letter_at END,
    delivery_attempt_id = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') AND delivery_status = 'pending' THEN NULL ELSE delivery_attempt_id END,
    delivery_attempt_started_at = CASE WHEN status IN ('pending', 'running', 'queued_to_main_session') AND delivery_status = 'pending' THEN NULL ELSE delivery_attempt_started_at END,
    block_state = '', blocked_request_id = NULL, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = `+r.bind(7)+` AND job_id = `+r.bind(8)+`
  AND (status IN ('pending', 'running', 'queued_to_main_session') OR block_state <> '')`,
		automationdomain.RunStatusCancelled, input.FinishedAt.UTC(), nullString(strings.TrimSpace(input.ActiveRunMessage)),
		automationdomain.DeliveryStatusNotAttempted, nullString(strings.TrimSpace(input.PendingDeliveryError)), input.DeliveryDeadLetter.UTC(),
		strings.TrimSpace(input.OwnerUserID), strings.TrimSpace(input.JobID))
	return err
}

func (r *Repository) supersedeDeletionPermissionsTx(ctx context.Context, tx *sql.Tx, ownerUserID string, jobID string) error {
	_, err := tx.ExecContext(ctx, `UPDATE automation_permission_requests
SET status = `+r.bind(1)+`, decision = NULL, resolved_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = `+r.bind(2)+` AND job_id = `+r.bind(3)+` AND status = `+r.bind(4),
		automationdomain.PermissionRequestStatusSuperseded, ownerUserID, jobID, automationdomain.PermissionRequestStatusPending)
	return err
}

func (r *Repository) deadLetterDeletionDeliveriesTx(ctx context.Context, tx *sql.Tx, input TaskDeleteFinalizationInput) error {
	_, err := tx.ExecContext(ctx, `UPDATE automation_task_runs
SET delivery_status = `+r.bind(1)+`, delivery_error = `+r.bind(2)+`, delivered_at = NULL,
    delivery_next_attempt_at = NULL, delivery_dead_letter_at = `+r.bind(3)+`, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = `+r.bind(4)+` AND job_id = `+r.bind(5)+`
  AND delivery_dead_letter_at IS NULL AND delivery_status = 'failed'`,
		automationdomain.DeliveryStatusFailed, nullString(strings.TrimSpace(input.DeliveryError)), input.DeliveryDeadLetter.UTC(),
		strings.TrimSpace(input.OwnerUserID), strings.TrimSpace(input.JobID))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE automation_task_runs
SET delivery_error = `+r.bind(1)+`, delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = `+r.bind(2)+`, delivery_attempt_id = NULL,
    delivery_attempt_started_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = `+r.bind(3)+` AND job_id = `+r.bind(4)+`
  AND delivery_dead_letter_at IS NULL AND delivery_status = 'retrying'`,
		nullString(strings.TrimSpace(input.UnconfirmedDeliveryError)), input.DeliveryDeadLetter.UTC(),
		strings.TrimSpace(input.OwnerUserID), strings.TrimSpace(input.JobID))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE automation_task_runs
SET delivery_status = `+r.bind(1)+`, delivery_error = `+r.bind(2)+`, delivered_at = NULL,
    delivery_next_attempt_at = NULL, delivery_dead_letter_at = `+r.bind(3)+`, updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = `+r.bind(4)+` AND job_id = `+r.bind(5)+`
  AND delivery_dead_letter_at IS NULL AND delivery_status = 'pending'
  AND status NOT IN ('pending', 'running', 'queued_to_main_session')`,
		automationdomain.DeliveryStatusNotAttempted, nullString(strings.TrimSpace(input.PendingDeliveryError)),
		input.DeliveryDeadLetter.UTC(), strings.TrimSpace(input.OwnerUserID), strings.TrimSpace(input.JobID))
	return err
}

func (r *Repository) insertTaskEventTx(ctx context.Context, tx *sql.Tx, input TaskEventInput) error {
	detail := input.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	body, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO automation_task_events (
    event_id, job_id, owner_user_id, agent_id, action, actor_user_id,
    actor_agent_id, run_id, detail_json, created_at
) VALUES (`+r.bindList(9)+`, CURRENT_TIMESTAMP)`,
		strings.TrimSpace(input.EventID), strings.TrimSpace(input.JobID), strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.AgentID), strings.TrimSpace(input.Action), nullString(strings.TrimSpace(input.ActorUserID)),
		nullString(strings.TrimSpace(input.ActorAgentID)), nullString(strings.TrimSpace(input.RunID)), string(body))
	return err
}
