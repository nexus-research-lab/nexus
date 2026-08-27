// INPUT: owner-scoped automation capability requests, task policy CAS 与 run 阻塞/恢复状态。
// OUTPUT: 持久审批请求、原子决策结果、无结果终态投递收口和绑定 exact request 的同一 logical run 重试状态。
// POS: automation 权限事实的 SQL 仓储；不解释具体工具或 connector 语义。
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

// PermissionRequestCreateInput 描述一次运行时阻塞请求。
type PermissionRequestCreateInput struct {
	Request    automationdomain.AutomationPermissionRequest
	TaskState  string
	BlockState string
}

// PermissionRequestDecisionStoreInput 描述一次 owner 决策及其任务/run 原子投影。
type PermissionRequestDecisionStoreInput struct {
	OwnerUserID       string
	RequestID         string
	Decision          string
	ResolvedByUserID  string
	ResolvedAt        time.Time
	ExpectedRevision  int
	NextPolicy        *automationdomain.TaskPermissionPolicy
	TaskState         string
	RunBlockState     string
	FinishRunAsDenied bool
	DeniedMessage     string
}

// RunResumeInput 描述同一 logical run 的新执行 attempt。
type RunResumeInput struct {
	RunID                    string
	OwnerUserID              string
	RequestID                string
	RoundID                  string
	SessionKey               string
	StartedAt                time.Time
	PermissionPolicyRevision int
}

// TaskPermissionBoundaryUpdateInput 把任务定义写入与旧审批/run 失效放在同一事务。
type TaskPermissionBoundaryUpdateInput struct {
	Job                  automationdomain.ScheduledTask
	ExpectedVersion      *int64
	ExpectedRunningRunID *string
	CancellationMessage  string
}

// UpdateTaskAndInvalidatePermissionBoundary 原子提交任务新定义，并使旧 revision
// 的 pending request 与 blocked run 失效。返回值只用于事务成功后的通知。
func (r *Repository) UpdateTaskAndInvalidatePermissionBoundary(
	ctx context.Context,
	input TaskPermissionBoundaryUpdateInput,
) (*automationdomain.ScheduledTask, []automationdomain.AutomationPermissionRequest, error) {
	attempts := automationWriteRetryAttempts
	if r.isPostgres {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		task, requests, err := r.updateTaskAndInvalidatePermissionBoundaryOnce(ctx, input)
		if err == nil || !isSQLiteLockedError(err) || attempt == attempts-1 {
			return task, requests, err
		}
		timer := time.NewTimer(automationWriteRetryDelay * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, nil, errors.New("scheduled task permission boundary update retries exhausted")
}

func (r *Repository) updateTaskAndInvalidatePermissionBoundaryOnce(
	ctx context.Context,
	input TaskPermissionBoundaryUpdateInput,
) (*automationdomain.ScheduledTask, []automationdomain.AutomationPermissionRequest, error) {
	var (
		definitionQuery string
		definitionArgs  []any
		err             error
	)
	if input.ExpectedVersion != nil {
		definitionQuery, definitionArgs, err = r.scheduledTaskVersionUpdateStatement(
			input.Job,
			*input.ExpectedVersion,
			input.ExpectedRunningRunID,
		)
	} else {
		definitionQuery = r.upsertScheduledTaskQuery
		definitionArgs, err = scheduledTaskUpsertArgs(input.Job)
	}
	if err != nil {
		return nil, nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	pending, err := r.listPendingPermissionRequestsForTaskTx(
		ctx,
		tx,
		input.Job.OwnerUserID,
		input.Job.JobID,
	)
	if err != nil {
		return nil, nil, err
	}
	result, err := tx.ExecContext(ctx, definitionQuery, definitionArgs...)
	if err != nil {
		return nil, nil, err
	}
	count, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return nil, nil, rowsErr
	}
	if count != 1 {
		if input.ExpectedVersion != nil {
			return nil, nil, automationdomain.ErrConfigurationVersionConflict
		}
		return nil, nil, automationdomain.ErrTaskDeleting
	}

	if _, err = tx.ExecContext(
		ctx,
		fmt.Sprintf(
			`UPDATE automation_permission_requests
SET status = %s,
    decision = NULL,
    resolved_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND status = %s`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4),
		),
		automationdomain.PermissionRequestStatusSuperseded,
		strings.TrimSpace(input.Job.OwnerUserID),
		strings.TrimSpace(input.Job.JobID),
		automationdomain.PermissionRequestStatusPending,
	); err != nil {
		return nil, nil, err
	}
	invalidatedRunID, err := r.latestBlockedRunForPermissionInvalidationTx(
		ctx,
		tx,
		input.Job.OwnerUserID,
		input.Job.JobID,
		input.Job.PermissionPolicy.Revision,
	)
	if err != nil {
		return nil, nil, err
	}
	runResult, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(
			`UPDATE automation_task_runs
SET status = %s,
    finished_at = CURRENT_TIMESTAMP,
    error_message = %s,
    delivery_status = %s,
    delivery_error = NULL,
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
  AND block_state <> ''
  AND permission_policy_revision <> %s`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		),
		automationdomain.RunStatusCancelled,
		nullString(strings.TrimSpace(input.CancellationMessage)),
		automationdomain.DeliveryStatusNotAttempted,
		strings.TrimSpace(input.Job.OwnerUserID),
		strings.TrimSpace(input.Job.JobID),
		input.Job.PermissionPolicy.Revision,
	)
	if err != nil {
		return nil, nil, err
	}
	invalidatedCount, err := runResult.RowsAffected()
	if err != nil {
		return nil, nil, err
	}
	if invalidatedCount > 0 && invalidatedRunID != "" {
		if err = r.projectPermissionTerminalRunSummaryTx(
			ctx,
			tx,
			input.Job.OwnerUserID,
			input.Job.JobID,
			invalidatedRunID,
		); err != nil {
			return nil, nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	updated, err := r.GetScheduledTask(
		ctx,
		strings.TrimSpace(input.Job.OwnerUserID),
		strings.TrimSpace(input.Job.JobID),
	)
	if err != nil {
		return nil, nil, err
	}
	if updated == nil {
		return nil, nil, automationdomain.ErrJobNotFound
	}
	return updated, pending, nil
}

// latestBlockedRunForPermissionInvalidationTx selects the deterministic run
// that will become the newest completion when matching blocked runs are
// cancelled together. The final projection still compares all terminal runs.
func (r *Repository) latestBlockedRunForPermissionInvalidationTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	jobID string,
	nextPolicyRevision int,
) (string, error) {
	var runID string
	err := tx.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT run_id
FROM automation_task_runs
WHERE owner_user_id = %s
  AND job_id = %s
  AND block_state <> ''
  AND permission_policy_revision <> %s
ORDER BY run_id DESC
LIMIT 1`, r.bind(1), r.bind(2), r.bind(3)),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		nextPolicyRevision,
	).Scan(&runID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return strings.TrimSpace(runID), err
}

// projectPermissionTerminalRunSummaryTx updates the task summary only when the
// exact permission-terminated run is still the latest durable terminal fact.
func (r *Repository) projectPermissionTerminalRunSummaryTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	jobID string,
	runID string,
) error {
	var (
		status         string
		finishedAt     time.Time
		errorMessage   sql.NullString
		deliveryStatus string
	)
	err := tx.QueryRowContext(
		ctx,
		fmt.Sprintf(`SELECT status, finished_at, error_message, delivery_status
FROM automation_task_runs
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND status IN (%s, %s, %s)
  AND finished_at IS NOT NULL`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(runID),
		automationdomain.RunStatusSucceeded,
		automationdomain.RunStatusFailed,
		automationdomain.RunStatusCancelled,
	).Scan(&status, &finishedAt, &errorMessage, &deliveryStatus)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(
		ctx,
		fmt.Sprintf(`UPDATE automation_scheduled_tasks
SET last_run_at = %s,
    last_run_status = %s,
    failure_streak = failure_streak + 1,
    last_error = %s,
    last_delivery_status = %s,
    last_completed_run_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND deletion_state = ''
  AND NOT EXISTS (
      SELECT 1
      FROM automation_task_runs newer
      WHERE newer.owner_user_id = %s
        AND newer.job_id = %s
        AND newer.status IN (%s, %s, %s)
        AND newer.finished_at IS NOT NULL
        AND (
            newer.finished_at > %s
            OR (newer.finished_at = %s AND newer.run_id > %s)
        )
  )`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5),
			r.bind(6), r.bind(7), r.bind(8), r.bind(9), r.bind(10),
			r.bind(11), r.bind(12), r.bind(13), r.bind(14), r.bind(15),
		),
		finishedAt.UTC(),
		strings.TrimSpace(status),
		nullString(errorMessage.String),
		strings.TrimSpace(deliveryStatus),
		strings.TrimSpace(runID),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		automationdomain.RunStatusSucceeded,
		automationdomain.RunStatusFailed,
		automationdomain.RunStatusCancelled,
		finishedAt.UTC(),
		finishedAt.UTC(),
		strings.TrimSpace(runID),
	)
	return err
}

func (r *Repository) listPendingPermissionRequestsForTaskTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	jobID string,
) ([]automationdomain.AutomationPermissionRequest, error) {
	query := permissionRequestSelectSQL + fmt.Sprintf(
		" WHERE owner_user_id = %s AND job_id = %s AND status = %s ORDER BY created_at DESC, request_id DESC",
		r.bind(1), r.bind(2), r.bind(3),
	)
	rows, err := tx.QueryContext(
		ctx,
		query,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		automationdomain.PermissionRequestStatusPending,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]automationdomain.AutomationPermissionRequest, 0)
	for rows.Next() {
		item, scanErr := scanAutomationPermissionRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

// UpdateTaskPermissionPolicyIfRevision 只在修订仍匹配时写入任务授权快照。
func (r *Repository) UpdateTaskPermissionPolicyIfRevision(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	expectedRevision int,
	policy automationdomain.TaskPermissionPolicy,
	state string,
) (bool, error) {
	body, err := json.Marshal(policy)
	if err != nil {
		return false, err
	}
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET permission_policy_json = %s,
    permission_policy_revision = %s,
    permission_state = %s,
    pending_permission_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
  AND permission_policy_revision = %s
  AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		string(body),
		policy.Revision,
		strings.TrimSpace(state),
		strings.TrimSpace(jobID),
		strings.TrimSpace(ownerUserID),
		expectedRevision,
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// SetTaskPermissionState 更新任务交互状态，不改授权修订。
func (r *Repository) SetTaskPermissionState(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	state string,
	pendingRequestID string,
) error {
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET permission_state = %s,
    pending_permission_request_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s AND owner_user_id = %s AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		strings.TrimSpace(state),
		nullString(strings.TrimSpace(pendingRequestID)),
		strings.TrimSpace(jobID),
		strings.TrimSpace(ownerUserID),
	)
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

// ClearTaskPermissionRetryState 只清理调用方持有的 exact retry request。
// 新请求已替换 pending_request_id 或策略 revision 已推进时返回 false，绝不覆盖。
func (r *Repository) ClearTaskPermissionRetryState(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	policyRevision int,
	requestID string,
) (bool, error) {
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET permission_state = %s,
    pending_permission_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
  AND permission_policy_revision = %s
  AND pending_permission_request_id = %s
  AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.TaskPermissionStateReady,
		strings.TrimSpace(jobID),
		strings.TrimSpace(ownerUserID),
		policyRevision,
		strings.TrimSpace(requestID),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// RestoreTaskPermissionRetryState 在 task 尚未绑定请求或仍绑定同一请求时恢复。
// 非空的其他 request_id 是更新的权威事实，必须保留。
func (r *Repository) RestoreTaskPermissionRetryState(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	policyRevision int,
	requestID string,
) (bool, error) {
	query := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET permission_state = %s,
    pending_permission_request_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
  AND permission_policy_revision = %s
  AND deletion_state = ''
  AND (
    pending_permission_request_id IS NULL
    OR pending_permission_request_id = ''
    OR pending_permission_request_id = %s
  )
  AND EXISTS (
    SELECT 1
    FROM automation_task_runs AS run
    WHERE run.owner_user_id = %s
      AND run.job_id = %s
      AND run.status = %s
      AND run.block_state = %s
      AND run.blocked_request_id = %s
  )`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9), r.bind(10), r.bind(11),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.TaskPermissionStateReadyToRetry,
		nullString(strings.TrimSpace(requestID)),
		strings.TrimSpace(jobID),
		strings.TrimSpace(ownerUserID),
		policyRevision,
		strings.TrimSpace(requestID),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		automationdomain.RunStatusPending,
		automationdomain.RunBlockStateReadyToRetry,
		strings.TrimSpace(requestID),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// SupersedePendingPermissionRequests 使旧任务修订上的未决请求失效。
func (r *Repository) SupersedePendingPermissionRequests(ctx context.Context, ownerUserID string, jobID string) error {
	query := fmt.Sprintf(
		`UPDATE automation_permission_requests
SET status = %s,
    decision = NULL,
    resolved_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND status = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4),
	)
	_, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.PermissionRequestStatusSuperseded,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		automationdomain.PermissionRequestStatusPending,
	)
	return err
}

// CancelBlockedRunsForTaskRevision 结束已被新任务修订取代的阻塞 run。
func (r *Repository) CancelBlockedRunsForTaskRevision(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	currentRevision int,
	message string,
) error {
	query := fmt.Sprintf(
		`UPDATE automation_task_runs
SET status = %s,
    finished_at = CURRENT_TIMESTAMP,
    error_message = %s,
    block_state = '',
    blocked_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND block_state <> ''
  AND permission_policy_revision <> %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5),
	)
	_, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.RunStatusCancelled,
		nullString(strings.TrimSpace(message)),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		currentRevision,
	)
	return err
}

// CreatePermissionRequestAndBlockRun 原子写入请求、任务卡片状态和 run 阻塞状态。
func (r *Repository) CreatePermissionRequestAndBlockRun(
	ctx context.Context,
	input PermissionRequestCreateInput,
) (*automationdomain.AutomationPermissionRequest, bool, error) {
	request := input.Request
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.OwnerUserID = strings.TrimSpace(request.OwnerUserID)
	request.JobID = strings.TrimSpace(request.JobID)
	request.RunID = strings.TrimSpace(request.RunID)
	request.Kind = strings.TrimSpace(request.Kind)
	request.Capability.ToolName = strings.TrimSpace(request.Capability.ToolName)
	request.Capability.ConnectorID = strings.TrimSpace(request.Capability.ConnectorID)
	request.Capability.Effect = strings.TrimSpace(request.Capability.Effect)
	request.Capability.ResourceScope = strings.TrimSpace(request.Capability.ResourceScope)
	request.Capability.InputFingerprint = strings.TrimSpace(request.Capability.InputFingerprint)
	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)
	request.Reason = strings.TrimSpace(request.Reason)
	request.SessionKey = strings.TrimSpace(request.SessionKey)
	request.DeliverySessionKey = strings.TrimSpace(request.DeliverySessionKey)
	request.RoundID = strings.TrimSpace(request.RoundID)
	request.ToolUseID = strings.TrimSpace(request.ToolUseID)
	input.TaskState = strings.TrimSpace(input.TaskState)
	input.BlockState = strings.TrimSpace(input.BlockState)
	capabilityJSON, err := json.Marshal(request.Capability)
	if err != nil {
		return nil, false, err
	}
	inputSummaryJSON, err := json.Marshal(request.InputSummary)
	if err != nil {
		return nil, false, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	insertQuery := fmt.Sprintf(
		`INSERT INTO automation_permission_requests (
    request_id, owner_user_id, job_id, run_id, policy_revision, kind, status,
    tool_name, connector_id, effect, resource_scope, input_fingerprint,
    capability_json, input_summary_json, title, description, reason,
    session_key, delivery_session_key, round_id, tool_use_id, resume_safe, created_at, updated_at
) VALUES (%s)
ON CONFLICT DO NOTHING`,
		r.bindList(22)+",CURRENT_TIMESTAMP,CURRENT_TIMESTAMP",
	)
	result, err := tx.ExecContext(
		ctx,
		insertQuery,
		request.RequestID,
		request.OwnerUserID,
		request.JobID,
		nullString(request.RunID),
		request.PolicyRevision,
		request.Kind,
		automationdomain.PermissionRequestStatusPending,
		request.Capability.ToolName,
		nullString(request.Capability.ConnectorID),
		request.Capability.Effect,
		nullString(request.Capability.ResourceScope),
		request.Capability.InputFingerprint,
		string(capabilityJSON),
		string(inputSummaryJSON),
		nullString(request.Title),
		nullString(request.Description),
		nullString(request.Reason),
		nullString(request.SessionKey),
		nullString(request.DeliverySessionKey),
		nullString(request.RoundID),
		nullString(request.ToolUseID),
		request.ResumeSafe,
	)
	if err != nil {
		return nil, false, err
	}
	createdCount, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	created := createdCount > 0
	if !created {
		existing, loadErr := r.getPendingPermissionRequestByCapabilityTx(ctx, tx, request)
		if loadErr != nil {
			return nil, false, loadErr
		}
		request = *existing
	}

	taskUpdate := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET permission_state = %s,
    pending_permission_request_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
  AND permission_policy_revision = %s
  AND (pending_permission_request_id IS NULL OR pending_permission_request_id = %s)
  AND deletion_state = ''`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
	)
	taskResult, err := tx.ExecContext(
		ctx,
		taskUpdate,
		input.TaskState,
		request.RequestID,
		request.JobID,
		request.OwnerUserID,
		request.PolicyRevision,
		request.RequestID,
	)
	if err != nil {
		return nil, false, err
	}
	taskCount, err := taskResult.RowsAffected()
	if err != nil {
		return nil, false, err
	}
	if taskCount == 0 {
		return nil, false, automationdomain.ErrPermissionRequestStale
	}

	if request.RunID != "" {
		runUpdate := fmt.Sprintf(
			`UPDATE automation_task_runs
SET status = %s,
    block_state = %s,
    blocked_request_id = %s,
    error_message = %s,
    finished_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND permission_policy_revision = %s
  AND status IN (%s, %s, %s)`,
			r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
			r.bind(7), r.bind(8), r.bind(9), r.bind(10),
		)
		runResult, runErr := tx.ExecContext(
			ctx,
			runUpdate,
			automationdomain.RunStatusPending,
			input.BlockState,
			request.RequestID,
			nullString(request.Reason),
			request.RunID,
			request.OwnerUserID,
			request.PolicyRevision,
			automationdomain.RunStatusPending,
			automationdomain.RunStatusRunning,
			automationdomain.RunStatusQueuedToMain,
		)
		if runErr != nil {
			return nil, false, runErr
		}
		runCount, rowsErr := runResult.RowsAffected()
		if rowsErr != nil {
			return nil, false, rowsErr
		}
		if runCount == 0 {
			return nil, false, automationdomain.ErrPermissionRequestStale
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	loaded, err := r.GetPermissionRequest(ctx, request.OwnerUserID, request.RequestID)
	return loaded, created, err
}

func (r *Repository) getPendingPermissionRequestByCapabilityTx(
	ctx context.Context,
	tx *sql.Tx,
	request automationdomain.AutomationPermissionRequest,
) (*automationdomain.AutomationPermissionRequest, error) {
	query := permissionRequestSelectSQL + fmt.Sprintf(
		` WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND kind = %s
  AND input_fingerprint = %s
  AND status = %s
ORDER BY created_at DESC, request_id DESC
LIMIT 1`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
	)
	item, err := scanAutomationPermissionRequest(tx.QueryRowContext(
		ctx,
		query,
		request.OwnerUserID,
		request.JobID,
		request.RunID,
		request.Kind,
		request.Capability.InputFingerprint,
		automationdomain.PermissionRequestStatusPending,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, automationdomain.ErrPermissionRequestNotFound
	}
	return item, err
}

const permissionRequestSelectSQL = `
SELECT
    request_id, owner_user_id, job_id, run_id, policy_revision, kind, status,
    decision, capability_json, input_summary_json, title, description, reason,
    session_key, delivery_session_key, round_id, tool_use_id, resume_safe, resolved_by_user_id,
    created_at, updated_at, resolved_at
FROM automation_permission_requests`

// ListPermissionRequests 返回 owner 可见的审批请求。
func (r *Repository) ListPermissionRequests(
	ctx context.Context,
	ownerUserID string,
	status string,
	jobID string,
) ([]automationdomain.AutomationPermissionRequest, error) {
	query, args := r.permissionRequestListQuery(ownerUserID, status, jobID)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]automationdomain.AutomationPermissionRequest, 0)
	for rows.Next() {
		item, scanErr := scanAutomationPermissionRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) permissionRequestListQuery(
	ownerUserID string,
	status string,
	jobID string,
) (string, []any) {
	args := make([]any, 0, 4)
	conditions := make([]string, 0, 3)
	if value := strings.TrimSpace(ownerUserID); value != "" {
		args = append(args, value)
		conditions = append(conditions, "owner_user_id = "+r.bind(len(args)))
	}
	if value := strings.TrimSpace(status); value != "" {
		if value == "actionable" {
			args = append(
				args,
				automationdomain.PermissionRequestStatusPending,
				automationdomain.PermissionRequestStatusApproved,
			)
			conditions = append(conditions, fmt.Sprintf(
				"status IN (%s, %s)",
				r.bind(len(args)-1),
				r.bind(len(args)),
			))
			conditions = append(conditions, `EXISTS (
    SELECT 1
    FROM automation_scheduled_tasks AS task
    JOIN automation_task_runs AS run
      ON run.owner_user_id = automation_permission_requests.owner_user_id
     AND run.job_id = automation_permission_requests.job_id
     AND run.run_id = automation_permission_requests.run_id
    WHERE task.owner_user_id = automation_permission_requests.owner_user_id
      AND task.job_id = automation_permission_requests.job_id
      AND run.status = 'pending'
      AND (
        (
          automation_permission_requests.status = 'pending'
          AND task.pending_permission_request_id = automation_permission_requests.request_id
          AND run.blocked_request_id = automation_permission_requests.request_id
          AND run.block_state IN ('awaiting_approval', 'awaiting_reauth', 'awaiting_input')
        )
        OR (
          automation_permission_requests.status = 'approved'
          AND (
            run.blocked_request_id = automation_permission_requests.request_id
            OR (
              (run.blocked_request_id IS NULL OR run.blocked_request_id = '')
              AND task.pending_permission_request_id = automation_permission_requests.request_id
            )
          )
          AND run.block_state = 'ready_to_retry'
        )
      )
      AND (
        (
          automation_permission_requests.decision = 'allow_task'
          AND task.permission_policy_revision = automation_permission_requests.policy_revision + 1
          AND run.permission_policy_revision = task.permission_policy_revision
        )
        OR (
          (automation_permission_requests.decision IS NULL OR automation_permission_requests.decision <> 'allow_task')
          AND task.permission_policy_revision = automation_permission_requests.policy_revision
          AND run.permission_policy_revision = automation_permission_requests.policy_revision
        )
      )
)`)
		} else {
			args = append(args, value)
			conditions = append(conditions, "status = "+r.bind(len(args)))
		}
	}
	if value := strings.TrimSpace(jobID); value != "" {
		args = append(args, value)
		conditions = append(conditions, "job_id = "+r.bind(len(args)))
	}
	query := permissionRequestSelectSQL
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC, request_id DESC"
	return query, args
}

// BindRunReadyToRetryRequest 修复旧数据只在 task 上保存 retry request 的投影。
// 调用方必须先验证 task.pending_permission_request_id 与 request 完全一致。
func (r *Repository) BindRunReadyToRetryRequest(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	runID string,
	requestID string,
) (bool, error) {
	query := fmt.Sprintf(
		`UPDATE automation_task_runs
SET blocked_request_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND status = %s
  AND block_state = %s
  AND COALESCE(blocked_request_id, '') = ''
  AND EXISTS (
    SELECT 1
    FROM automation_scheduled_tasks AS task
    WHERE task.owner_user_id = %s
      AND task.job_id = %s
      AND task.pending_permission_request_id = %s
  )`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		strings.TrimSpace(requestID),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(runID),
		automationdomain.RunStatusPending,
		automationdomain.RunBlockStateReadyToRetry,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(requestID),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// GetPermissionRequest 读取 owner-scoped 审批请求。
func (r *Repository) GetPermissionRequest(ctx context.Context, ownerUserID string, requestID string) (*automationdomain.AutomationPermissionRequest, error) {
	query := permissionRequestSelectSQL + fmt.Sprintf(
		" WHERE request_id = %s AND owner_user_id = %s",
		r.bind(1), r.bind(2),
	)
	item, err := scanAutomationPermissionRequest(r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(requestID),
		strings.TrimSpace(ownerUserID),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, automationdomain.ErrPermissionRequestNotFound
	}
	return item, err
}

// HasApprovedRunPermission 判断同一 run 是否已有完全相同输入的一次授权。
func (r *Repository) HasApprovedRunPermission(
	ctx context.Context,
	ownerUserID string,
	jobID string,
	runID string,
	policyRevision int,
	inputFingerprint string,
) (bool, error) {
	query := fmt.Sprintf(
		`SELECT COUNT(1)
FROM automation_permission_requests
WHERE owner_user_id = %s
  AND job_id = %s
  AND run_id = %s
  AND policy_revision = %s
  AND input_fingerprint = %s
  AND status = %s
  AND decision = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7),
	)
	var count int
	err := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(jobID),
		strings.TrimSpace(runID),
		policyRevision,
		strings.TrimSpace(inputFingerprint),
		automationdomain.PermissionRequestStatusApproved,
		automationdomain.PermissionDecisionAllowOnce,
	).Scan(&count)
	return count > 0, err
}

// ResolvePermissionRequest 原子提交请求决策、任务策略和 run 后继状态。
func (r *Repository) ResolvePermissionRequest(
	ctx context.Context,
	input PermissionRequestDecisionStoreInput,
) (*automationdomain.AutomationPermissionRequest, error) {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	input.Decision = strings.TrimSpace(input.Decision)
	input.ResolvedByUserID = strings.TrimSpace(input.ResolvedByUserID)
	input.TaskState = strings.TrimSpace(input.TaskState)
	input.RunBlockState = strings.TrimSpace(input.RunBlockState)
	input.DeniedMessage = strings.TrimSpace(input.DeniedMessage)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	query := permissionRequestSelectSQL + fmt.Sprintf(
		" WHERE request_id = %s AND owner_user_id = %s",
		r.bind(1), r.bind(2),
	)
	request, err := scanAutomationPermissionRequest(tx.QueryRowContext(
		ctx,
		query,
		input.RequestID,
		input.OwnerUserID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, automationdomain.ErrPermissionRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	if request.Status != automationdomain.PermissionRequestStatusPending {
		return nil, automationdomain.ErrPermissionRequestResolved
	}
	if request.PolicyRevision != input.ExpectedRevision {
		return nil, automationdomain.ErrPermissionRequestStale
	}

	nextRevision := request.PolicyRevision
	policyJSON := ""
	if input.NextPolicy != nil {
		nextRevision = input.NextPolicy.Revision
		body, marshalErr := json.Marshal(input.NextPolicy)
		if marshalErr != nil {
			return nil, marshalErr
		}
		policyJSON = string(body)
	}
	pendingRequestID := ""
	if input.TaskState == automationdomain.TaskPermissionStateReadyToRetry {
		pendingRequestID = request.RequestID
	}
	taskArgs := []any{input.TaskState, nullString(pendingRequestID)}
	taskUpdate := fmt.Sprintf(
		`UPDATE automation_scheduled_tasks
SET permission_state = %s,
    pending_permission_request_id = %s`,
		r.bind(1), r.bind(2),
	)
	if input.NextPolicy != nil {
		taskArgs = append(taskArgs, policyJSON, nextRevision)
		taskUpdate += fmt.Sprintf(
			`, permission_policy_json = %s,
    permission_policy_revision = %s`,
			r.bind(3), r.bind(4),
		)
	}
	taskArgs = append(
		taskArgs,
		request.JobID,
		input.OwnerUserID,
		input.ExpectedRevision,
		request.RequestID,
	)
	taskUpdate += fmt.Sprintf(
		`,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
  AND permission_policy_revision = %s
  AND pending_permission_request_id = %s
  AND deletion_state = ''`,
		r.bind(len(taskArgs)-3), r.bind(len(taskArgs)-2), r.bind(len(taskArgs)-1), r.bind(len(taskArgs)),
	)
	taskResult, err := tx.ExecContext(ctx, taskUpdate, taskArgs...)
	if err != nil {
		return nil, err
	}
	taskCount, err := taskResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if taskCount == 0 {
		return nil, automationdomain.ErrPermissionRequestStale
	}

	requestStatus := automationdomain.PermissionRequestStatusApproved
	if input.Decision == automationdomain.PermissionDecisionDeny {
		requestStatus = automationdomain.PermissionRequestStatusDenied
	}
	requestUpdate := fmt.Sprintf(
		`UPDATE automation_permission_requests
SET status = %s,
    decision = %s,
    resolved_by_user_id = %s,
    resolved_at = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE request_id = %s
  AND owner_user_id = %s
  AND status = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7),
	)
	requestResult, err := tx.ExecContext(
		ctx,
		requestUpdate,
		requestStatus,
		input.Decision,
		input.ResolvedByUserID,
		input.ResolvedAt.UTC(),
		request.RequestID,
		input.OwnerUserID,
		automationdomain.PermissionRequestStatusPending,
	)
	if err != nil {
		return nil, err
	}
	requestCount, err := requestResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	if requestCount == 0 {
		return nil, automationdomain.ErrPermissionRequestResolved
	}

	if request.RunID != "" {
		if input.FinishRunAsDenied {
			runUpdate := fmt.Sprintf(
				`UPDATE automation_task_runs
SET status = %s,
    finished_at = %s,
    error_message = %s,
    delivery_status = %s,
    delivery_error = NULL,
    delivered_at = NULL,
    delivery_attempts = 0,
    delivery_attempt_id = NULL,
    delivery_attempt_started_at = NULL,
    delivery_next_attempt_at = NULL,
    delivery_dead_letter_at = NULL,
    block_state = '',
    blocked_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND blocked_request_id = %s
  AND block_state <> ''`,
				r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7),
			)
			runResult, runErr := tx.ExecContext(
				ctx,
				runUpdate,
				automationdomain.RunStatusFailed,
				input.ResolvedAt.UTC(),
				nullString(input.DeniedMessage),
				automationdomain.DeliveryStatusNotAttempted,
				request.RunID,
				input.OwnerUserID,
				request.RequestID,
			)
			if runErr != nil {
				return nil, runErr
			}
			if count, rowsErr := runResult.RowsAffected(); rowsErr != nil || count == 0 {
				if rowsErr != nil {
					return nil, rowsErr
				}
				return nil, automationdomain.ErrPermissionRequestStale
			}
			if runErr = r.projectPermissionTerminalRunSummaryTx(
				ctx,
				tx,
				input.OwnerUserID,
				request.JobID,
				request.RunID,
			); runErr != nil {
				return nil, runErr
			}
		} else {
			blockedRequestID := ""
			if input.RunBlockState == automationdomain.RunBlockStateReadyToRetry {
				blockedRequestID = request.RequestID
			}
			runUpdate := fmt.Sprintf(
				`UPDATE automation_task_runs
SET status = %s,
    permission_policy_revision = %s,
    block_state = %s,
    blocked_request_id = %s,
    error_message = NULL,
    finished_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND blocked_request_id = %s
  AND block_state <> ''`,
				r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6), r.bind(7),
			)
			runResult, runErr := tx.ExecContext(
				ctx,
				runUpdate,
				automationdomain.RunStatusPending,
				nextRevision,
				input.RunBlockState,
				nullString(blockedRequestID),
				request.RunID,
				input.OwnerUserID,
				request.RequestID,
			)
			if runErr != nil {
				return nil, runErr
			}
			if count, rowsErr := runResult.RowsAffected(); rowsErr != nil || count == 0 {
				if rowsErr != nil {
					return nil, rowsErr
				}
				return nil, automationdomain.ErrPermissionRequestStale
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPermissionRequest(ctx, input.OwnerUserID, input.RequestID)
}

// PrepareRunResume 把已授权的 logical run 切到新的 execution attempt。
func (r *Repository) PrepareRunResume(ctx context.Context, input RunResumeInput) (bool, error) {
	query := fmt.Sprintf(
		`UPDATE automation_task_runs
SET status = %s,
    session_key = %s,
    round_id = %s,
    started_at = %s,
    finished_at = NULL,
    attempts = attempts + 1,
    error_message = NULL,
    block_state = '',
    blocked_request_id = NULL,
    permission_policy_revision = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND status = %s
  AND block_state = %s
  AND blocked_request_id = %s
  AND EXISTS (
    SELECT 1 FROM automation_scheduled_tasks AS task
    WHERE task.job_id = automation_task_runs.job_id
      AND task.owner_user_id = automation_task_runs.owner_user_id
      AND task.deletion_state = ''
  )`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
		r.bind(7), r.bind(8), r.bind(9), r.bind(10),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.RunStatusRunning,
		nullString(strings.TrimSpace(input.SessionKey)),
		nullString(strings.TrimSpace(input.RoundID)),
		input.StartedAt.UTC(),
		input.PermissionPolicyRevision,
		strings.TrimSpace(input.RunID),
		strings.TrimSpace(input.OwnerUserID),
		automationdomain.RunStatusPending,
		automationdomain.RunBlockStateReadyToRetry,
		strings.TrimSpace(input.RequestID),
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// RestorePreparedRunReadyToRetry 补偿已 claim 但尚未成功派发的恢复 attempt。
// round_id 与 policy revision 共同证明仍是调用方刚准备的物理 attempt；若新请求
// 或终态已接管 run，则返回 stale，绝不把较新的状态改回旧 request。
func (r *Repository) RestorePreparedRunReadyToRetry(
	ctx context.Context,
	input RunResumeInput,
	errorMessage *string,
) error {
	query := fmt.Sprintf(
		`UPDATE automation_task_runs
SET status = %s,
    finished_at = NULL,
    error_message = %s,
    block_state = %s,
    blocked_request_id = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND status = %s
  AND permission_policy_revision = %s
  AND block_state = ''
  AND COALESCE(blocked_request_id, '') = ''
  AND COALESCE(round_id, '') = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5),
		r.bind(6), r.bind(7), r.bind(8), r.bind(9),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		automationdomain.RunStatusPending,
		nullableString(errorMessage),
		automationdomain.RunBlockStateReadyToRetry,
		nullString(strings.TrimSpace(input.RequestID)),
		strings.TrimSpace(input.RunID),
		strings.TrimSpace(input.OwnerUserID),
		automationdomain.RunStatusRunning,
		input.PermissionPolicyRevision,
		strings.TrimSpace(input.RoundID),
	)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return automationdomain.ErrPermissionRequestStale
	}
	return nil
}

// MarkRunEffectStarted 记录 run 已执行过非只读能力，后续不能静默重放。
func (r *Repository) MarkRunEffectStarted(ctx context.Context, ownerUserID string, runID string) error {
	query := fmt.Sprintf(
		`UPDATE automation_task_runs
SET effect_started = %s,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND block_state = ''
  AND status IN (%s, %s, %s)
  AND EXISTS (
    SELECT 1 FROM automation_scheduled_tasks AS task
    WHERE task.job_id = automation_task_runs.job_id
      AND task.owner_user_id = automation_task_runs.owner_user_id
      AND task.deletion_state = ''
  )`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
	)
	result, err := r.execWithRetry(
		ctx,
		query,
		true,
		strings.TrimSpace(runID),
		strings.TrimSpace(ownerUserID),
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
	if count == 0 {
		return automationdomain.ErrPermissionRequestStale
	}
	return nil
}

func scanAutomationPermissionRequest(scanner interface{ Scan(...any) error }) (*automationdomain.AutomationPermissionRequest, error) {
	var (
		item               automationdomain.AutomationPermissionRequest
		runID              sql.NullString
		decision           sql.NullString
		capabilityJSON     string
		inputSummaryJSON   string
		title              sql.NullString
		description        sql.NullString
		reason             sql.NullString
		sessionKey         sql.NullString
		deliverySessionKey sql.NullString
		roundID            sql.NullString
		toolUseID          sql.NullString
		resolvedByUserID   sql.NullString
		resolvedAt         sql.NullTime
	)
	if err := scanner.Scan(
		&item.RequestID,
		&item.OwnerUserID,
		&item.JobID,
		&runID,
		&item.PolicyRevision,
		&item.Kind,
		&item.Status,
		&decision,
		&capabilityJSON,
		&inputSummaryJSON,
		&title,
		&description,
		&reason,
		&sessionKey,
		&deliverySessionKey,
		&roundID,
		&toolUseID,
		&item.ResumeSafe,
		&resolvedByUserID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&resolvedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(capabilityJSON), &item.Capability); err != nil {
		return nil, err
	}
	if strings.TrimSpace(inputSummaryJSON) != "" {
		if err := json.Unmarshal([]byte(inputSummaryJSON), &item.InputSummary); err != nil {
			return nil, err
		}
	}
	item.RunID = nullStringValue(runID)
	item.Decision = nullStringValue(decision)
	item.Title = nullStringValue(title)
	item.Description = nullStringValue(description)
	item.Reason = nullStringValue(reason)
	item.SessionKey = nullStringValue(sessionKey)
	item.DeliverySessionKey = nullStringValue(deliverySessionKey)
	item.RoundID = nullStringValue(roundID)
	item.ToolUseID = nullStringValue(toolUseID)
	item.ResolvedByUserID = nullStringValue(resolvedByUserID)
	item.ResolvedAt = nullTimePointer(resolvedAt)
	return &item, nil
}
