// INPUT: 已持久化的 Goal-bound Execution、exact Goal/revision/criteria fence 与 background retry deadline。
// OUTPUT: 与 Execution mutation 同事务建立的 pending receipt、幂等 confirmed receipt 和跨进程到期扫描。
// POS: Execution 正向绑定提交后到 Goal 反向绑定确认之间的通用 durable recovery 边界；不依赖 Plan proposal。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GoalConfirmationState is the durable state of one exact Execution -> Goal
// binding confirmation. A receipt never changes its identity fence.
type GoalConfirmationState string

const (
	GoalConfirmationPending   GoalConfirmationState = "pending"
	GoalConfirmationConfirmed GoalConfirmationState = "confirmed"
)

// GoalConfirmationReceipt is sufficient to retry Goal confirmation after the
// originating request, Plan proposal, or process has disappeared.
type GoalConfirmationReceipt struct {
	ExecutionID           string
	GoalID                string
	GoalObjectiveRevision int64
	CompletionCriteria    []string
	State                 GoalConfirmationState
	Version               int64
	AttemptCount          int
	NextAttemptAt         *time.Time
	LastError             string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ConfirmedAt           *time.Time
}

type ListRecoverableGoalConfirmationsQuery struct {
	Now   time.Time
	Limit int
}

type MarkGoalConfirmationCommand struct {
	ExecutionID           string
	GoalID                string
	GoalObjectiveRevision int64
	NextAttemptAt         *time.Time
	LastError             string
}

// EnsureGoalConfirmationReceipt reconstructs a missing legacy receipt from
// canonical Execution truth. New mutations call ensureGoalConfirmationReceiptTx
// inside their authoritative SQL transaction instead.
func (r *Repository) EnsureGoalConfirmationReceipt(
	ctx context.Context,
	executionID string,
) (*GoalConfirmationReceipt, error) {
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return nil, fmt.Errorf("%w: execution id is required", ErrInvariant)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	execution, err := r.getExecution(ctx, tx, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, sql.ErrNoRows
	}
	if err = r.ensureGoalConfirmationReceiptTx(ctx, tx, *execution); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetGoalConfirmationReceipt(ctx, executionID)
}

// GetGoalConfirmationReceipt returns the exact durable receipt for one
// Execution, or nil when the Execution has no Goal confirmation boundary.
func (r *Repository) GetGoalConfirmationReceipt(
	ctx context.Context,
	executionID string,
) (*GoalConfirmationReceipt, error) {
	item, err := scanGoalConfirmation(r.db.QueryRowContext(
		ctx,
		r.goalConfirmationSelect()+`WHERE execution_id = `+r.bind(1),
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

// ListRecoverableGoalConfirmations returns due pending receipts for the
// trusted background reconciler. It does not claim model-facing authority.
func (r *Repository) ListRecoverableGoalConfirmations(
	ctx context.Context,
	query ListRecoverableGoalConfirmationsQuery,
) ([]GoalConfirmationReceipt, error) {
	if query.Limit <= 0 {
		return nil, fmt.Errorf("%w: positive Goal confirmation recovery limit is required", ErrInvariant)
	}
	now := query.Now.UTC()
	if now.IsZero() {
		now = r.currentTime()
	}
	rows, err := r.db.QueryContext(ctx, r.goalConfirmationSelect()+`
WHERE state = 'pending'
  AND next_attempt_at <= `+r.bind(1)+`
ORDER BY next_attempt_at, updated_at, execution_id
LIMIT `+r.bind(2), r.timestamp(now), query.Limit)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanGoalConfirmation)
}

// MarkGoalConfirmationConfirmed closes one exact pending receipt. Repeated or
// concurrent confirmation is an idempotent read of the same terminal receipt.
func (r *Repository) MarkGoalConfirmationConfirmed(
	ctx context.Context,
	command MarkGoalConfirmationCommand,
) (*GoalConfirmationReceipt, error) {
	current, err := r.requireGoalConfirmationReceipt(ctx, command)
	if err != nil {
		return nil, err
	}
	if current.State == GoalConfirmationConfirmed {
		return current, nil
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_goal_confirmations
SET state = 'confirmed',
    version = version + 1,
    attempt_count = attempt_count + 1,
    next_attempt_at = NULL,
    last_error = NULL,
    updated_at = `+r.bind(1)+`,
    confirmed_at = `+r.bind(2)+`
WHERE execution_id = `+r.bind(3)+`
  AND goal_id = `+r.bind(4)+`
  AND goal_objective_revision = `+r.bind(5)+`
  AND state = 'pending'
  AND version = `+r.bind(6),
		r.timestamp(now), r.timestamp(now), current.ExecutionID, current.GoalID,
		current.GoalObjectiveRevision, current.Version,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	updated, err := r.GetGoalConfirmationReceipt(ctx, current.ExecutionID)
	if err != nil {
		return nil, err
	}
	if updated != nil && updated.State == GoalConfirmationConfirmed {
		return updated, nil
	}
	if affected != 1 {
		return nil, ErrVersionConflict
	}
	return nil, fmt.Errorf("%w: confirmed Goal receipt disappeared", ErrInvariant)
}

// ScheduleGoalConfirmationRetry records one failed confirmation attempt while
// preserving a concurrent confirmed state.
func (r *Repository) ScheduleGoalConfirmationRetry(
	ctx context.Context,
	command MarkGoalConfirmationCommand,
) (*GoalConfirmationReceipt, error) {
	command.LastError = strings.TrimSpace(command.LastError)
	if command.LastError == "" || command.NextAttemptAt == nil || command.NextAttemptAt.IsZero() {
		return nil, fmt.Errorf("%w: Goal confirmation retry requires error and deadline", ErrInvariant)
	}
	current, err := r.requireGoalConfirmationReceipt(ctx, command)
	if err != nil {
		return nil, err
	}
	if current.State == GoalConfirmationConfirmed {
		return current, nil
	}
	nextAttemptAt := command.NextAttemptAt.UTC()
	now := r.currentTime()
	if !nextAttemptAt.After(now) {
		return nil, fmt.Errorf("%w: Goal confirmation retry deadline must be in the future", ErrInvariant)
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_goal_confirmations
SET version = version + 1,
    attempt_count = attempt_count + 1,
    next_attempt_at = `+r.bind(1)+`,
    last_error = `+r.bind(2)+`,
    updated_at = `+r.bind(3)+`
WHERE execution_id = `+r.bind(4)+`
  AND goal_id = `+r.bind(5)+`
  AND goal_objective_revision = `+r.bind(6)+`
  AND state = 'pending'
  AND version = `+r.bind(7),
		r.timestamp(nextAttemptAt), command.LastError, r.timestamp(now),
		current.ExecutionID, current.GoalID, current.GoalObjectiveRevision, current.Version,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	updated, err := r.GetGoalConfirmationReceipt(ctx, current.ExecutionID)
	if err != nil {
		return nil, err
	}
	if updated != nil && (affected == 1 || updated.State == GoalConfirmationConfirmed) {
		return updated, nil
	}
	return nil, ErrVersionConflict
}

func (r *Repository) ensureGoalConfirmationReceiptTx(
	ctx context.Context,
	tx *sql.Tx,
	execution protocol.Execution,
) error {
	execution.ID = strings.TrimSpace(execution.ID)
	execution.GoalID = strings.TrimSpace(execution.GoalID)
	if execution.GoalID == "" {
		return nil
	}
	if execution.ID == "" || execution.GoalObjectiveRevision <= 0 {
		return fmt.Errorf("%w: complete Goal confirmation identity is required", ErrInvariant)
	}
	criteriaJSON, err := marshalSlice(execution.CompletionCriteria)
	if err != nil {
		return err
	}
	now := r.currentTime()
	createdAt := timeOr(execution.UpdatedAt, now)
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_goal_confirmations (
    execution_id, goal_id, goal_objective_revision,
    completion_criteria_json, state, version, attempt_count,
    next_attempt_at, last_error, created_at, updated_at, confirmed_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.jsonBind(4)+`,
    'pending', 1, 0, `+r.bind(5)+`, NULL, `+r.bind(6)+`,`+r.bind(7)+`, NULL)
ON CONFLICT (execution_id) DO NOTHING`,
		execution.ID, execution.GoalID, execution.GoalObjectiveRevision, criteriaJSON,
		r.timestamp(now), r.timestamp(createdAt), r.timestamp(now),
	)
	if err != nil {
		return err
	}
	stored, err := scanGoalConfirmation(tx.QueryRowContext(
		ctx,
		r.goalConfirmationSelect()+`WHERE execution_id = `+r.bind(1),
		execution.ID,
	))
	if err != nil {
		return err
	}
	if stored.GoalID != execution.GoalID ||
		stored.GoalObjectiveRevision != execution.GoalObjectiveRevision ||
		!slices.Equal(stored.CompletionCriteria, execution.CompletionCriteria) {
		return fmt.Errorf("%w: Execution Goal confirmation receipt fence changed", ErrInvariant)
	}
	return nil
}

func (r *Repository) requireGoalConfirmationReceipt(
	ctx context.Context,
	command MarkGoalConfirmationCommand,
) (*GoalConfirmationReceipt, error) {
	command.ExecutionID = strings.TrimSpace(command.ExecutionID)
	command.GoalID = strings.TrimSpace(command.GoalID)
	if command.ExecutionID == "" || command.GoalID == "" || command.GoalObjectiveRevision <= 0 {
		return nil, fmt.Errorf("%w: complete Goal confirmation receipt identity is required", ErrInvariant)
	}
	current, err := r.GetGoalConfirmationReceipt(ctx, command.ExecutionID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("%w: Goal confirmation receipt is missing", ErrInvariant)
	}
	if current.GoalID != command.GoalID ||
		current.GoalObjectiveRevision != command.GoalObjectiveRevision {
		return nil, fmt.Errorf("%w: Goal confirmation receipt identity changed", ErrInvariant)
	}
	return current, nil
}

func (r *Repository) goalConfirmationSelect() string {
	return `SELECT
    execution_id, goal_id, goal_objective_revision,
    ` + r.dialect.JSONText("completion_criteria_json") + `,
    state, version, attempt_count, next_attempt_at, last_error,
    created_at, updated_at, confirmed_at
FROM execution_goal_confirmations
`
}

func scanGoalConfirmation(
	scanner interface{ Scan(...any) error },
) (GoalConfirmationReceipt, error) {
	var item GoalConfirmationReceipt
	var criteriaJSON, state string
	var nextAttemptAt, confirmedAt sql.NullTime
	var lastError sql.NullString
	err := scanner.Scan(
		&item.ExecutionID, &item.GoalID, &item.GoalObjectiveRevision,
		&criteriaJSON, &state, &item.Version, &item.AttemptCount,
		&nextAttemptAt, &lastError, &item.CreatedAt, &item.UpdatedAt, &confirmedAt,
	)
	if err != nil {
		return GoalConfirmationReceipt{}, err
	}
	item.CompletionCriteria, err = parseSlice[string](criteriaJSON)
	if err != nil {
		return GoalConfirmationReceipt{}, err
	}
	item.State = GoalConfirmationState(state)
	item.NextAttemptAt = nullTimePointer(nextAttemptAt)
	item.LastError = nullStringValue(lastError)
	item.ConfirmedAt = nullTimePointer(confirmedAt)
	return item, nil
}
