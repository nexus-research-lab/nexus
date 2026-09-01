// INPUT: exact owner/job/run、配置与权限快照，以及可选的人工命令 request identity。
// OUTPUT: 条件 runtime claim、首条 run ledger 原子提交与 typed CAS/replay 结果。
// POS: Automation 执行受理事务边界；dispatch 只能发生在本层 commit 成功之后。
package automation

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

// JobRuntimeUpdateInput 表示定时任务调度运行态的持久化快照。
type JobRuntimeUpdateInput struct {
	JobID              string
	NextRunAt          *time.Time
	RunningRunID       string
	RunningStartedAt   *time.Time
	LastRunAt          *time.Time
	LastRunStatus      string
	FailureStreak      int
	LastError          *string
	LastDeliveryStatus string
}

// JobRuntimeClaimInput 表示一次调度执行的领取请求。
type JobRuntimeClaimInput struct {
	OwnerUserID                  string
	JobID                        string
	RunID                        string
	StartedAt                    time.Time
	NextRunAt                    *time.Time
	OverlapPolicy                string
	AllowDisabled                bool
	ExpectedConfigurationVersion *int64
	ExpectedPermissionRevision   *int
	ExpectedPermissionState      *string
	ExpectedPermissionRequestID  *string
	ResetDeniedPermission        bool
}

// InitialRunClaimInput 把 task runtime claim 与对应首条 run ledger 绑定。
type InitialRunClaimInput struct {
	Runtime            JobRuntimeClaimInput
	Run                RunPendingInput
	OverlapTerminalRun *RunPendingInput
}

// InitialRunClaimResult 区分当前事务领取、同一人工请求重放和普通 CAS miss。
type InitialRunClaimResult struct {
	Claimed  bool
	Replayed bool
	Terminal bool
	RunID    string
}

// JobRuntimeRecoveryInput 把 exact run 终态与任务运行占用收口到同一事务。
type JobRuntimeRecoveryInput struct {
	OwnerUserID  string
	JobID        string
	RunID        string
	FinishedAt   time.Time
	ErrorMessage *string
	Runtime      JobRuntimeUpdateInput
}

// JobRuntimePermissionPauseInput 只允许暂停 exact run 或尚未被其他 run 领取的任务。
type JobRuntimePermissionPauseInput struct {
	OwnerUserID          string
	JobID                string
	ExpectedRunID        string
	NextRunningRunID     string
	NextRunningStartedAt *time.Time
	LastRunStatus        string
	LastError            *string
}

// ClaimScheduledTaskRuntime 通过条件更新领取一次任务执行权。
func (r *Repository) ClaimScheduledTaskRuntime(ctx context.Context, input JobRuntimeClaimInput) (bool, error) {
	query, args, err := r.runtimeClaimQuery(input)
	if err != nil {
		return false, err
	}
	result, err := r.execWithRetry(ctx, query, args...)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count > 0 || (input.ExpectedConfigurationVersion == nil &&
		input.ExpectedPermissionRevision == nil &&
		input.ExpectedPermissionState == nil &&
		input.ExpectedPermissionRequestID == nil) {
		return count > 0, nil
	}
	return false, r.runtimeClaimMiss(ctx, input)
}

func (r *Repository) runtimeClaimQuery(input JobRuntimeClaimInput) (string, []any, error) {
	args := []any{
		nullString(input.RunID),
		input.StartedAt.UTC(),
		nullableTime(input.NextRunAt),
	}
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET running_run_id = %s,
    running_started_at = %s,
    next_run_at = %s`,
		r.bind(1),
		r.bind(2),
		r.bind(3),
	)
	if input.ResetDeniedPermission {
		if input.ExpectedPermissionState == nil ||
			strings.TrimSpace(*input.ExpectedPermissionState) != automationdomain.TaskPermissionStateDenied ||
			input.ExpectedPermissionRequestID == nil ||
			strings.TrimSpace(*input.ExpectedPermissionRequestID) != "" {
			return "", nil, automationdomain.ErrPermissionRequestStale
		}
		args = append(args, automationdomain.TaskPermissionStateReady)
		query += fmt.Sprintf(",\n    permission_state = %s,\n    pending_permission_request_id = NULL", r.bind(len(args)))
	}
	args = append(args, strings.TrimSpace(input.JobID))
	query += fmt.Sprintf(",\n    updated_at = CURRENT_TIMESTAMP\nWHERE job_id = %s\n  AND deletion_state = ''", r.bind(len(args)))
	if ownerUserID := strings.TrimSpace(input.OwnerUserID); ownerUserID != "" {
		args = append(args, ownerUserID)
		query += fmt.Sprintf(" AND owner_user_id = %s", r.bind(len(args)))
	}
	args = append(args, automationdomain.TaskSessionBindingStateReady)
	query += fmt.Sprintf(" AND session_binding_state = %s", r.bind(len(args)))
	if !input.AllowDisabled {
		args = append(args, true)
		query += fmt.Sprintf(" AND enabled = %s", r.bind(len(args)))
	}
	if strings.TrimSpace(input.OverlapPolicy) == "skip" {
		query += " AND (running_run_id IS NULL OR running_run_id = '')"
	}
	if input.ExpectedConfigurationVersion != nil {
		if *input.ExpectedConfigurationVersion < 1 {
			return "", nil, automationdomain.ErrConfigurationVersionConflict
		}
		args = append(args, *input.ExpectedConfigurationVersion)
		query += fmt.Sprintf(" AND configuration_version = %s", r.bind(len(args)))
	}
	if input.ExpectedPermissionRevision != nil {
		if *input.ExpectedPermissionRevision < 1 {
			return "", nil, automationdomain.ErrPermissionRequestStale
		}
		args = append(args, *input.ExpectedPermissionRevision)
		query += fmt.Sprintf(" AND permission_policy_revision = %s", r.bind(len(args)))
	}
	if input.ExpectedPermissionState != nil {
		args = append(args, strings.TrimSpace(*input.ExpectedPermissionState))
		query += fmt.Sprintf(" AND COALESCE(permission_state, '') = %s", r.bind(len(args)))
	}
	if input.ExpectedPermissionRequestID != nil {
		args = append(args, strings.TrimSpace(*input.ExpectedPermissionRequestID))
		query += fmt.Sprintf(" AND COALESCE(pending_permission_request_id, '') = %s", r.bind(len(args)))
	}
	return query, args, nil
}

func (r *Repository) runtimeClaimMiss(ctx context.Context, input JobRuntimeClaimInput) error {
	current, err := r.GetScheduledTask(ctx, strings.TrimSpace(input.OwnerUserID), strings.TrimSpace(input.JobID))
	if err != nil {
		return err
	}
	if current == nil {
		return automationdomain.ErrJobNotFound
	}
	if strings.TrimSpace(current.DeletionState) != "" {
		return automationdomain.ErrTaskDeleting
	}
	if input.ExpectedConfigurationVersion != nil &&
		current.ConfigurationVersion != *input.ExpectedConfigurationVersion {
		return automationdomain.ErrConfigurationVersionConflict
	}
	if input.ExpectedPermissionRevision != nil &&
		current.PermissionPolicy.Revision != *input.ExpectedPermissionRevision {
		return automationdomain.ErrPermissionRequestStale
	}
	if input.ExpectedPermissionState != nil &&
		strings.TrimSpace(current.PermissionState) != strings.TrimSpace(*input.ExpectedPermissionState) {
		return automationdomain.ErrPermissionRequestStale
	}
	if input.ExpectedPermissionRequestID != nil &&
		strings.TrimSpace(current.PendingPermissionRequestID) != strings.TrimSpace(*input.ExpectedPermissionRequestID) {
		return automationdomain.ErrPermissionRequestStale
	}
	return nil
}

// ClaimScheduledTaskRun 在一个事务内领取 task runtime 并写入对应首条 run。
// 同一 owner/request 的重试只重放 exact run；不同意图 fail closed。
func (r *Repository) ClaimScheduledTaskRun(
	ctx context.Context,
	input InitialRunClaimInput,
) (InitialRunClaimResult, error) {
	if err := validateInitialRunClaim(input); err != nil {
		return InitialRunClaimResult{}, err
	}
	attempts := 1
	if !r.isPostgres {
		attempts = automationWriteRetryAttempts
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		result, err := r.claimScheduledTaskRunOnce(ctx, input)
		if err == nil || !isSQLiteLockedError(err) || attempt == attempts-1 {
			return result, err
		}
		lastErr = err
		timer := time.NewTimer(automationWriteRetryDelay * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return InitialRunClaimResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	return InitialRunClaimResult{}, lastErr
}

func (r *Repository) terminalRunRequestQuery(
	expectation JobRuntimeClaimInput,
	run RunPendingInput,
) (string, []any, error) {
	valueArgs, err := r.initialRunValueArgs(run)
	if err != nil {
		return "", nil, err
	}
	args := append(valueArgs, strings.TrimSpace(run.JobID), strings.TrimSpace(run.OwnerUserID))
	query := fmt.Sprintf(
		`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, session_key, round_id,
    delivery_mode, delivery_to, delivery_target_json, delivery_status,
    scheduled_for, started_at, finished_at, attempts, error_message,
    permission_policy_revision, client_request_id, client_intent_digest,
    created_at, updated_at
) SELECT %s,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
FROM automation_scheduled_tasks
WHERE job_id = %s AND owner_user_id = %s AND deletion_state = ''
  AND session_binding_state = 'ready'
  AND overlap_policy = 'skip'
  AND COALESCE(running_run_id, '') <> ''`,
		r.bindList(len(valueArgs)), r.bind(len(valueArgs)+1), r.bind(len(valueArgs)+2),
	)
	if expectation.ExpectedConfigurationVersion != nil {
		args = append(args, *expectation.ExpectedConfigurationVersion)
		query += fmt.Sprintf(" AND configuration_version = %s", r.bind(len(args)))
	}
	if expectation.ExpectedPermissionRevision != nil {
		args = append(args, *expectation.ExpectedPermissionRevision)
		query += fmt.Sprintf(" AND permission_policy_revision = %s", r.bind(len(args)))
	}
	if expectation.ExpectedPermissionState != nil {
		args = append(args, strings.TrimSpace(*expectation.ExpectedPermissionState))
		query += fmt.Sprintf(" AND COALESCE(permission_state, '') = %s", r.bind(len(args)))
	}
	if expectation.ExpectedPermissionRequestID != nil {
		args = append(args, strings.TrimSpace(*expectation.ExpectedPermissionRequestID))
		query += fmt.Sprintf(" AND COALESCE(pending_permission_request_id, '') = %s", r.bind(len(args)))
	}
	return query, args, nil
}

func validateInitialRunClaim(input InitialRunClaimInput) error {
	ownerUserID := strings.TrimSpace(input.Runtime.OwnerUserID)
	jobID := strings.TrimSpace(input.Runtime.JobID)
	runID := strings.TrimSpace(input.Runtime.RunID)
	if ownerUserID == "" || jobID == "" || runID == "" ||
		ownerUserID != strings.TrimSpace(input.Run.OwnerUserID) ||
		jobID != strings.TrimSpace(input.Run.JobID) ||
		runID != strings.TrimSpace(input.Run.RunID) {
		return fmt.Errorf("automation initial run claim requires exact matching owner/job/run identity")
	}
	if input.Runtime.ExpectedConfigurationVersion == nil ||
		input.Runtime.ExpectedPermissionRevision == nil ||
		input.Runtime.ExpectedPermissionState == nil ||
		input.Runtime.ExpectedPermissionRequestID == nil {
		return fmt.Errorf("automation initial run claim requires configuration and permission snapshots")
	}
	requestID := strings.TrimSpace(input.Run.ClientRequestID)
	intentDigest := strings.TrimSpace(input.Run.IntentDigest)
	if (requestID == "") != (intentDigest == "") || len(requestID) > 128 || len(intentDigest) > 64 {
		return fmt.Errorf("automation initial run request identity is invalid")
	}
	if input.OverlapTerminalRun != nil {
		terminal := *input.OverlapTerminalRun
		if requestID == "" || !input.Runtime.AllowDisabled ||
			automationdomain.NormalizeOverlapPolicy(input.Runtime.OverlapPolicy) != automationdomain.OverlapPolicySkip ||
			strings.TrimSpace(input.Run.TriggerKind) != automationdomain.TriggerKindManual ||
			strings.TrimSpace(terminal.TriggerKind) != automationdomain.TriggerKindManual ||
			terminal.FinishedAt == nil ||
			strings.TrimSpace(terminal.Status) != automationdomain.RunStatusSkipped ||
			terminal.StartedAt != nil || terminal.Attempts != 0 ||
			strings.TrimSpace(terminal.DeliveryStatus) != automationdomain.DeliveryStatusNotAttempted ||
			strings.TrimSpace(terminal.OwnerUserID) != ownerUserID ||
			strings.TrimSpace(terminal.JobID) != jobID ||
			strings.TrimSpace(terminal.RunID) != runID ||
			strings.TrimSpace(terminal.ClientRequestID) != requestID ||
			strings.TrimSpace(terminal.IntentDigest) != intentDigest {
			return fmt.Errorf("automation overlap terminal run identity is invalid")
		}
	}
	return nil
}

func (r *Repository) claimScheduledTaskRunOnce(
	ctx context.Context,
	input InitialRunClaimInput,
) (InitialRunClaimResult, error) {
	claimQuery, claimArgs, err := r.runtimeClaimQuery(input.Runtime)
	if err != nil {
		return InitialRunClaimResult{}, err
	}
	runArgs, err := r.initialRunInsertArgs(input.Run)
	if err != nil {
		return InitialRunClaimResult{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return InitialRunClaimResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	claimResult, err := tx.ExecContext(ctx, claimQuery, claimArgs...)
	if err != nil {
		return InitialRunClaimResult{}, err
	}
	count, err := claimResult.RowsAffected()
	if err != nil {
		return InitialRunClaimResult{}, err
	}
	if count != 1 {
		if count == 0 && input.OverlapTerminalRun != nil {
			terminalQuery, terminalArgs, terminalQueryErr := r.terminalRunRequestQuery(
				input.Runtime, *input.OverlapTerminalRun,
			)
			if terminalQueryErr != nil {
				return InitialRunClaimResult{}, terminalQueryErr
			}
			terminalResult, terminalErr := tx.ExecContext(ctx, terminalQuery, terminalArgs...)
			if terminalErr != nil {
				_ = tx.Rollback()
				if replay, found, replayErr := r.replayInitialRunClaim(ctx, *input.OverlapTerminalRun); found || replayErr != nil {
					return replay, replayErr
				}
				return InitialRunClaimResult{}, terminalErr
			}
			inserted, rowsErr := terminalResult.RowsAffected()
			if rowsErr != nil {
				return InitialRunClaimResult{}, rowsErr
			}
			if inserted == 1 {
				if commitErr := tx.Commit(); commitErr != nil {
					return InitialRunClaimResult{}, commitErr
				}
				return InitialRunClaimResult{
					Claimed: true, Terminal: true, RunID: strings.TrimSpace(input.OverlapTerminalRun.RunID),
				}, nil
			}
		}
		_ = tx.Rollback()
		if replay, found, replayErr := r.replayInitialRunClaim(ctx, input.Run); found || replayErr != nil {
			return replay, replayErr
		}
		if count == 0 {
			if missErr := r.runtimeClaimMiss(ctx, input.Runtime); missErr != nil {
				return InitialRunClaimResult{}, missErr
			}
			if strings.TrimSpace(input.Run.ClientRequestID) == "" {
				return InitialRunClaimResult{}, nil
			}
			return InitialRunClaimResult{}, automationdomain.ErrRuntimeCommandUncertain
		}
		return InitialRunClaimResult{}, fmt.Errorf("automation runtime claim changed %d tasks", count)
	}
	insertResult, err := tx.ExecContext(ctx, r.insertRunPendingQuery, runArgs...)
	if err != nil {
		_ = tx.Rollback()
		if replay, found, replayErr := r.replayInitialRunClaim(ctx, input.Run); found || replayErr != nil {
			return replay, replayErr
		}
		return InitialRunClaimResult{}, err
	}
	inserted, err := insertResult.RowsAffected()
	if err != nil {
		return InitialRunClaimResult{}, err
	}
	if inserted != 1 {
		return InitialRunClaimResult{}, fmt.Errorf("automation initial run insert changed %d rows", inserted)
	}
	if err = tx.Commit(); err != nil {
		return InitialRunClaimResult{}, err
	}
	return InitialRunClaimResult{Claimed: true, RunID: strings.TrimSpace(input.Run.RunID)}, nil
}

func (r *Repository) replayInitialRunClaim(
	ctx context.Context,
	input RunPendingInput,
) (InitialRunClaimResult, bool, error) {
	run, found, err := r.GetRunByClientRequest(
		ctx,
		input.OwnerUserID,
		input.JobID,
		input.ClientRequestID,
		input.IntentDigest,
	)
	if err != nil || !found {
		return InitialRunClaimResult{}, found, err
	}
	return InitialRunClaimResult{Replayed: true, RunID: strings.TrimSpace(run.RunID)}, true, nil
}

// UpdateScheduledTaskRuntime 写入调度器运行态，不覆盖任务定义字段。
func (r *Repository) UpdateScheduledTaskRuntime(ctx context.Context, input JobRuntimeUpdateInput) error {
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET next_run_at = %s,
    running_run_id = %s,
    running_started_at = %s,
    last_run_at = %s,
    last_run_status = %s,
    failure_streak = %s,
    last_error = %s,
    last_delivery_status = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND deletion_state = ''`,
		r.bind(1),
		r.bind(2),
		r.bind(3),
		r.bind(4),
		r.bind(5),
		r.bind(6),
		r.bind(7),
		r.bind(8),
		r.bind(9),
	)
	_, err := r.execWithRetry(
		ctx,
		query,
		nullableTime(input.NextRunAt),
		nullString(input.RunningRunID),
		nullableTime(input.RunningStartedAt),
		nullableTime(input.LastRunAt),
		nullString(input.LastRunStatus),
		input.FailureStreak,
		nullableString(input.LastError),
		nullString(input.LastDeliveryStatus),
		strings.TrimSpace(input.JobID),
	)
	return err
}

// PauseScheduledTaskRuntimeForPermission 写入等待权限的运行态，但不会覆盖另一
// 个实例已经领取的新 run。返回 false 表示权威运行身份已经变化。
func (r *Repository) PauseScheduledTaskRuntimeForPermission(
	ctx context.Context,
	input JobRuntimePermissionPauseInput,
) (bool, error) {
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET running_run_id = %s,
    running_started_at = %s,
    last_run_status = %s,
    last_error = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
	  AND COALESCE(running_run_id, '') = %s
  AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		nullString(input.NextRunningRunID),
		nullableTime(input.NextRunningStartedAt),
		nullString(input.LastRunStatus),
		nullableString(input.LastError),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.ExpectedRunID),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// RecoverScheduledTaskRuntime 原子取消 exact active run 并释放同一任务占用。
// 任务已切换到其他 run 时整个事务回滚，不能把新运行误清除。
func (r *Repository) RecoverScheduledTaskRuntime(
	ctx context.Context,
	input JobRuntimeRecoveryInput,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err = r.cancelActiveRunForRecovery(ctx, tx, input); err != nil {
		return err
	}
	updated, err := r.releaseTaskRuntimeForRecovery(ctx, tx, input)
	if err != nil {
		return err
	}
	if !updated {
		return automationdomain.ErrRunRecoveryConflict
	}
	return tx.Commit()
}

func (r *Repository) cancelActiveRunForRecovery(
	ctx context.Context,
	tx *sql.Tx,
	input JobRuntimeRecoveryInput,
) error {
	query := fmt.Sprintf(
		`UPDATE automation_task_runs
SET status = %s,
    finished_at = %s,
    error_message = %s,
    delivery_status = %s,
    delivery_error = NULL,
    delivered_at = NULL,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = NULL,
    block_state = '',
    blocked_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND job_id = %s
  AND owner_user_id = %s
  AND status IN (%s, %s, %s)`,
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
	)
	result, err := tx.ExecContext(
		ctx,
		query,
		automationdomain.RunStatusCancelled,
		input.FinishedAt.UTC(),
		nullableString(input.ErrorMessage),
		automationdomain.DeliveryStatusNotAttempted,
		strings.TrimSpace(input.RunID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
		automationdomain.RunStatusPending,
		automationdomain.RunStatusRunning,
		automationdomain.RunStatusQueuedToMain,
	)
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

	var status string
	err = tx.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`SELECT status
FROM automation_task_runs
WHERE run_id = %s
  AND job_id = %s
  AND owner_user_id = %s`,
			r.bind(1),
			r.bind(2),
			r.bind(3),
		),
		strings.TrimSpace(input.RunID),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
	).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return automationdomain.ErrRunRecoveryConflict
		}
		return err
	}
	if strings.TrimSpace(status) != automationdomain.RunStatusCancelled {
		return automationdomain.ErrRunRecoveryConflict
	}
	return nil
}

func (r *Repository) releaseTaskRuntimeForRecovery(
	ctx context.Context,
	tx *sql.Tx,
	input JobRuntimeRecoveryInput,
) (bool, error) {
	runtime := input.Runtime
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET next_run_at = %s,
    running_run_id = NULL,
    running_started_at = NULL,
    last_run_at = %s,
    last_run_status = %s,
    failure_streak = %s,
    last_error = %s,
    last_delivery_status = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
  AND COALESCE(running_run_id, '') = %s
  AND deletion_state = ''`,
		r.bind(1),
		r.bind(2),
		r.bind(3),
		r.bind(4),
		r.bind(5),
		r.bind(6),
		r.bind(7),
		r.bind(8),
		r.bind(9),
	)
	result, err := tx.ExecContext(
		ctx,
		query,
		nullableTime(runtime.NextRunAt),
		nullableTime(runtime.LastRunAt),
		nullString(runtime.LastRunStatus),
		runtime.FailureStreak,
		nullableString(runtime.LastError),
		nullString(runtime.LastDeliveryStatus),
		strings.TrimSpace(input.JobID),
		strings.TrimSpace(input.OwnerUserID),
		strings.TrimSpace(input.RunID),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
