// INPUT: parent round exit 的 immutable child Attempt identity 与 grace deadline。
// OUTPUT: durable reconciliation schedule，以及跨进程可恢复的 expired child 查询。
// POS: runtime hook 的进程内关联与 Attempt 终态事务之间的持久化恢复边界。
package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ScheduleSubagentReconciliation 持久化 child Attempt 的 parent-exit deadline。
func (r *Repository) ScheduleSubagentReconciliation(
	ctx context.Context,
	command ScheduleSubagentReconciliationCommand,
) (*protocol.ExecutionSnapshot, error) {
	command.ExecutionID = strings.TrimSpace(command.ExecutionID)
	command.AttemptID = strings.TrimSpace(command.AttemptID)
	exitedAt := command.ParentRoundExitedAt.UTC()
	reconcileAfter := command.ReconcileAfter.UTC()
	if command.ExecutionID == "" || command.AttemptID == "" ||
		!protocol.ValidSubagentReconciliationDeadline(exitedAt, reconcileAfter) {
		return nil, fmt.Errorf(
			"%w: child Attempt identity and an exact 30 second reconciliation deadline are required",
			ErrInvariant,
		)
	}
	mutation, err := r.beginMutation(
		ctx,
		command.ExecutionID,
		command.ExpectedExecutionVersion,
		command.Meta,
		protocol.ExecutionEventAttemptReconcile,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	attempt, err := r.getAttempt(ctx, mutation.tx, command.AttemptID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if attempt == nil ||
		attempt.ExecutorKind != protocol.AttemptExecutorSubagent ||
		strings.TrimSpace(attempt.ParentAttemptID) == "" ||
		attempt.Status != protocol.WorkAttemptStatusRunning {
		r.abortMutation(mutation)
		return nil, fmt.Errorf(
			"%w: only a running child Attempt can schedule reconciliation",
			ErrInvariant,
		)
	}
	if err = validateExpectedVersion(
		command.ExpectedAttemptVersion,
		"expected Attempt version",
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE execution_attempts
SET parent_round_exited_at = `+r.bind(1)+`,
    reconcile_after = `+r.bind(2)+`,
    version = version + 1
WHERE attempt_id = `+r.bind(3)+`
  AND execution_id = `+r.bind(4)+`
  AND version = `+r.bind(5)+`
  AND executor_kind = 'subagent'
  AND parent_attempt_id IS NOT NULL
  AND status = 'running'`,
		r.timestamp(exitedAt),
		r.timestamp(reconcileAfter),
		attempt.ID,
		attempt.ExecutionID,
		command.ExpectedAttemptVersion,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityAttempt,
		EntityID:      attempt.ID,
		EntityVersion: attempt.Version + 1,
		PlanID:        attempt.PlanID,
		WorkItemID:    attempt.WorkItemID,
		SpecID:        attempt.SpecID,
		AssignmentID:  attempt.AssignmentID,
		AttemptID:     attempt.ID,
	})
}

// ListExpiredSubagentAttempts 返回已经越过 durable grace deadline 的 running child。
func (r *Repository) ListExpiredSubagentAttempts(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]protocol.WorkAttempt, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%w: positive reconciliation limit is required", ErrInvariant)
	}
	rows, err := r.db.QueryContext(ctx, r.attemptSelect("attempt.")+`
FROM execution_attempts attempt
JOIN executions execution ON execution.execution_id = attempt.execution_id
WHERE attempt.executor_kind = 'subagent'
  AND attempt.parent_attempt_id IS NOT NULL
  AND attempt.status = 'running'
  AND attempt.reconcile_after IS NOT NULL
  AND attempt.reconcile_after <= `+r.bind(1)+`
  AND execution.status IN ('active', 'waiting', 'paused')
ORDER BY attempt.reconcile_after, attempt.attempt_id
LIMIT `+r.bind(2),
		r.timestamp(now.UTC()),
		limit,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAttempt)
}

// ListOrphanedSubagentAttempts returns running children created by a previous
// server process for which parent-exit scheduling never committed. Callers
// must pass an immutable process-start cutoff and wait the normal grace period
// before terminalizing them; periodic scans must not advance that cutoff.
func (r *Repository) ListOrphanedSubagentAttempts(
	ctx context.Context,
	createdBefore time.Time,
	limit int,
) ([]protocol.WorkAttempt, error) {
	if createdBefore.IsZero() || limit <= 0 {
		return nil, fmt.Errorf(
			"%w: process-start cutoff and positive reconciliation limit are required",
			ErrInvariant,
		)
	}
	rows, err := r.db.QueryContext(ctx, r.attemptSelect("attempt.")+`
FROM execution_attempts attempt
JOIN executions execution ON execution.execution_id = attempt.execution_id
WHERE attempt.executor_kind = 'subagent'
  AND attempt.parent_attempt_id IS NOT NULL
  AND attempt.status = 'running'
  AND attempt.reconcile_after IS NULL
  AND attempt.created_at < `+r.bind(1)+`
  AND execution.status IN ('active', 'waiting', 'paused')
ORDER BY attempt.created_at, attempt.attempt_id
LIMIT `+r.bind(2),
		r.timestamp(createdBefore.UTC()),
		limit,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAttempt)
}
