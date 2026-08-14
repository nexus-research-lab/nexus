// INPUT: accepted Review transaction、durable completion retry deadline 与 receipt CAS。
// OUTPUT: 与 Acceptance 原子建立的 pending receipt、与 Complete 原子关闭的 terminal receipt，以及跨进程到期扫描。
// POS: Review acceptance 到 Execution completion 之间的 durable recovery boundary；模型不直接持有此能力。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CompletionAuditState is the durable lifecycle of one Execution completion
// check triggered by an accepted review.
type CompletionAuditState string

const (
	CompletionAuditPending   CompletionAuditState = "pending"
	CompletionAuditCompleted CompletionAuditState = "completed"
	CompletionAuditDiscarded CompletionAuditState = "discarded"
)

// CompletionAuditReceipt is sufficient to retry completion after the review
// request or process has disappeared. Completion still re-derives blockers
// from the latest authoritative snapshot.
type CompletionAuditReceipt struct {
	ExecutionID         string
	TriggerAcceptanceID string
	State               CompletionAuditState
	Version             int64
	AttemptCount        int
	NextAttemptAt       *time.Time
	LastError           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	SettledAt           *time.Time
}

type ListRecoverableCompletionAuditsQuery struct {
	Now   time.Time
	Limit int
}

// TransitionCompletionAuditCommand fences a background receipt transition.
// TriggerAcceptanceID and ExpectedVersion ensure an older worker cannot settle
// a receipt that a later accepted review has reawakened.
type TransitionCompletionAuditCommand struct {
	ExecutionID         string
	TriggerAcceptanceID string
	ExpectedVersion     int64
	NextAttemptAt       *time.Time
	LastError           string
}

// GetCompletionAuditReceipt returns the durable receipt for one Execution, or
// nil when no accepted review has requested a completion check.
func (r *Repository) GetCompletionAuditReceipt(
	ctx context.Context,
	executionID string,
) (*CompletionAuditReceipt, error) {
	item, err := scanCompletionAudit(r.db.QueryRowContext(
		ctx,
		r.completionAuditSelect()+`WHERE execution_id = `+r.bind(1),
		strings.TrimSpace(executionID),
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListRecoverableCompletionAudits returns due pending receipts. Listing does
// not grant completion authority; Complete performs the authoritative CAS and
// blocker check.
func (r *Repository) ListRecoverableCompletionAudits(
	ctx context.Context,
	query ListRecoverableCompletionAuditsQuery,
) ([]CompletionAuditReceipt, error) {
	if query.Limit <= 0 {
		return nil, fmt.Errorf("%w: positive completion audit recovery limit is required", ErrInvariant)
	}
	now := query.Now.UTC()
	if now.IsZero() {
		now = r.currentTime()
	}
	rows, err := r.db.QueryContext(ctx, r.completionAuditSelect()+`
WHERE state = 'pending'
  AND next_attempt_at <= `+r.bind(1)+`
ORDER BY next_attempt_at, updated_at, execution_id
LIMIT `+r.bind(2), r.timestamp(now), query.Limit)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanCompletionAudit)
}

// ScheduleCompletionAuditRetry records a deferred or failed check without
// overwriting a later review wake-up or a terminal receipt.
func (r *Repository) ScheduleCompletionAuditRetry(
	ctx context.Context,
	command TransitionCompletionAuditCommand,
) (*CompletionAuditReceipt, error) {
	command.LastError = strings.TrimSpace(command.LastError)
	if command.LastError == "" || command.NextAttemptAt == nil || command.NextAttemptAt.IsZero() {
		return nil, fmt.Errorf("%w: completion audit retry requires error and deadline", ErrInvariant)
	}
	current, err := r.requireCompletionAuditReceipt(ctx, command)
	if err != nil {
		return nil, err
	}
	if current.State != CompletionAuditPending {
		return current, nil
	}
	nextAttemptAt := command.NextAttemptAt.UTC()
	now := r.currentTime()
	if !nextAttemptAt.After(now) {
		return nil, fmt.Errorf("%w: completion audit retry deadline must be in the future", ErrInvariant)
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_completion_audits
SET version = version + 1,
    attempt_count = attempt_count + 1,
    next_attempt_at = `+r.bind(1)+`,
    last_error = `+r.bind(2)+`,
    updated_at = `+r.bind(3)+`
WHERE execution_id = `+r.bind(4)+`
  AND trigger_acceptance_id = `+r.bind(5)+`
  AND state = 'pending'
  AND version = `+r.bind(6),
		r.timestamp(nextAttemptAt), command.LastError, r.timestamp(now),
		current.ExecutionID, current.TriggerAcceptanceID, current.Version,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	updated, err := r.GetCompletionAuditReceipt(ctx, current.ExecutionID)
	if err != nil {
		return nil, err
	}
	if affected == 1 || updated != nil && updated.State != CompletionAuditPending {
		return updated, nil
	}
	return nil, ErrVersionConflict
}

// MarkCompletionAuditCompleted settles a pending legacy/concurrent receipt
// after authoritative Execution truth is already completed. Normal Complete
// calls settle the receipt inside their own transaction.
func (r *Repository) MarkCompletionAuditCompleted(
	ctx context.Context,
	command TransitionCompletionAuditCommand,
) (*CompletionAuditReceipt, error) {
	return r.settleCompletionAudit(ctx, command, CompletionAuditCompleted)
}

// MarkCompletionAuditDiscarded settles a receipt whose Execution reached a
// non-completed terminal state.
func (r *Repository) MarkCompletionAuditDiscarded(
	ctx context.Context,
	command TransitionCompletionAuditCommand,
) (*CompletionAuditReceipt, error) {
	command.LastError = strings.TrimSpace(command.LastError)
	if command.LastError == "" {
		return nil, fmt.Errorf("%w: discarded completion audit requires a reason", ErrInvariant)
	}
	return r.settleCompletionAudit(ctx, command, CompletionAuditDiscarded)
}

func (r *Repository) settleCompletionAudit(
	ctx context.Context,
	command TransitionCompletionAuditCommand,
	state CompletionAuditState,
) (*CompletionAuditReceipt, error) {
	current, err := r.requireCompletionAuditReceipt(ctx, command)
	if err != nil {
		return nil, err
	}
	if current.State != CompletionAuditPending {
		return current, nil
	}
	now := r.currentTime()
	var lastError any
	if state == CompletionAuditDiscarded {
		lastError = strings.TrimSpace(command.LastError)
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_completion_audits
SET state = `+r.bind(1)+`,
    version = version + 1,
    attempt_count = attempt_count + 1,
    next_attempt_at = NULL,
    last_error = `+r.bind(2)+`,
    updated_at = `+r.bind(3)+`,
    settled_at = `+r.bind(4)+`
WHERE execution_id = `+r.bind(5)+`
  AND trigger_acceptance_id = `+r.bind(6)+`
  AND state = 'pending'
  AND version = `+r.bind(7),
		state, lastError, r.timestamp(now), r.timestamp(now),
		current.ExecutionID, current.TriggerAcceptanceID, current.Version,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	updated, err := r.GetCompletionAuditReceipt(ctx, current.ExecutionID)
	if err != nil {
		return nil, err
	}
	if affected == 1 || updated != nil && updated.State != CompletionAuditPending {
		return updated, nil
	}
	return nil, ErrVersionConflict
}

func (r *Repository) ensureCompletionAuditTx(
	ctx context.Context,
	tx *sql.Tx,
	executionID string,
	acceptanceID string,
	createdAt time.Time,
) error {
	executionID = strings.TrimSpace(executionID)
	acceptanceID = strings.TrimSpace(acceptanceID)
	if executionID == "" || acceptanceID == "" {
		return fmt.Errorf("%w: completion audit identity is incomplete", ErrInvariant)
	}
	now := r.currentTime()
	createdAt = timeOr(createdAt, now)
	_, err := tx.ExecContext(ctx, `
INSERT INTO execution_completion_audits (
    execution_id, trigger_acceptance_id, state, version, attempt_count,
    next_attempt_at, last_error, created_at, updated_at, settled_at
) VALUES (`+r.bind(1)+`,`+r.bind(2)+`, 'pending', 1, 0, `+r.bind(3)+`, NULL, `+
		r.bind(4)+`,`+r.bind(5)+`, NULL)
ON CONFLICT (execution_id) DO UPDATE SET
    trigger_acceptance_id = excluded.trigger_acceptance_id,
    version = execution_completion_audits.version + 1,
    attempt_count = 0,
    next_attempt_at = excluded.next_attempt_at,
    last_error = NULL,
    updated_at = excluded.updated_at
WHERE execution_completion_audits.state = 'pending'`,
		executionID, acceptanceID, r.timestamp(now),
		r.timestamp(createdAt), r.timestamp(now),
	)
	if err != nil {
		return err
	}
	stored, err := scanCompletionAudit(tx.QueryRowContext(
		ctx,
		r.completionAuditSelect()+`WHERE execution_id = `+r.bind(1),
		executionID,
	))
	if err != nil {
		return err
	}
	if stored.State != CompletionAuditPending || stored.TriggerAcceptanceID != acceptanceID {
		return fmt.Errorf("%w: completion audit receipt is already terminal", ErrInvariant)
	}
	return nil
}

func (r *Repository) completeCompletionAuditTx(
	ctx context.Context,
	tx *sql.Tx,
	executionID string,
	completedAt time.Time,
) error {
	_, err := tx.ExecContext(ctx, `
UPDATE execution_completion_audits
SET state = 'completed',
    version = version + 1,
    attempt_count = attempt_count + 1,
    next_attempt_at = NULL,
    last_error = NULL,
    updated_at = `+r.bind(1)+`,
    settled_at = `+r.bind(2)+`
WHERE execution_id = `+r.bind(3)+`
  AND state = 'pending'`,
		r.timestamp(completedAt), r.timestamp(completedAt), strings.TrimSpace(executionID),
	)
	return err
}

func (r *Repository) requireCompletionAuditReceipt(
	ctx context.Context,
	command TransitionCompletionAuditCommand,
) (*CompletionAuditReceipt, error) {
	command.ExecutionID = strings.TrimSpace(command.ExecutionID)
	command.TriggerAcceptanceID = strings.TrimSpace(command.TriggerAcceptanceID)
	if command.ExecutionID == "" || command.TriggerAcceptanceID == "" || command.ExpectedVersion <= 0 {
		return nil, fmt.Errorf("%w: complete completion audit receipt identity is required", ErrInvariant)
	}
	current, err := r.GetCompletionAuditReceipt(ctx, command.ExecutionID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("%w: completion audit receipt is missing", ErrInvariant)
	}
	if current.TriggerAcceptanceID != command.TriggerAcceptanceID {
		return nil, ErrVersionConflict
	}
	if current.State == CompletionAuditPending && current.Version != command.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	return current, nil
}

func (r *Repository) completionAuditSelect() string {
	return `SELECT
    execution_id, trigger_acceptance_id, state, version, attempt_count,
    next_attempt_at, last_error, created_at, updated_at, settled_at
FROM execution_completion_audits
`
}

func scanCompletionAudit(
	scanner interface{ Scan(...any) error },
) (CompletionAuditReceipt, error) {
	var item CompletionAuditReceipt
	var state string
	var nextAttemptAt, settledAt sql.NullTime
	var lastError sql.NullString
	err := scanner.Scan(
		&item.ExecutionID, &item.TriggerAcceptanceID, &state,
		&item.Version, &item.AttemptCount, &nextAttemptAt, &lastError,
		&item.CreatedAt, &item.UpdatedAt, &settledAt,
	)
	if err != nil {
		return CompletionAuditReceipt{}, err
	}
	item.State = CompletionAuditState(state)
	item.NextAttemptAt = nullTimePointer(nextAttemptAt)
	item.LastError = nullStringValue(lastError)
	item.SettledAt = nullTimePointer(settledAt)
	return item, nil
}
