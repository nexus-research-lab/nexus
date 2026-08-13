// INPUT: owner-scoped automation capability requests, task policy CAS 与 run 阻塞/恢复状态。
// OUTPUT: 持久审批请求、原子决策结果和同一 logical run 的重试状态。
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
	RoundID                  string
	SessionKey               string
	StartedAt                time.Time
	PermissionPolicyRevision int
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
  AND permission_policy_revision = %s`,
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
WHERE job_id = %s AND owner_user_id = %s`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4),
	)
	_, err := r.execWithRetry(
		ctx,
		query,
		strings.TrimSpace(state),
		nullString(strings.TrimSpace(pendingRequestID)),
		strings.TrimSpace(jobID),
		strings.TrimSpace(ownerUserID),
	)
	return err
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
		strings.TrimSpace(request.RequestID),
		strings.TrimSpace(request.OwnerUserID),
		strings.TrimSpace(request.JobID),
		nullString(strings.TrimSpace(request.RunID)),
		request.PolicyRevision,
		strings.TrimSpace(request.Kind),
		automationdomain.PermissionRequestStatusPending,
		strings.TrimSpace(request.Capability.ToolName),
		nullString(strings.TrimSpace(request.Capability.ConnectorID)),
		strings.TrimSpace(request.Capability.Effect),
		nullString(strings.TrimSpace(request.Capability.ResourceScope)),
		strings.TrimSpace(request.Capability.InputFingerprint),
		string(capabilityJSON),
		string(inputSummaryJSON),
		nullString(strings.TrimSpace(request.Title)),
		nullString(strings.TrimSpace(request.Description)),
		nullString(strings.TrimSpace(request.Reason)),
		nullString(strings.TrimSpace(request.SessionKey)),
		nullString(strings.TrimSpace(request.DeliverySessionKey)),
		nullString(strings.TrimSpace(request.RoundID)),
		nullString(strings.TrimSpace(request.ToolUseID)),
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
  AND (pending_permission_request_id IS NULL OR pending_permission_request_id = %s)`,
		r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
	)
	taskResult, err := tx.ExecContext(
		ctx,
		taskUpdate,
		strings.TrimSpace(input.TaskState),
		strings.TrimSpace(request.RequestID),
		strings.TrimSpace(request.JobID),
		strings.TrimSpace(request.OwnerUserID),
		request.PolicyRevision,
		strings.TrimSpace(request.RequestID),
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

	if strings.TrimSpace(request.RunID) != "" {
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
			strings.TrimSpace(input.BlockState),
			strings.TrimSpace(request.RequestID),
			nullString(strings.TrimSpace(request.Reason)),
			strings.TrimSpace(request.RunID),
			strings.TrimSpace(request.OwnerUserID),
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
		strings.TrimSpace(request.OwnerUserID),
		strings.TrimSpace(request.JobID),
		strings.TrimSpace(request.RunID),
		strings.TrimSpace(request.Kind),
		strings.TrimSpace(request.Capability.InputFingerprint),
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
	args := make([]any, 0, 3)
	conditions := make([]string, 0, 3)
	if value := strings.TrimSpace(ownerUserID); value != "" {
		args = append(args, value)
		conditions = append(conditions, "owner_user_id = "+r.bind(len(args)))
	}
	if value := strings.TrimSpace(status); value != "" {
		if value == "actionable" {
			conditions = append(conditions, `EXISTS (
    SELECT 1
    FROM automation_scheduled_tasks AS task
    JOIN automation_task_runs AS run
      ON run.owner_user_id = automation_permission_requests.owner_user_id
     AND run.job_id = automation_permission_requests.job_id
     AND run.run_id = automation_permission_requests.run_id
    WHERE task.owner_user_id = automation_permission_requests.owner_user_id
      AND task.job_id = automation_permission_requests.job_id
      AND task.pending_permission_request_id = automation_permission_requests.request_id
      AND run.status = 'pending'
      AND (
        (
          automation_permission_requests.status = 'pending'
          AND run.blocked_request_id = automation_permission_requests.request_id
          AND run.block_state IN ('awaiting_approval', 'awaiting_reauth', 'awaiting_input')
        )
        OR (
          automation_permission_requests.status = 'approved'
          AND run.blocked_request_id IS NULL
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
		strings.TrimSpace(input.RequestID),
		strings.TrimSpace(input.OwnerUserID),
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
	taskArgs := []any{strings.TrimSpace(input.TaskState), nullString(pendingRequestID)}
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
		strings.TrimSpace(request.JobID),
		strings.TrimSpace(input.OwnerUserID),
		input.ExpectedRevision,
		strings.TrimSpace(request.RequestID),
	)
	taskUpdate += fmt.Sprintf(
		`,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = %s
  AND owner_user_id = %s
  AND permission_policy_revision = %s
  AND pending_permission_request_id = %s`,
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
		strings.TrimSpace(input.Decision),
		strings.TrimSpace(input.ResolvedByUserID),
		input.ResolvedAt.UTC(),
		strings.TrimSpace(request.RequestID),
		strings.TrimSpace(input.OwnerUserID),
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

	if strings.TrimSpace(request.RunID) != "" {
		if input.FinishRunAsDenied {
			runUpdate := fmt.Sprintf(
				`UPDATE automation_task_runs
SET status = %s,
    finished_at = %s,
    error_message = %s,
    block_state = '',
    blocked_request_id = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND blocked_request_id = %s
  AND block_state <> ''`,
				r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
			)
			runResult, runErr := tx.ExecContext(
				ctx,
				runUpdate,
				automationdomain.RunStatusFailed,
				input.ResolvedAt.UTC(),
				nullString(strings.TrimSpace(input.DeniedMessage)),
				strings.TrimSpace(request.RunID),
				strings.TrimSpace(input.OwnerUserID),
				strings.TrimSpace(request.RequestID),
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
		} else {
			runUpdate := fmt.Sprintf(
				`UPDATE automation_task_runs
SET status = %s,
    permission_policy_revision = %s,
    block_state = %s,
    blocked_request_id = NULL,
    error_message = NULL,
    finished_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE run_id = %s
  AND owner_user_id = %s
  AND blocked_request_id = %s
  AND block_state <> ''`,
				r.bind(1), r.bind(2), r.bind(3), r.bind(4), r.bind(5), r.bind(6),
			)
			runResult, runErr := tx.ExecContext(
				ctx,
				runUpdate,
				automationdomain.RunStatusPending,
				nextRevision,
				strings.TrimSpace(input.RunBlockState),
				strings.TrimSpace(request.RunID),
				strings.TrimSpace(input.OwnerUserID),
				strings.TrimSpace(request.RequestID),
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
  AND block_state IN (%s, %s)`,
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
		automationdomain.RunBlockStateNone,
		automationdomain.RunBlockStateReadyToRetry,
	)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
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
  AND status IN (%s, %s, %s)`,
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
