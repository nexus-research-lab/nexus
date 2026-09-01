// INPUT: exact owner/job/run terminal observation 与完成前计算出的 runtime 快照。
// OUTPUT: 同事务 terminal run、exact active task runtime 释放与最后完成 run 身份。
// POS: Automation execution→delivery 两阶段边界；提交失败时外部投递不得开始。
package automation

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// RunTerminalCommitInput 描述 execution 结束后、任何外投发生前的原子提交。
type RunTerminalCommitInput struct {
	OwnerUserID                  string
	JobID                        string
	ExpectedConfigurationVersion int64
	Finish                       RunFinishInput
	Runtime                      JobRuntimeUpdateInput
}

// RunTerminalCommitResult 区分 run 已提交与 task runtime 是否由该 run 持有。
type RunTerminalCommitResult struct {
	Committed      bool
	RuntimeUpdated bool
}

// DeletingRunTerminalCommitInput is the only terminal path permitted after a
// task deletion claim. The private token fences the exact durable claim.
type DeletingRunTerminalCommitInput struct {
	OwnerUserID   string
	JobID         string
	DeletionToken string
	Finish        RunFinishInput
}

// CommitRunTerminalAndRuntime 先 exact 提交 terminal run；只有 task 仍由同一
// run 持有时才释放 runtime。旧 overlap run 可以结束，但不能覆盖新 run。
func (r *Repository) CommitRunTerminalAndRuntime(
	ctx context.Context,
	input RunTerminalCommitInput,
) (RunTerminalCommitResult, error) {
	ownerUserID := strings.TrimSpace(input.OwnerUserID)
	jobID := strings.TrimSpace(input.JobID)
	runID := strings.TrimSpace(input.Finish.RunID)
	terminalStatus := strings.TrimSpace(input.Finish.Status)
	if ownerUserID == "" || jobID == "" || runID == "" ||
		(terminalStatus != automationdomain.RunStatusSucceeded &&
			terminalStatus != automationdomain.RunStatusFailed &&
			terminalStatus != automationdomain.RunStatusCancelled) {
		return RunTerminalCommitResult{}, automationdomain.ErrRunCompletionConflict
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return RunTerminalCommitResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	committed, err := r.commitTerminalRun(ctx, tx, ownerUserID, jobID, input.Finish)
	if err != nil {
		return RunTerminalCommitResult{}, err
	}
	if !committed {
		return RunTerminalCommitResult{}, automationdomain.ErrRunCompletionConflict
	}

	var runningRunID string
	var deletionState string
	err = tx.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT COALESCE(running_run_id, ''), COALESCE(deletion_state, '')
FROM automation_scheduled_tasks
WHERE owner_user_id = %s AND job_id = %s`, r.bind(1), r.bind(2)),
		ownerUserID,
		jobID,
	).Scan(&runningRunID, &deletionState)
	if err != nil {
		if err == sql.ErrNoRows {
			return RunTerminalCommitResult{}, automationdomain.ErrTaskDeleting
		}
		return RunTerminalCommitResult{}, err
	}
	if strings.TrimSpace(deletionState) != "" {
		return RunTerminalCommitResult{}, automationdomain.ErrTaskDeleting
	}

	runtimeUpdated := false
	if strings.TrimSpace(runningRunID) == runID {
		nextRunningRunID, nextRunningStartedAt, nextErr := r.latestActiveRunIdentity(ctx, tx, ownerUserID, jobID)
		if nextErr != nil {
			return RunTerminalCommitResult{}, nextErr
		}
		runtimeUpdated, err = r.releaseCompletedTaskRuntime(
			ctx, tx, input, nextRunningRunID, nextRunningStartedAt,
		)
		if err != nil {
			return RunTerminalCommitResult{}, err
		}
		if !runtimeUpdated {
			return RunTerminalCommitResult{}, automationdomain.ErrRunCompletionConflict
		}
	} else if err = r.updateCompletedTaskFacts(ctx, tx, input); err != nil {
		return RunTerminalCommitResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return RunTerminalCommitResult{}, err
	}
	return RunTerminalCommitResult{Committed: true, RuntimeUpdated: runtimeUpdated}, nil
}

// CommitDeletingRunTerminal preserves an exact execution result that arrives
// after deletion was claimed. It never updates task runtime/summary and always
// suppresses delivery as not_attempted + dead-letter.
func (r *Repository) CommitDeletingRunTerminal(
	ctx context.Context,
	input DeletingRunTerminalCommitInput,
) error {
	ownerUserID := strings.TrimSpace(input.OwnerUserID)
	jobID := strings.TrimSpace(input.JobID)
	runID := strings.TrimSpace(input.Finish.RunID)
	token := strings.TrimSpace(input.DeletionToken)
	terminalStatus := strings.TrimSpace(input.Finish.Status)
	if ownerUserID == "" || jobID == "" || runID == "" || token == "" ||
		(terminalStatus != automationdomain.RunStatusSucceeded &&
			terminalStatus != automationdomain.RunStatusFailed &&
			terminalStatus != automationdomain.RunStatusCancelled) {
		return automationdomain.ErrRunCompletionConflict
	}
	deliveryError := "result delivery was not attempted because task deletion is in progress"
	query := fmt.Sprintf(`UPDATE automation_task_runs
SET status = %s,
    finished_at = %s,
    error_message = %s,
    session_id = %s,
    message_count = %s,
    result_summary = %s,
    assistant_text = %s,
    result_text = %s,
    artifact_path = %s,
    delivery_status = %s,
    delivery_error = %s,
    delivered_at = NULL,
    delivery_attempts = 0,
    delivery_attempt_id = NULL,
    delivery_attempt_started_at = NULL,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = %s,
    block_state = '',
    blocked_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND status IN (%s, %s, %s)
  AND EXISTS (
      SELECT 1 FROM automation_scheduled_tasks task
      WHERE task.owner_user_id = %s
        AND task.job_id = %s
        AND COALESCE(task.deletion_state, '') <> ''
        AND task.deletion_token = %s
  )`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9), r.bind(10), r.bind(11), r.bind(12),
		r.bind(13), r.bind(14), r.bind(15), r.bind(16), r.bind(17), r.bind(18),
		r.bind(19), r.bind(20), r.bind(21),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		terminalStatus,
		input.Finish.FinishedAt.UTC(),
		nullableString(input.Finish.ErrorMessage),
		nullableString(input.Finish.SessionID),
		input.Finish.MessageCount,
		nullableString(input.Finish.ResultSummary),
		nullableString(input.Finish.AssistantText),
		nullableString(input.Finish.ResultText),
		nullableString(input.Finish.ArtifactPath),
		automationdomain.DeliveryStatusNotAttempted,
		deliveryError,
		input.Finish.FinishedAt.UTC(),
		ownerUserID,
		jobID,
		runID,
		automationdomain.RunStatusPending,
		automationdomain.RunStatusRunning,
		automationdomain.RunStatusQueuedToMain,
		ownerUserID,
		jobID,
		token,
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrRunCompletionConflict
	}
	return nil
}

// updateCompletedTaskFacts records a completed overlap run without changing the
// runtime identity currently held by another active run. A completion observed
// later wins only when its finished_at is at least as new as the stored fact.
func (r *Repository) updateCompletedTaskFacts(
	ctx context.Context,
	tx *sql.Tx,
	input RunTerminalCommitInput,
) error {
	runtime := input.Runtime
	_, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE automation_scheduled_tasks
SET last_run_at = %s,
    last_run_status = %s,
    failure_streak = %s,
    last_error = %s,
    last_delivery_status = %s,
    last_completed_run_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND deletion_state = ''
  AND (last_run_at IS NULL OR last_run_at <= %s)`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5),
			r.bind(6), r.bind(7), r.bind(8), r.bind(9),
		),
		nullableTime(runtime.LastRunAt),
		nullString(runtime.LastRunStatus),
		runtime.FailureStreak,
		nullableString(runtime.LastError),
		nullString(runtime.LastDeliveryStatus),
		strings.TrimSpace(input.Finish.RunID),
		strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.JobID),
		input.Finish.FinishedAt.UTC(),
	)
	return err
}

func (r *Repository) latestActiveRunIdentity(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	jobID string,
) (string, *time.Time, error) {
	var runID string
	var startedAt sql.NullTime
	err := tx.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT run_id, started_at
FROM automation_task_runs
WHERE owner_user_id = %s
  AND job_id = %s
  AND status IN (%s, %s, %s)
ORDER BY started_at DESC, run_id DESC
LIMIT 1`, r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5)),
		ownerUserID,
		jobID,
		automationdomain.RunStatusPending,
		automationdomain.RunStatusRunning,
		automationdomain.RunStatusQueuedToMain,
	).Scan(&runID, &startedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil, nil
		}
		return "", nil, err
	}
	return strings.TrimSpace(runID), nullTimePointer(startedAt), nil
}

func (r *Repository) commitTerminalRun(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	jobID string,
	input RunFinishInput,
) (bool, error) {
	query := fmt.Sprintf(`UPDATE automation_task_runs
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
    delivered_at = NULL,
    delivery_attempts = 0,
    delivery_attempt_id = NULL,
    delivery_attempt_started_at = NULL,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = NULL,
    block_state = '',
    blocked_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND status IN (%s, %s, %s)
  AND EXISTS (
      SELECT 1 FROM automation_scheduled_tasks task
      WHERE task.owner_user_id = %s
        AND task.job_id = %s
        AND COALESCE(task.deletion_state, '') = ''
  )`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9), r.bind(10), r.bind(11), r.bind(12),
		r.bind(13), r.bind(14), r.bind(15), r.bind(16), r.bind(17), r.bind(18),
		r.bind(19), r.bind(20),
	)
	result, err := tx.ExecContext(
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
		ownerUserID,
		jobID,
		strings.TrimSpace(input.RunID),
		automationdomain.RunStatusPending,
		automationdomain.RunStatusRunning,
		automationdomain.RunStatusQueuedToMain,
		ownerUserID,
		jobID,
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *Repository) releaseCompletedTaskRuntime(
	ctx context.Context,
	tx *sql.Tx,
	input RunTerminalCommitInput,
	nextRunningRunID string,
	nextRunningStartedAt *time.Time,
) (bool, error) {
	runtime := input.Runtime
	baseQuery := fmt.Sprintf(`UPDATE automation_scheduled_tasks
SET next_run_at = %s,
    running_run_id = %s,
    running_started_at = %s,
    last_run_at = %s,
    last_run_status = %s,
    failure_streak = %s,
    last_error = %s,
    last_delivery_status = %s,
    last_completed_run_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND COALESCE(running_run_id, '') = %s
  AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9), r.bind(10), r.bind(11), r.bind(12),
	)
	args := []any{
		nullableTime(runtime.NextRunAt),
		nullString(nextRunningRunID),
		nullableTime(nextRunningStartedAt),
		nullableTime(runtime.LastRunAt),
		nullString(runtime.LastRunStatus),
		runtime.FailureStreak,
		nullableString(runtime.LastError),
		nullString(runtime.LastDeliveryStatus),
		strings.TrimSpace(input.Finish.RunID),
		strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.Finish.RunID),
	}
	if input.ExpectedConfigurationVersion > 0 {
		args = append(args, input.ExpectedConfigurationVersion)
		versioned := baseQuery + " AND configuration_version = " + r.bind(len(args))
		result, err := tx.ExecContext(ctx, versioned, args...)
		if err != nil {
			return false, err
		}
		count, err := result.RowsAffected()
		if err != nil || count == 1 {
			return count == 1, err
		}
	}

	// 配置在执行期间变化时保留新 schedule/next_run_at，只释放 exact run 并
	// 写入本次完成事实，避免旧配置快照覆盖用户刚保存的任务定义。
	narrowQuery := fmt.Sprintf(`UPDATE automation_scheduled_tasks
SET running_run_id = %s,
    running_started_at = %s,
    last_run_at = %s,
    last_run_status = %s,
    failure_streak = %s,
    last_error = %s,
    last_delivery_status = %s,
    last_completed_run_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND COALESCE(running_run_id, '') = %s
  AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9), r.bind(10), r.bind(11),
	)
	result, err := tx.ExecContext(
		ctx,
		narrowQuery,
		nullString(nextRunningRunID),
		nullableTime(nextRunningStartedAt),
		nullableTime(runtime.LastRunAt),
		nullString(runtime.LastRunStatus),
		runtime.FailureStreak,
		nullableString(runtime.LastError),
		nullString(runtime.LastDeliveryStatus),
		strings.TrimSpace(input.Finish.RunID),
		strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.Finish.RunID),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
