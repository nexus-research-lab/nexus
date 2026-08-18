// INPUT: durable Execution outbox, subagent deadline and recovery receipt states.
// OUTPUT: one-query deadline snapshots used to arm process-local one-shot timers.
// POS: scheduling index over authoritative domain rows; it never claims or mutates work.
package orchestration

import (
	"context"
	"time"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

// DispatchDeadlines contains the earliest time each independent outbox class
// can next be claimed. Nil means that class has no recoverable row.
type DispatchDeadlines struct {
	Cancellation *time.Time
	Room         *time.Time
	Review       *time.Time
}

// RecoveryDeadlines contains the earliest time each durable orchestration saga
// can next advance. Immediate Plan proposals use updated_at as their deadline.
type RecoveryDeadlines struct {
	CompletionAudit  *time.Time
	GoalConfirmation *time.Time
	PlanProposal     *time.Time
}

// ExecutionDispatchDeadlines returns all three outbox deadlines in one query.
func (r *Repository) ExecutionDispatchDeadlines(
	ctx context.Context,
) (DispatchDeadlines, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  (
    SELECT MIN(CASE
      WHEN cancellation.status = 'claimed' THEN cancellation.lease_expires_at
      ELSE cancellation.available_at
    END)
    FROM execution_cancellation_dispatches cancellation
    WHERE cancellation.status = 'pending'
       OR (cancellation.status = 'claimed' AND cancellation.lease_expires_at IS NOT NULL)
  ),
  (
    SELECT MIN(CASE
      WHEN dispatch.status = 'claimed' THEN dispatch.lease_expires_at
      ELSE dispatch.available_at
    END)
    FROM execution_dispatches dispatch
    WHERE dispatch.kind IN ('room_public', 'room_directed')
      AND (
        dispatch.status IN ('pending', 'failed')
        OR (dispatch.status = 'claimed' AND dispatch.lease_expires_at IS NOT NULL)
      )
  ),
  (
    SELECT MIN(CASE
      WHEN review_dispatch.status = 'claimed' THEN review_dispatch.lease_expires_at
      ELSE review_dispatch.available_at
    END)
    FROM execution_review_dispatches review_dispatch
    JOIN executions execution
      ON execution.execution_id = review_dispatch.execution_id
    WHERE execution.status IN ('active', 'waiting')
      AND (
        review_dispatch.status IN ('pending', 'failed')
        OR (
          review_dispatch.status = 'claimed'
          AND review_dispatch.lease_expires_at IS NOT NULL
        )
      )
  )`)
	var cancellation, room, review any
	if err := row.Scan(&cancellation, &room, &review); err != nil {
		return DispatchDeadlines{}, err
	}
	cancellationAt, err := storage.NullableTime(cancellation)
	if err != nil {
		return DispatchDeadlines{}, err
	}
	roomAt, err := storage.NullableTime(room)
	if err != nil {
		return DispatchDeadlines{}, err
	}
	reviewAt, err := storage.NullableTime(review)
	if err != nil {
		return DispatchDeadlines{}, err
	}
	return DispatchDeadlines{
		Cancellation: cancellationAt,
		Room:         roomAt,
		Review:       reviewAt,
	}, nil
}

// NextSubagentReconciliationAt returns the earliest durable parent-exit grace
// deadline. Restart-only orphan handling deliberately remains process-scoped.
func (r *Repository) NextSubagentReconciliationAt(
	ctx context.Context,
) (*time.Time, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT MIN(attempt.reconcile_after)
FROM execution_attempts attempt
JOIN executions execution ON execution.execution_id = attempt.execution_id
WHERE attempt.executor_kind = 'subagent'
  AND attempt.parent_attempt_id IS NOT NULL
  AND attempt.status = 'running'
  AND attempt.reconcile_after IS NOT NULL
  AND execution.status IN ('active', 'waiting', 'paused')`)
	var deadline any
	if err := row.Scan(&deadline); err != nil {
		return nil, err
	}
	return storage.NullableTime(deadline)
}

// OrchestrationRecoveryDeadlines returns all saga deadlines in one query so an
// idle audit performs one read and only invokes a reconciler whose work is due.
func (r *Repository) OrchestrationRecoveryDeadlines(
	ctx context.Context,
) (RecoveryDeadlines, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT
  (
    SELECT MIN(next_attempt_at)
    FROM execution_completion_audits
    WHERE state = 'pending'
  ),
  (
    SELECT MIN(next_attempt_at)
    FROM execution_goal_confirmations
    WHERE state = 'pending'
  ),
  (
    SELECT MIN(COALESCE(proposal.next_attempt_at, proposal.updated_at))
    FROM execution_plan_proposals proposal
    WHERE proposal.status = 'materializing'
       OR (proposal.status = 'materialized' AND proposal.confirmation_state = 'pending')
       OR (
         proposal.status = 'blocked'
         AND EXISTS (
           SELECT 1
           FROM execution_events receipt
           WHERE receipt.execution_id = proposal.reserved_execution_id
             AND receipt.command_id = proposal.materialization_command_id || ':plan'
             AND receipt.event_type = 'plan_activated'
             AND receipt.entity_type = 'plan'
             AND receipt.plan_id IS NOT NULL
         )
       )
  )`)
	var completion, confirmation, proposal any
	if err := row.Scan(&completion, &confirmation, &proposal); err != nil {
		return RecoveryDeadlines{}, err
	}
	completionAt, err := storage.NullableTime(completion)
	if err != nil {
		return RecoveryDeadlines{}, err
	}
	confirmationAt, err := storage.NullableTime(confirmation)
	if err != nil {
		return RecoveryDeadlines{}, err
	}
	proposalAt, err := storage.NullableTime(proposal)
	if err != nil {
		return RecoveryDeadlines{}, err
	}
	return RecoveryDeadlines{
		CompletionAudit:  completionAt,
		GoalConfirmation: confirmationAt,
		PlanProposal:     proposalAt,
	}, nil
}
