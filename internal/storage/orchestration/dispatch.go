// INPUT: pending Room Dispatch、consumer lease、delivery receipt 与 retry schedule。
// OUTPUT: 可恢复的 outbox claim/deliver/retry CAS，以及永久失效 contract 的 terminal cancel。
// POS: Assignment 事务之后的可靠 Room 投递状态机；不依赖 realtime 服务。
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListAvailableRoomDispatches 返回当前可 claim 的 Room outbox 项。
//
// 返回结果只是候选快照；真正互斥由 ClaimDispatch 的单行 CAS 保证。
func (r *Repository) ListAvailableRoomDispatches(
	ctx context.Context,
	limit int,
) ([]protocol.ExecutionDispatch, error) {
	if limit <= 0 {
		limit = 32
	}
	if limit > 256 {
		limit = 256
	}
	now := r.currentTime()
	rows, err := r.db.QueryContext(ctx, r.dispatchSelect("dispatch.")+`
FROM execution_dispatches dispatch
WHERE dispatch.kind IN ('room_public', 'room_directed')
  AND (
      (dispatch.status IN ('pending', 'failed') AND dispatch.available_at <= `+r.bind(1)+`)
      OR (
          dispatch.status = 'claimed'
          AND dispatch.lease_expires_at IS NOT NULL
          AND dispatch.lease_expires_at <= `+r.bind(2)+`
      )
  )
ORDER BY dispatch.available_at, dispatch.created_at, dispatch.dispatch_id
LIMIT `+r.bind(3),
		r.timestamp(now), r.timestamp(now), limit,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanDispatch)
}

// ClaimDispatch 通过 lease + row version 原子认领一个候选 Dispatch。
func (r *Repository) ClaimDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	leaseDuration time.Duration,
) (*protocol.ExecutionDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if dispatchID == "" || leaseOwner == "" || expectedVersion <= 0 {
		return nil, fmt.Errorf("%w: dispatch id, expected version and lease owner are required", ErrInvariant)
	}
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	now := r.currentTime()
	leaseExpiresAt := now.Add(leaseDuration)
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'claimed',
    delivery_attempts = delivery_attempts + 1,
    version = version + 1,
    lease_owner = `+r.bind(1)+`,
    lease_expires_at = `+r.bind(2)+`,
    claimed_at = `+r.bind(3)+`,
    updated_at = `+r.bind(4)+`,
    last_error = NULL
WHERE dispatch_id = `+r.bind(5)+`
  AND version = `+r.bind(6)+`
  AND kind IN ('room_public', 'room_directed')
  AND (
      (status IN ('pending', 'failed') AND available_at <= `+r.bind(7)+`)
      OR (
          status = 'claimed'
          AND lease_expires_at IS NOT NULL
          AND lease_expires_at <= `+r.bind(8)+`
      )
  )`,
		leaseOwner, r.timestamp(leaseExpiresAt), r.timestamp(now), r.timestamp(now),
		dispatchID, expectedVersion, r.timestamp(now), r.timestamp(now),
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, ErrDispatchLease
	}
	item, err := r.getDispatch(ctx, r.db, dispatchID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrDispatchLease
	}
	return item, nil
}

// MarkDispatchDelivered 仅允许当前 lease owner ACK 自己 claim 的 row version。
//
// 同一 receipt 的重复 ACK 返回当前 delivered row，不产生第二次变化。
func (r *Repository) MarkDispatchDelivered(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	handoffID string,
	queueItemID string,
) (*protocol.ExecutionDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	handoffID = strings.TrimSpace(handoffID)
	queueItemID = strings.TrimSpace(queueItemID)
	if dispatchID == "" || leaseOwner == "" || expectedVersion <= 0 {
		return nil, fmt.Errorf("%w: dispatch id, expected version and lease owner are required", ErrInvariant)
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'delivered',
    version = version + 1,
    handoff_id = `+r.bind(1)+`,
    queue_item_id = `+r.bind(2)+`,
    lease_owner = NULL,
    lease_expires_at = NULL,
    delivered_at = `+r.bind(3)+`,
    updated_at = `+r.bind(4)+`,
    last_error = NULL
WHERE dispatch_id = `+r.bind(5)+`
  AND version = `+r.bind(6)+`
  AND status = 'claimed'
  AND lease_owner = `+r.bind(7),
		nullString(handoffID), nullString(queueItemID), r.timestamp(now), r.timestamp(now),
		dispatchID, expectedVersion, leaseOwner,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 1 {
		return r.getDispatch(ctx, r.db, dispatchID)
	}
	current, getErr := r.getDispatch(ctx, r.db, dispatchID)
	if getErr != nil {
		return nil, getErr
	}
	if current != nil && current.Status == protocol.ExecutionDispatchStatusDelivered &&
		current.HandoffID == handoffID && current.QueueItemID == queueItemID {
		return current, nil
	}
	return nil, ErrDispatchLease
}

// RetryDispatch 释放当前 lease，并把同一 outbox row 安排到未来重试。
func (r *Repository) RetryDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	retryAt time.Time,
	cause string,
) (*protocol.ExecutionDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	cause = strings.TrimSpace(cause)
	if dispatchID == "" || leaseOwner == "" || expectedVersion <= 0 || cause == "" {
		return nil, fmt.Errorf("%w: dispatch id, expected version, lease owner and cause are required", ErrInvariant)
	}
	now := r.currentTime()
	if retryAt.IsZero() || retryAt.Before(now) {
		retryAt = now
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'pending',
    version = version + 1,
    available_at = `+r.bind(1)+`,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = `+r.bind(2)+`,
    last_error = `+r.bind(3)+`
WHERE dispatch_id = `+r.bind(4)+`
  AND version = `+r.bind(5)+`
  AND status = 'claimed'
  AND lease_owner = `+r.bind(6),
		r.timestamp(retryAt), r.timestamp(now), cause,
		dispatchID, expectedVersion, leaseOwner,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		current, getErr := r.getDispatch(ctx, r.db, dispatchID)
		if getErr != nil {
			return nil, getErr
		}
		if current != nil && current.Status == protocol.ExecutionDispatchStatusDelivered {
			return current, nil
		}
		return nil, ErrDispatchLease
	}
	return r.getDispatch(ctx, r.db, dispatchID)
}

// CancelDispatch 终止一个已 claim、但无法完成权威 Room 投递的 Dispatch。
//
// 若它仍是尚未启动的 current responsibility，取消必须在同一事务释放
// Assignment、终结 pending Attempt、推进 Execution version 并记录审计事件；
// 否则仅终结 outbox row，避免误伤已经开始或已被其他控制路径收束的执行。
func (r *Repository) CancelDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	reason string,
) (*protocol.ExecutionDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	reason = strings.TrimSpace(reason)
	if dispatchID == "" || expectedVersion <= 0 || leaseOwner == "" || reason == "" {
		return nil, fmt.Errorf(
			"%w: dispatch cancellation identity, lease owner and reason are required",
			ErrInvariant,
		)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	current, err := r.getDispatch(ctx, tx, dispatchID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrDispatchLease
	}
	if current.Status == protocol.ExecutionDispatchStatusCancelled ||
		current.Status == protocol.ExecutionDispatchStatusDelivered {
		return current, nil
	}
	if current.Status != protocol.ExecutionDispatchStatusClaimed ||
		current.Version != expectedVersion ||
		current.LeaseOwner != leaseOwner {
		return nil, ErrDispatchLease
	}

	now := r.currentTime()
	assignment, err := r.getAssignment(ctx, tx, current.AssignmentID)
	if err != nil {
		return nil, err
	}
	releaseResponsibility := assignment != nil &&
		assignment.ExecutionID == current.ExecutionID &&
		assignment.PlanID == current.PlanID &&
		assignment.WorkItemID == current.WorkItemID &&
		assignment.SpecID == current.SpecID &&
		assignment.Status == protocol.WorkAssignmentStatusAssigned
	if releaseResponsibility {
		var runningAttempts int
		if err = tx.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_attempts
WHERE assignment_id = `+r.bind(1)+`
  AND status = 'running'`,
			current.AssignmentID,
		).Scan(&runningAttempts); err != nil {
			return nil, err
		}
		releaseResponsibility = runningAttempts == 0
	}
	if releaseResponsibility {
		executionResult, updateErr := tx.ExecContext(ctx, `
UPDATE executions
SET version = version + 1,
    updated_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND status IN ('active', 'waiting', 'paused')
  AND EXISTS (
      SELECT 1
      FROM execution_plan_revisions plan
      WHERE plan.execution_id = executions.execution_id
        AND plan.plan_id = `+r.bind(3)+`
        AND plan.status = 'active'
  )
  AND EXISTS (
      SELECT 1
      FROM execution_plan_items item
      JOIN execution_work_item_states state
        ON state.execution_id = item.execution_id
       AND state.work_item_id = item.work_item_id
      WHERE item.execution_id = executions.execution_id
        AND item.plan_id = `+r.bind(4)+`
        AND item.work_item_id = `+r.bind(5)+`
        AND item.spec_id = `+r.bind(6)+`
        AND state.current_spec_id = item.spec_id
        AND state.status = 'open'
  )`,
			r.timestamp(now),
			current.ExecutionID,
			current.PlanID,
			current.PlanID,
			current.WorkItemID,
			current.SpecID,
		)
		if updateErr != nil {
			return nil, updateErr
		}
		affected, affectedErr := executionResult.RowsAffected()
		if affectedErr != nil {
			return nil, affectedErr
		}
		releaseResponsibility = affected == 1
	}

	result, err := tx.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'cancelled',
    version = version + 1,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = `+r.bind(1)+`,
    last_error = `+r.bind(2)+`
WHERE dispatch_id = `+r.bind(3)+`
  AND version = `+r.bind(4)+`
  AND status = 'claimed'
  AND lease_owner = `+r.bind(5),
		r.timestamp(now), reason, dispatchID, expectedVersion, leaseOwner,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, ErrDispatchLease
	}
	if releaseResponsibility {
		terminalReason := "permanent dispatch failure: " + reason
		if _, err = tx.ExecContext(ctx, `
UPDATE execution_attempts
SET status = 'cancelled',
    failure_reason = `+r.bind(1)+`,
    version = version + 1,
    finished_at = `+r.bind(2)+`
WHERE assignment_id = `+r.bind(3)+`
  AND dispatch_id = `+r.bind(4)+`
  AND status = 'pending'`,
			terminalReason,
			r.timestamp(now),
			current.AssignmentID,
			current.ID,
		); err != nil {
			return nil, err
		}
		assignmentResult, updateErr := tx.ExecContext(ctx, `
UPDATE execution_work_assignments
SET status = 'released',
    version = version + 1,
    released_at = `+r.bind(1)+`
WHERE assignment_id = `+r.bind(2)+`
  AND version = `+r.bind(3)+`
  AND status = 'assigned'`,
			r.timestamp(now),
			assignment.ID,
			assignment.Version,
		)
		if updateErr != nil {
			return nil, updateErr
		}
		if err = requireOne(assignmentResult); err != nil {
			return nil, err
		}
		if err = r.insertEvent(ctx, tx, protocol.ExecutionEvent{
			ID:            "event-dispatch-cancel-" + current.ID,
			ExecutionID:   current.ExecutionID,
			Type:          protocol.ExecutionEventStatusChanged,
			EntityType:    protocol.ExecutionEntityAssignment,
			EntityID:      assignment.ID,
			EntityVersion: assignment.Version + 1,
			ActorKind:     protocol.ExecutionActorSystem,
			ActorID:       leaseOwner,
			CommandID:     "cancel-dispatch-" + current.ID,
			PlanID:        current.PlanID,
			WorkItemID:    current.WorkItemID,
			SpecID:        current.SpecID,
			AssignmentID:  assignment.ID,
			DispatchID:    current.ID,
			Payload: map[string]any{
				"reason":            reason,
				"dispatch_status":   protocol.ExecutionDispatchStatusCancelled,
				"assignment_status": protocol.WorkAssignmentStatusReleased,
				"attempt_status":    protocol.WorkAttemptStatusCancelled,
			},
			CreatedAt: now,
		}); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.getDispatch(ctx, r.db, dispatchID)
}

func (r *Repository) getDispatch(
	ctx context.Context,
	queryer interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	dispatchID string,
) (*protocol.ExecutionDispatch, error) {
	item, err := scanDispatch(queryer.QueryRowContext(
		ctx,
		r.dispatchSelect("")+`
FROM execution_dispatches
WHERE dispatch_id = `+r.bind(1),
		strings.TrimSpace(dispatchID),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}
