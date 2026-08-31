// INPUT: 跨 Agent Room Submission 的 review-return outbox、consumer lease、delivery receipt 与 retry schedule。
// OUTPUT: 跨 Agent review 与 Submission 同事务创建、可恢复 claim/deliver/retry/cancel；自审不制造回投。
// POS: review 回交不复用 worker Assignment Dispatch，也不伪造 reviewer Attempt。
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

func (r *Repository) normalizeReviewDispatch(
	source *protocol.ExecutionReviewDispatch,
	submission protocol.WorkSubmission,
	assignment protocol.WorkAssignment,
	meta CommandMeta,
	now time.Time,
) (*protocol.ExecutionReviewDispatch, error) {
	crossAgentReview := strings.TrimSpace(assignment.ReturnToAgentID) !=
		strings.TrimSpace(assignment.OwnerAgentID)
	if source == nil {
		// self 同时用于 DM 与 Room，只有服务层提供的回投才能证明这是 Room 跨 Agent 审核。
		if assignment.Strategy != protocol.AssignmentStrategyRoomMember || !crossAgentReview {
			return nil, nil
		}
		return nil, fmt.Errorf(
			"%w: cross-Agent Room Submission requires a review-return Dispatch",
			ErrInvariant,
		)
	}
	if !crossAgentReview {
		return nil, fmt.Errorf(
			"%w: self-review Assignment must not create a review-return Dispatch",
			ErrInvariant,
		)
	}
	item := *source
	item.ID = strings.TrimSpace(item.ID)
	item.ExecutionID = submission.ExecutionID
	item.PlanID = submission.PlanID
	item.WorkItemID = submission.WorkItemID
	item.SpecID = submission.SpecID
	item.AssignmentID = submission.AssignmentID
	item.SubmissionID = submission.ID
	item.CommandID = strings.TrimSpace(meta.CommandID)
	requestedTarget := strings.TrimSpace(item.TargetAgentID)
	item.TargetAgentID = strings.TrimSpace(assignment.ReturnToAgentID)
	if requestedTarget != "" && requestedTarget != item.TargetAgentID {
		return nil, fmt.Errorf(
			"%w: review-return target differs from Assignment return target",
			ErrInvariant,
		)
	}
	item.DedupeKey = strings.TrimSpace(item.DedupeKey)
	item.Instruction = strings.TrimSpace(item.Instruction)
	if item.DedupeKey == "" {
		item.DedupeKey = "review-return:" + submission.ID
	}
	if item.ID == "" || item.CommandID == "" || item.TargetAgentID == "" ||
		item.Instruction == "" {
		return nil, fmt.Errorf(
			"%w: review-return Dispatch identity, target and instruction are required",
			ErrInvariant,
		)
	}
	if item.Status == "" {
		item.Status = protocol.ExecutionReviewDispatchStatusPending
	}
	if item.Status != protocol.ExecutionReviewDispatchStatusPending {
		return nil, fmt.Errorf(
			"%w: new review-return Dispatch must be pending",
			ErrInvariant,
		)
	}
	item.DeliveryAttempts = 0
	item.Version = 1
	item.AvailableAt = timeOr(item.AvailableAt, now)
	item.CreatedAt = timeOr(item.CreatedAt, now)
	item.UpdatedAt = item.CreatedAt
	item.HandoffID = ""
	item.QueueItemID = ""
	item.LeaseOwner = ""
	item.LeaseExpiresAt = nil
	item.ClaimedAt = nil
	item.DeliveredAt = nil
	item.LastError = ""
	return &item, nil
}

func (r *Repository) insertReviewDispatch(
	ctx context.Context,
	tx *sql.Tx,
	item protocol.ExecutionReviewDispatch,
) error {
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_review_dispatches (
    review_dispatch_id, execution_id, plan_id, work_item_id, spec_id,
    assignment_id, submission_id, command_id, dedupe_key, target_agent_id,
    status, instruction, handoff_id, queue_item_id, delivery_attempts, version,
    available_at, lease_owner, lease_expires_at, created_at, updated_at,
    claimed_at, delivered_at, last_error, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+
		r.bind(16)+`,`+r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`,`+r.bind(20)+`,`+
		r.bind(21)+`,`+r.bind(22)+`,`+r.bind(23)+`,`+r.bind(24)+`,`+r.jsonBind(25)+`)`,
		item.ID, item.ExecutionID, item.PlanID, item.WorkItemID, item.SpecID,
		item.AssignmentID, item.SubmissionID, item.CommandID, item.DedupeKey,
		item.TargetAgentID, item.Status, item.Instruction, nullString(item.HandoffID),
		nullString(item.QueueItemID), item.DeliveryAttempts, item.Version,
		r.timestamp(item.AvailableAt), nullString(item.LeaseOwner),
		nullTime(item.LeaseExpiresAt), r.timestamp(item.CreatedAt),
		r.timestamp(item.UpdatedAt), nullTime(item.ClaimedAt),
		nullTime(item.DeliveredAt), nullString(item.LastError), metadataJSON,
	)
	return err
}

// ListAvailableReviewDispatches 返回 active/waiting Room Execution 当前可 claim 的回交项。
func (r *Repository) ListAvailableReviewDispatches(
	ctx context.Context,
	limit int,
) ([]protocol.ExecutionReviewDispatch, error) {
	if limit <= 0 {
		limit = 32
	}
	if limit > 256 {
		limit = 256
	}
	now := r.currentTime()
	rows, err := r.db.QueryContext(ctx, r.reviewDispatchSelect("review_dispatch.")+`
FROM execution_review_dispatches review_dispatch
JOIN executions execution
  ON execution.execution_id = review_dispatch.execution_id
WHERE execution.status IN ('active', 'waiting')
  AND (
      (
          review_dispatch.status IN ('pending', 'failed')
          AND review_dispatch.available_at <= `+r.bind(1)+`
      )
      OR (
          review_dispatch.status = 'claimed'
          AND review_dispatch.lease_expires_at IS NOT NULL
          AND review_dispatch.lease_expires_at <= `+r.bind(2)+`
      )
  )
ORDER BY review_dispatch.available_at, review_dispatch.created_at,
         review_dispatch.review_dispatch_id
LIMIT `+r.bind(3),
		r.timestamp(now), r.timestamp(now), limit,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanReviewDispatch)
}

// ClaimReviewDispatch 用 lease + row version 认领一个仍待 review 的 Submission 回交。
func (r *Repository) ClaimReviewDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	leaseDuration time.Duration,
) (*protocol.ExecutionReviewDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if dispatchID == "" || expectedVersion <= 0 || leaseOwner == "" {
		return nil, fmt.Errorf(
			"%w: review dispatch id, expected version and lease owner are required",
			ErrInvariant,
		)
	}
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_review_dispatches
SET status = 'claimed',
    delivery_attempts = delivery_attempts + 1,
    version = version + 1,
    lease_owner = `+r.bind(1)+`,
    lease_expires_at = `+r.bind(2)+`,
    claimed_at = `+r.bind(3)+`,
    updated_at = `+r.bind(4)+`,
    last_error = NULL
WHERE review_dispatch_id = `+r.bind(5)+`
  AND version = `+r.bind(6)+`
  AND EXISTS (
      SELECT 1
      FROM executions execution
      JOIN execution_plan_revisions plan
        ON plan.plan_id = execution_review_dispatches.plan_id
       AND plan.execution_id = execution.execution_id
       AND plan.status = 'active'
      JOIN execution_work_assignments assignment
        ON assignment.assignment_id = execution_review_dispatches.assignment_id
       AND assignment.execution_id = execution_review_dispatches.execution_id
       AND assignment.plan_id = execution_review_dispatches.plan_id
       AND assignment.work_item_id = execution_review_dispatches.work_item_id
       AND assignment.spec_id = execution_review_dispatches.spec_id
       AND assignment.return_to_agent_id = execution_review_dispatches.target_agent_id
       AND assignment.status = 'active'
      WHERE execution.execution_id = execution_review_dispatches.execution_id
        AND execution.status IN ('active', 'waiting')
        AND NOT EXISTS (
            SELECT 1
            FROM execution_acceptances acceptance
            WHERE acceptance.submission_id = execution_review_dispatches.submission_id
        )
  )
  AND (
      (
          status IN ('pending', 'failed')
          AND available_at <= `+r.bind(7)+`
      )
      OR (
          status = 'claimed'
          AND lease_expires_at IS NOT NULL
          AND lease_expires_at <= `+r.bind(8)+`
      )
  )`,
		leaseOwner,
		r.timestamp(now.Add(leaseDuration)),
		r.timestamp(now),
		r.timestamp(now),
		dispatchID,
		expectedVersion,
		r.timestamp(now),
		r.timestamp(now),
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
	return r.getReviewDispatch(ctx, r.db, dispatchID)
}

// MarkReviewDispatchDelivered ACK 已同步落入 Room durable handoff/queue。
func (r *Repository) MarkReviewDispatchDelivered(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	handoffID string,
	queueItemID string,
) (*protocol.ExecutionReviewDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	handoffID = strings.TrimSpace(handoffID)
	queueItemID = strings.TrimSpace(queueItemID)
	if dispatchID == "" || expectedVersion <= 0 || leaseOwner == "" ||
		handoffID == "" || queueItemID == "" {
		return nil, fmt.Errorf(
			"%w: review delivery identity and receipt are required",
			ErrInvariant,
		)
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_review_dispatches
SET status = 'delivered',
    handoff_id = `+r.bind(1)+`,
    queue_item_id = `+r.bind(2)+`,
    version = version + 1,
    lease_owner = NULL,
    lease_expires_at = NULL,
    delivered_at = `+r.bind(3)+`,
    updated_at = `+r.bind(4)+`,
    last_error = NULL
WHERE review_dispatch_id = `+r.bind(5)+`
  AND version = `+r.bind(6)+`
  AND status = 'claimed'
  AND lease_owner = `+r.bind(7),
		handoffID, queueItemID, r.timestamp(now), r.timestamp(now),
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
		current, getErr := r.getReviewDispatch(ctx, r.db, dispatchID)
		if getErr != nil {
			return nil, getErr
		}
		if current != nil &&
			current.Status == protocol.ExecutionReviewDispatchStatusDelivered &&
			current.HandoffID == handoffID &&
			current.QueueItemID == queueItemID {
			return current, nil
		}
		return nil, ErrDispatchLease
	}
	return r.getReviewDispatch(ctx, r.db, dispatchID)
}

// RetryReviewDispatch 释放当前 lease 并按同一 outbox row 重试。
func (r *Repository) RetryReviewDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	retryAt time.Time,
	cause string,
) (*protocol.ExecutionReviewDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	cause = strings.TrimSpace(cause)
	if dispatchID == "" || expectedVersion <= 0 || leaseOwner == "" || cause == "" {
		return nil, fmt.Errorf(
			"%w: review retry identity, lease owner and cause are required",
			ErrInvariant,
		)
	}
	now := r.currentTime()
	if retryAt.IsZero() || retryAt.Before(now) {
		retryAt = now
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_review_dispatches
SET status = 'pending',
    version = version + 1,
    available_at = `+r.bind(1)+`,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = `+r.bind(2)+`,
    last_error = `+r.bind(3)+`
WHERE review_dispatch_id = `+r.bind(4)+`
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
		current, getErr := r.getReviewDispatch(ctx, r.db, dispatchID)
		if getErr != nil {
			return nil, getErr
		}
		if current != nil &&
			(current.Status == protocol.ExecutionReviewDispatchStatusDelivered ||
				current.Status == protocol.ExecutionReviewDispatchStatusCancelled) {
			return current, nil
		}
		return nil, ErrDispatchLease
	}
	return r.getReviewDispatch(ctx, r.db, dispatchID)
}

// CancelReviewDispatch 收束已 claim 但不再允许投递的迟到回交。
func (r *Repository) CancelReviewDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	reason string,
) (*protocol.ExecutionReviewDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	reason = strings.TrimSpace(reason)
	if dispatchID == "" || expectedVersion <= 0 || leaseOwner == "" || reason == "" {
		return nil, fmt.Errorf(
			"%w: review cancellation identity, lease owner and reason are required",
			ErrInvariant,
		)
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_review_dispatches
SET status = 'cancelled',
    version = version + 1,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = `+r.bind(1)+`,
    last_error = `+r.bind(2)+`
WHERE review_dispatch_id = `+r.bind(3)+`
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
		current, getErr := r.getReviewDispatch(ctx, r.db, dispatchID)
		if getErr != nil {
			return nil, getErr
		}
		if current != nil &&
			(current.Status == protocol.ExecutionReviewDispatchStatusCancelled ||
				current.Status == protocol.ExecutionReviewDispatchStatusDelivered) {
			return current, nil
		}
		return nil, ErrDispatchLease
	}
	return r.getReviewDispatch(ctx, r.db, dispatchID)
}

func (r *Repository) getReviewDispatch(
	ctx context.Context,
	queryer interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	dispatchID string,
) (*protocol.ExecutionReviewDispatch, error) {
	item, err := scanReviewDispatch(queryer.QueryRowContext(
		ctx,
		r.reviewDispatchSelect("")+`
FROM execution_review_dispatches
WHERE review_dispatch_id = `+r.bind(1),
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
