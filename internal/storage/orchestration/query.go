// INPUT: Execution identity 与 active Plan SQL rows。
// OUTPUT: 根/子 Attempt 独立压缩的有界 ExecutionSnapshot 及各 aggregate 查询。
// POS: runtime/context 与 mutation 前置检查共用的一致读取投影。
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) executionSelect() string {
	return `SELECT
    execution_id, owner_user_id, session_key, scope_kind,
    room_id, conversation_id, coordinator_agent_id, origin, objective,
    ` + r.dialect.JSONText("completion_criteria_json") + `,
	    goal_id, goal_objective_revision, goal_activation_origin, goal_activation_reason,
	    recovery_of_execution_id, replaces_execution_id, root_round_id, trigger_message_id, status, version,
    created_at, updated_at, completed_at, ` + r.dialect.JSONText("metadata_json") + `
FROM executions`
}

func (r *Repository) planSelect(prefix string) string {
	return `SELECT
    ` + prefix + `plan_id, ` + prefix + `execution_id, ` + prefix + `revision,
    ` + prefix + `status, ` + prefix + `base_plan_id, ` + prefix + `created_by_agent_id,
    ` + prefix + `revision_reason, ` + prefix + `version, ` + prefix + `created_at,
    ` + prefix + `activated_at, ` + prefix + `superseded_at,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) workItemSelect(prefix string) string {
	return `SELECT
    ` + prefix + `work_item_id, ` + prefix + `execution_id, ` + prefix + `logical_key,
    ` + prefix + `kind, ` + prefix + `created_at, ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) stateSelect(prefix string) string {
	return `SELECT
    ` + prefix + `work_item_id, ` + prefix + `execution_id, ` + prefix + `current_spec_id,
    ` + prefix + `status, ` + prefix + `block_reason, ` + prefix + `needed_input,
    ` + prefix + `version, ` + prefix + `updated_at, ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) specSelect(prefix string) string {
	return `SELECT
    ` + prefix + `spec_id, ` + prefix + `work_item_id, ` + prefix + `execution_id,
    ` + prefix + `spec_version, ` + prefix + `subject, ` + prefix + `objective,
    ` + prefix + `deliverable, ` + r.dialect.JSONText(prefix+"acceptance_criteria_json") + `,
    ` + r.dialect.JSONText(prefix+"input_refs_json") + `, ` + prefix + `spec_hash,
    ` + prefix + `created_by_agent_id, ` + prefix + `created_at,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func planItemSelect(prefix string) string {
	return `SELECT
    ` + prefix + `plan_id, ` + prefix + `execution_id, ` + prefix + `work_item_id,
    ` + prefix + `spec_id, ` + prefix + `parent_work_item_id, ` + prefix + `is_required,
    ` + prefix + `is_terminal, ` + prefix + `position, ` + prefix + `created_at`
}

func dependencySelect(prefix string) string {
	return `SELECT
    ` + prefix + `plan_id, ` + prefix + `execution_id, ` + prefix + `work_item_id,
    ` + prefix + `depends_on_work_item_id, ` + prefix + `dependency_kind, ` + prefix + `created_at`
}

func claimSelect(prefix string) string {
	return `SELECT
    ` + prefix + `plan_id, ` + prefix + `execution_id, ` + prefix + `work_item_id,
    ` + prefix + `spec_id, ` + prefix + `claim_key, ` + prefix + `mode, ` + prefix + `created_at`
}

func (r *Repository) assignmentSelect(prefix string) string {
	return `SELECT
    ` + prefix + `assignment_id, ` + prefix + `execution_id, ` + prefix + `plan_id,
    ` + prefix + `work_item_id, ` + prefix + `spec_id, ` + prefix + `owner_agent_id,
    ` + prefix + `assigned_by_agent_id, ` + prefix + `return_to_agent_id,
    ` + prefix + `strategy, ` + prefix + `status, ` + prefix + `assignment_reason,
    ` + prefix + `takeover_reason, ` + prefix + `version, ` + prefix + `assigned_at,
    ` + prefix + `activated_at, ` + prefix + `released_at, ` + prefix + `completed_at,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) dispatchSelect(prefix string) string {
	return `SELECT
    ` + prefix + `dispatch_id, ` + prefix + `execution_id, ` + prefix + `plan_id,
    ` + prefix + `work_item_id, ` + prefix + `spec_id, ` + prefix + `assignment_id,
    ` + prefix + `command_id, ` + prefix + `dedupe_key, ` + prefix + `target_agent_id,
    ` + prefix + `kind, ` + prefix + `status, ` + prefix + `instruction,
    ` + prefix + `handoff_id, ` + prefix + `queue_item_id, ` + prefix + `delivery_attempts,
    ` + prefix + `version, ` + prefix + `available_at, ` + prefix + `lease_owner,
    ` + prefix + `lease_expires_at, ` + prefix + `created_at, ` + prefix + `updated_at,
    ` + prefix + `claimed_at, ` + prefix + `delivered_at, ` + prefix + `last_error,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) reviewDispatchSelect(prefix string) string {
	return `SELECT
    ` + prefix + `review_dispatch_id, ` + prefix + `execution_id, ` + prefix + `plan_id,
    ` + prefix + `work_item_id, ` + prefix + `spec_id, ` + prefix + `assignment_id,
    ` + prefix + `submission_id, ` + prefix + `command_id, ` + prefix + `dedupe_key,
    ` + prefix + `target_agent_id, ` + prefix + `status, ` + prefix + `instruction,
    ` + prefix + `handoff_id, ` + prefix + `queue_item_id, ` + prefix + `delivery_attempts,
    ` + prefix + `version, ` + prefix + `available_at, ` + prefix + `lease_owner,
    ` + prefix + `lease_expires_at, ` + prefix + `created_at, ` + prefix + `updated_at,
    ` + prefix + `claimed_at, ` + prefix + `delivered_at, ` + prefix + `last_error,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) cancellationDispatchSelect(prefix string) string {
	return `SELECT
    ` + prefix + `cancellation_dispatch_id, ` + prefix + `execution_id,
    ` + prefix + `plan_id, ` + prefix + `work_item_id, ` + prefix + `spec_id,
    ` + prefix + `assignment_id, ` + prefix + `attempt_id,
    ` + prefix + `runtime_attempt_id, ` + prefix + `dispatch_id,
    ` + prefix + `command_id, ` + prefix + `dedupe_key, ` + prefix + `scope_kind,
    ` + prefix + `scope_session_key, ` + prefix + `room_id,
    ` + prefix + `conversation_id, ` + prefix + `executor_kind,
    ` + prefix + `target_kind, ` + prefix + `target_agent_id,
    ` + prefix + `runtime_session_key, ` + prefix + `room_session_id,
    ` + prefix + `sdk_session_id, ` + prefix + `runtime_round_id,
    ` + prefix + `root_round_id, ` + prefix + `agent_round_id,
    ` + prefix + `child_session_id, ` + prefix + `sdk_task_id,
    ` + prefix + `tool_use_id, ` + prefix + `status, ` + prefix + `reason,
    ` + prefix + `limitation_code, ` + prefix + `outcome, ` + prefix + `receipt,
    ` + prefix + `delivery_attempts, ` + prefix + `version,
    ` + prefix + `available_at, ` + prefix + `lease_owner,
    ` + prefix + `lease_expires_at, ` + prefix + `created_at,
    ` + prefix + `updated_at, ` + prefix + `claimed_at,
    ` + prefix + `delivered_at, ` + prefix + `last_error,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) attemptSelect(prefix string) string {
	return `SELECT
    ` + prefix + `attempt_id, ` + prefix + `execution_id, ` + prefix + `plan_id,
    ` + prefix + `work_item_id, ` + prefix + `spec_id, ` + prefix + `assignment_id,
    ` + prefix + `dispatch_id, ` + prefix + `parent_attempt_id, ` + prefix + `executor_kind,
    ` + prefix + `executor_agent_id, ` + prefix + `parent_agent_id,
    ` + prefix + `runtime_session_key, ` + prefix + `room_session_id,
    ` + prefix + `sdk_session_id, ` + prefix + `runtime_round_id, ` + prefix + `root_round_id,
    ` + prefix + `agent_round_id, ` + prefix + `child_session_id, ` + prefix + `sdk_task_id,
	    ` + prefix + `tool_use_id, ` + prefix + `status, ` + prefix + `failure_reason,
	    ` + prefix + `version, ` + prefix + `created_at, ` + prefix + `started_at,
	    ` + prefix + `finished_at, ` + prefix + `parent_round_exited_at,
	    ` + prefix + `reconcile_after, ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) submissionSelect(prefix string) string {
	return `SELECT
    ` + prefix + `submission_id, ` + prefix + `execution_id, ` + prefix + `plan_id,
    ` + prefix + `work_item_id, ` + prefix + `spec_id, ` + prefix + `assignment_id,
    ` + prefix + `attempt_id, ` + prefix + `submission_sequence,
    ` + prefix + `submitter_agent_id, ` + prefix + `result_summary,
    ` + r.dialect.JSONText(prefix+"result_refs_json") + `,
    ` + r.dialect.JSONText(prefix+"evidence_json") + `, ` + prefix + `created_at,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func (r *Repository) acceptanceSelect(prefix string) string {
	return `SELECT
    ` + prefix + `acceptance_id, ` + prefix + `execution_id, ` + prefix + `plan_id,
    ` + prefix + `work_item_id, ` + prefix + `spec_id, ` + prefix + `assignment_id,
    ` + prefix + `submission_id, ` + prefix + `decision, ` + prefix + `reviewer_kind,
    ` + prefix + `reviewer_id, ` + r.dialect.JSONText(prefix+"criteria_results_json") + `,
    ` + prefix + `feedback, ` + prefix + `decision_round_id, ` + prefix + `created_at,
    ` + r.dialect.JSONText(prefix+"metadata_json")
}

func eventSelect(payloadExpression string) string {
	return `SELECT
    event_id, execution_id, sequence, command_id, event_type,
    entity_type, entity_id, entity_version, actor_kind, actor_id,
    goal_id, plan_id, work_item_id, spec_id, assignment_id,
    dispatch_id, attempt_id, submission_id, review_dispatch_id, acceptance_id,
    root_round_id, runtime_round_id, agent_round_id, ` + payloadExpression + `, created_at
FROM execution_events`
}

func (r *Repository) getExecution(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
) (*protocol.Execution, error) {
	item, err := scanExecution(queryer.QueryRowContext(
		ctx,
		r.executionSelect()+` WHERE execution_id = `+r.bind(1),
		strings.TrimSpace(executionID),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getActivePlan(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
) (*protocol.ExecutionPlanRevision, error) {
	item, err := scanPlan(queryer.QueryRowContext(
		ctx,
		r.planSelect("")+`
FROM execution_plan_revisions
WHERE execution_id = `+r.bind(1)+`
  AND status = 'active'`,
		executionID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getPlan(
	ctx context.Context,
	queryer sqlQueryer,
	planID string,
) (*protocol.ExecutionPlanRevision, error) {
	item, err := scanPlan(queryer.QueryRowContext(
		ctx,
		r.planSelect("")+`
FROM execution_plan_revisions
WHERE plan_id = `+r.bind(1),
		planID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getState(
	ctx context.Context,
	queryer sqlQueryer,
	workItemID string,
) (*protocol.WorkItemState, error) {
	item, err := scanWorkItemState(queryer.QueryRowContext(
		ctx,
		r.stateSelect("")+`
FROM execution_work_item_states
WHERE work_item_id = `+r.bind(1),
		workItemID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getAssignment(
	ctx context.Context,
	queryer sqlQueryer,
	assignmentID string,
) (*protocol.WorkAssignment, error) {
	item, err := scanAssignment(queryer.QueryRowContext(
		ctx,
		r.assignmentSelect("")+`
FROM execution_work_assignments
WHERE assignment_id = `+r.bind(1),
		assignmentID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getAttempt(
	ctx context.Context,
	queryer sqlQueryer,
	attemptID string,
) (*protocol.WorkAttempt, error) {
	item, err := scanAttempt(queryer.QueryRowContext(
		ctx,
		r.attemptSelect("")+`
FROM execution_attempts
WHERE attempt_id = `+r.bind(1),
		attemptID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) getSubmission(
	ctx context.Context,
	queryer sqlQueryer,
	submissionID string,
) (*protocol.WorkSubmission, error) {
	item, err := scanSubmission(queryer.QueryRowContext(
		ctx,
		r.submissionSelect("")+`
FROM execution_submissions
WHERE submission_id = `+r.bind(1),
		submissionID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetSnapshot 一致读取 active Plan 的有界当前投影。
func (r *Repository) GetSnapshot(ctx context.Context, executionID string) (*protocol.ExecutionSnapshot, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	snapshot, err := r.getSnapshot(ctx, tx, strings.TrimSpace(executionID))
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (r *Repository) getSnapshot(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	execution, err := r.getExecution(ctx, queryer, executionID)
	if err != nil || execution == nil {
		return nil, err
	}
	snapshot := &protocol.ExecutionSnapshot{Execution: *execution}
	if snapshot.CancellationDispatches, err = r.listSnapshotCancellationDispatches(
		ctx,
		queryer,
		executionID,
	); err != nil {
		return nil, err
	}
	plan, err := r.getActivePlan(ctx, queryer, executionID)
	if err != nil {
		return nil, err
	}
	snapshot.Plan = plan
	if plan == nil {
		deriveSnapshot(snapshot)
		return snapshot, nil
	}
	if snapshot.WorkItems, err = r.listSnapshotWorkItems(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.WorkItemStates, err = r.listSnapshotStates(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.WorkItemSpecs, err = r.listSnapshotSpecs(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.PlanItems, err = r.listPlanItems(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.Dependencies, err = r.listDependencies(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.OutputClaims, err = r.listClaims(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.Assignments, err = r.listSnapshotAssignments(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.Dispatches, err = r.listSnapshotDispatches(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.Attempts, err = r.listSnapshotAttempts(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.Submissions, err = r.listSnapshotSubmissions(ctx, queryer, executionID, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.ReviewDispatches, err = r.listSnapshotReviewDispatches(ctx, queryer, plan.ID); err != nil {
		return nil, err
	}
	if snapshot.Acceptances, err = r.listSnapshotAcceptances(ctx, queryer, executionID, plan.ID); err != nil {
		return nil, err
	}
	deriveSnapshot(snapshot)
	return snapshot, nil
}

func scanRows[T any](
	rows *sql.Rows,
	scan func(interface{ Scan(...any) error }) (T, error),
) ([]T, error) {
	defer rows.Close()
	items := make([]T, 0)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) listSnapshotWorkItems(ctx context.Context, q sqlQueryer, planID string) ([]protocol.WorkItem, error) {
	rows, err := q.QueryContext(ctx, r.workItemSelect("work.")+`
FROM execution_work_items work
JOIN execution_plan_items item ON item.work_item_id = work.work_item_id
WHERE item.plan_id = `+r.bind(1)+`
ORDER BY item.position, work.work_item_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanWorkItem)
}

func (r *Repository) listSnapshotStates(ctx context.Context, q sqlQueryer, planID string) ([]protocol.WorkItemState, error) {
	rows, err := q.QueryContext(ctx, r.stateSelect("state.")+`
FROM execution_work_item_states state
JOIN execution_plan_items item ON item.work_item_id = state.work_item_id
WHERE item.plan_id = `+r.bind(1)+`
ORDER BY item.position, state.work_item_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanWorkItemState)
}

func (r *Repository) listSnapshotSpecs(ctx context.Context, q sqlQueryer, planID string) ([]protocol.WorkItemSpec, error) {
	rows, err := q.QueryContext(ctx, r.specSelect("spec.")+`
FROM execution_work_item_specs spec
JOIN execution_plan_items item
  ON item.spec_id = spec.spec_id AND item.work_item_id = spec.work_item_id
WHERE item.plan_id = `+r.bind(1)+`
ORDER BY item.position, spec.spec_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanSpec)
}

func (r *Repository) listPlanItems(ctx context.Context, q sqlQueryer, planID string) ([]protocol.ExecutionPlanItem, error) {
	rows, err := q.QueryContext(ctx, planItemSelect("")+`
FROM execution_plan_items
WHERE plan_id = `+r.bind(1)+`
ORDER BY position, work_item_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanPlanItem)
}

func (r *Repository) listDependencies(ctx context.Context, q sqlQueryer, planID string) ([]protocol.ExecutionPlanDependency, error) {
	rows, err := q.QueryContext(ctx, dependencySelect("")+`
FROM execution_plan_dependencies
WHERE plan_id = `+r.bind(1)+`
ORDER BY work_item_id, depends_on_work_item_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanDependency)
}

func (r *Repository) listClaims(ctx context.Context, q sqlQueryer, planID string) ([]protocol.ExecutionPlanOutputClaim, error) {
	rows, err := q.QueryContext(ctx, claimSelect("")+`
FROM execution_plan_output_claims
WHERE plan_id = `+r.bind(1)+`
ORDER BY claim_key, work_item_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanOutputClaim)
}

func (r *Repository) listSnapshotAssignments(ctx context.Context, q sqlQueryer, planID string) ([]protocol.WorkAssignment, error) {
	rows, err := q.QueryContext(ctx, r.assignmentSelect("assignment.")+`
FROM execution_work_assignments assignment
WHERE assignment.plan_id = `+r.bind(1)+`
  AND (
      assignment.status IN ('assigned', 'active')
      OR NOT EXISTS (
          SELECT 1 FROM execution_work_assignments newer
          WHERE newer.plan_id = assignment.plan_id
            AND newer.work_item_id = assignment.work_item_id
            AND (
                newer.assigned_at > assignment.assigned_at
                OR (newer.assigned_at = assignment.assigned_at AND newer.assignment_id > assignment.assignment_id)
            )
      )
  )
ORDER BY assignment.work_item_id, assignment.assigned_at, assignment.assignment_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAssignment)
}

func (r *Repository) listSnapshotDispatches(ctx context.Context, q sqlQueryer, planID string) ([]protocol.ExecutionDispatch, error) {
	rows, err := q.QueryContext(ctx, r.dispatchSelect("dispatch.")+`
FROM execution_dispatches dispatch
WHERE dispatch.plan_id = `+r.bind(1)+`
  AND (
      dispatch.status IN ('pending', 'claimed')
      OR NOT EXISTS (
          SELECT 1 FROM execution_dispatches newer
          WHERE newer.assignment_id = dispatch.assignment_id
            AND (
                newer.created_at > dispatch.created_at
                OR (newer.created_at = dispatch.created_at AND newer.dispatch_id > dispatch.dispatch_id)
            )
      )
  )
ORDER BY dispatch.work_item_id, dispatch.created_at, dispatch.dispatch_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanDispatch)
}

func (r *Repository) listSnapshotAttempts(ctx context.Context, q sqlQueryer, planID string) ([]protocol.WorkAttempt, error) {
	rows, err := q.QueryContext(ctx, r.attemptSelect("attempt.")+`
FROM execution_attempts attempt
WHERE attempt.plan_id = `+r.bind(1)+`
  AND (
      attempt.status IN ('pending', 'running')
      OR NOT EXISTS (
          SELECT 1 FROM execution_attempts newer
          WHERE newer.assignment_id = attempt.assignment_id
            AND (
                (attempt.parent_attempt_id IS NULL AND newer.parent_attempt_id IS NULL)
                OR
                (attempt.parent_attempt_id IS NOT NULL AND newer.parent_attempt_id IS NOT NULL)
            )
            AND (
                newer.created_at > attempt.created_at
                OR (newer.created_at = attempt.created_at AND newer.attempt_id > attempt.attempt_id)
            )
      )
      OR EXISTS (
          SELECT 1 FROM execution_submissions submission
          WHERE submission.attempt_id = attempt.attempt_id
      )
  )
ORDER BY attempt.work_item_id, attempt.created_at, attempt.attempt_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAttempt)
}

func (r *Repository) listSnapshotSubmissions(
	ctx context.Context,
	q sqlQueryer,
	executionID string,
	planID string,
) ([]protocol.WorkSubmission, error) {
	rows, err := q.QueryContext(ctx, r.submissionSelect("submission.")+`
FROM execution_submissions submission
JOIN execution_plan_items active_item
  ON active_item.plan_id = `+r.bind(1)+`
 AND active_item.work_item_id = submission.work_item_id
 AND active_item.spec_id = submission.spec_id
WHERE submission.execution_id = `+r.bind(2)+`
  AND NOT EXISTS (
      SELECT 1 FROM execution_submissions newer
      WHERE newer.execution_id = submission.execution_id
        AND newer.work_item_id = submission.work_item_id
        AND newer.spec_id = submission.spec_id
        AND newer.submission_sequence > submission.submission_sequence
  )
ORDER BY submission.work_item_id, submission.submission_sequence`, planID, executionID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanSubmission)
}

func (r *Repository) listSnapshotReviewDispatches(
	ctx context.Context,
	q sqlQueryer,
	planID string,
) ([]protocol.ExecutionReviewDispatch, error) {
	rows, err := q.QueryContext(ctx, r.reviewDispatchSelect("review_dispatch.")+`
FROM execution_review_dispatches review_dispatch
WHERE review_dispatch.plan_id = `+r.bind(1)+`
ORDER BY review_dispatch.created_at, review_dispatch.review_dispatch_id`, planID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanReviewDispatch)
}

func (r *Repository) listSnapshotCancellationDispatches(
	ctx context.Context,
	q sqlQueryer,
	executionID string,
) ([]protocol.ExecutionCancellationDispatch, error) {
	rows, err := q.QueryContext(
		ctx,
		r.cancellationDispatchSelect("cancellation.")+`
FROM execution_cancellation_dispatches cancellation
WHERE cancellation.execution_id = `+r.bind(1)+`
ORDER BY cancellation.created_at, cancellation.cancellation_dispatch_id`,
		executionID,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanCancellationDispatch)
}

func (r *Repository) listSnapshotAcceptances(
	ctx context.Context,
	q sqlQueryer,
	executionID string,
	planID string,
) ([]protocol.WorkAcceptance, error) {
	rows, err := q.QueryContext(ctx, r.acceptanceSelect("acceptance.")+`
FROM execution_acceptances acceptance
JOIN execution_submissions submission ON submission.submission_id = acceptance.submission_id
JOIN execution_plan_items active_item
  ON active_item.plan_id = `+r.bind(1)+`
 AND active_item.work_item_id = acceptance.work_item_id
 AND active_item.spec_id = acceptance.spec_id
WHERE acceptance.execution_id = `+r.bind(2)+`
  AND NOT EXISTS (
      SELECT 1 FROM execution_submissions newer
      WHERE newer.execution_id = submission.execution_id
        AND newer.work_item_id = submission.work_item_id
        AND newer.spec_id = submission.spec_id
        AND newer.submission_sequence > submission.submission_sequence
  )
ORDER BY acceptance.work_item_id, acceptance.created_at`, planID, executionID)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAcceptance)
}
