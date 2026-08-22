// INPUT: 将被 SQL mutation 终结的 live Attempt、精确 runtime identity 与 cancellation lease。
// OUTPUT: 同事务 capture/enqueue、可恢复 claim/retry/resolve 的 physical cancellation outbox。
// POS: 任何主动要求把 live Attempt 标为 interrupted/cancelled 的控制路径先经过此边界；
// 已物理结束的 runtime callback 只回写 terminal evidence，不重复发 cancellation。
package orchestration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type cancellationAttemptScope struct {
	ExecutionID  string
	PlanID       string
	WorkItemID   string
	SpecID       string
	AssignmentID string
}

func (r *Repository) enqueueAttemptCancellations(
	ctx context.Context,
	tx *sql.Tx,
	scope cancellationAttemptScope,
	commandID string,
	reason string,
	now time.Time,
) error {
	scope.ExecutionID = strings.TrimSpace(scope.ExecutionID)
	commandID = strings.TrimSpace(commandID)
	reason = strings.TrimSpace(reason)
	if scope.ExecutionID == "" || commandID == "" || reason == "" {
		return fmt.Errorf(
			"%w: cancellation capture requires execution, command and reason",
			ErrInvariant,
		)
	}
	execution, err := r.getExecution(ctx, tx, scope.ExecutionID)
	if err != nil {
		return err
	}
	if execution == nil {
		return sql.ErrNoRows
	}
	attempts, err := r.listCancellationAttempts(ctx, tx, scope)
	if err != nil {
		return err
	}
	for _, attempt := range attempts {
		assignment, getErr := r.getAssignment(ctx, tx, attempt.AssignmentID)
		if getErr != nil {
			return getErr
		}
		if assignment == nil {
			return fmt.Errorf(
				"%w: cancellation Attempt %q has no Assignment",
				ErrInvariant,
				attempt.ID,
			)
		}
		runtimeAttempt := attempt
		if attempt.ParentAttemptID != "" {
			parent, parentErr := r.getAttempt(ctx, tx, attempt.ParentAttemptID)
			if parentErr != nil {
				return parentErr
			}
			if parent == nil || parent.AssignmentID != attempt.AssignmentID {
				return fmt.Errorf(
					"%w: cancellation child Attempt %q has no runtime parent",
					ErrInvariant,
					attempt.ID,
				)
			}
			runtimeAttempt = *parent
		}
		dispatch := buildCancellationDispatch(
			*execution,
			*assignment,
			attempt,
			runtimeAttempt,
			commandID,
			reason,
			now,
		)
		if err = r.insertCancellationDispatch(ctx, tx, dispatch); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) listCancellationAttempts(
	ctx context.Context,
	tx *sql.Tx,
	scope cancellationAttemptScope,
) ([]protocol.WorkAttempt, error) {
	clauses := []string{"attempt.execution_id = " + r.bind(1)}
	args := []any{strings.TrimSpace(scope.ExecutionID)}
	appendClause := func(column string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		args = append(args, value)
		clauses = append(
			clauses,
			"attempt."+column+" = "+r.bind(len(args)),
		)
	}
	appendClause("plan_id", scope.PlanID)
	appendClause("work_item_id", scope.WorkItemID)
	appendClause("spec_id", scope.SpecID)
	appendClause("assignment_id", scope.AssignmentID)
	rows, err := tx.QueryContext(
		ctx,
		r.attemptSelect("attempt.")+`
FROM execution_attempts attempt
WHERE `+strings.Join(clauses, " AND ")+`
  AND attempt.status IN ('pending', 'running')
ORDER BY CASE WHEN attempt.parent_attempt_id IS NULL THEN 0 ELSE 1 END,
         attempt.created_at, attempt.attempt_id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanAttempt)
}

func buildCancellationDispatch(
	execution protocol.Execution,
	assignment protocol.WorkAssignment,
	attempt protocol.WorkAttempt,
	runtimeAttempt protocol.WorkAttempt,
	commandID string,
	reason string,
	now time.Time,
) protocol.ExecutionCancellationDispatch {
	item := protocol.ExecutionCancellationDispatch{
		ID:                cancellationDispatchID(attempt.ID),
		ExecutionID:       attempt.ExecutionID,
		PlanID:            attempt.PlanID,
		WorkItemID:        attempt.WorkItemID,
		SpecID:            attempt.SpecID,
		AssignmentID:      attempt.AssignmentID,
		AttemptID:         attempt.ID,
		RuntimeAttemptID:  runtimeAttempt.ID,
		DispatchID:        runtimeAttempt.DispatchID,
		CommandID:         commandID,
		DedupeKey:         "attempt:" + attempt.ID,
		ScopeKind:         execution.ScopeKind,
		ScopeSessionKey:   execution.SessionKey,
		RoomID:            execution.RoomID,
		ConversationID:    execution.ConversationID,
		ExecutorKind:      attempt.ExecutorKind,
		TargetAgentID:     assignment.OwnerAgentID,
		RuntimeSessionKey: runtimeAttempt.RuntimeSessionKey,
		RoomSessionID:     runtimeAttempt.RoomSessionID,
		SDKSessionID:      runtimeAttempt.SDKSessionID,
		RuntimeRoundID:    runtimeAttempt.RuntimeRoundID,
		RootRoundID:       runtimeAttempt.RootRoundID,
		AgentRoundID:      runtimeAttempt.AgentRoundID,
		ChildSessionID:    attempt.ChildSessionID,
		SDKTaskID:         attempt.SDKTaskID,
		ToolUseID:         attempt.ToolUseID,
		Status:            protocol.ExecutionCancellationDispatchPending,
		Reason:            reason,
		Version:           1,
		AvailableAt:       now,
		CreatedAt:         now,
		UpdatedAt:         now,
		Metadata: map[string]any{
			"source_attempt_status": attempt.Status,
		},
	}.Normalized()
	item.TargetKind, item.LimitationCode = cancellationDispatchTarget(item, attempt.Status)
	return item
}

// cancellationDispatchTarget 从已清洗的 dispatch 身份推导取消目标；RoomSlot
// 需要 slot 七元身份齐全，DM runtime round 需要 session 与 round 齐全。
func cancellationDispatchTarget(
	item protocol.ExecutionCancellationDispatch,
	attemptStatus protocol.WorkAttemptStatus,
) (protocol.ExecutionCancellationTargetKind, string) {
	if attemptStatus == protocol.WorkAttemptStatusPending {
		return protocol.ExecutionCancellationTargetNotStarted, "attempt_not_started"
	}
	switch item.ScopeKind {
	case protocol.ExecutionScopeRoom:
		if item.ScopeSessionKey != "" && item.RoomID != "" && item.ConversationID != "" &&
			item.TargetAgentID != "" && item.DispatchID != "" &&
			item.RuntimeSessionKey != "" && item.AgentRoundID != "" {
			return protocol.ExecutionCancellationTargetRoomSlot, ""
		}
	case protocol.ExecutionScopeDM:
		if item.RuntimeSessionKey != "" && item.RuntimeRoundID != "" {
			return protocol.ExecutionCancellationTargetRuntimeRound, ""
		}
	}
	return protocol.ExecutionCancellationTargetUnavailable, "runtime_identity_unavailable"
}

func cancellationDispatchID(attemptID string) string {
	digest := sha256.Sum256([]byte("execution-cancellation\x00" + strings.TrimSpace(attemptID)))
	return "cancel_" + hex.EncodeToString(digest[:20])
}

func (r *Repository) insertCancellationDispatch(
	ctx context.Context,
	tx *sql.Tx,
	item protocol.ExecutionCancellationDispatch,
) error {
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_cancellation_dispatches (
    cancellation_dispatch_id, execution_id, plan_id, work_item_id, spec_id,
    assignment_id, attempt_id, runtime_attempt_id, dispatch_id, command_id,
    dedupe_key, scope_kind, scope_session_key, room_id, conversation_id,
    executor_kind, target_kind, target_agent_id, runtime_session_key,
    room_session_id, sdk_session_id, runtime_round_id, root_round_id,
    agent_round_id, child_session_id, sdk_task_id, tool_use_id, status,
    reason, limitation_code, outcome, receipt, delivery_attempts, version,
    available_at, lease_owner, lease_expires_at, created_at, updated_at,
    claimed_at, delivered_at, last_error, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+
		r.bind(16)+`,`+r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`,`+r.bind(20)+`,`+
		r.bind(21)+`,`+r.bind(22)+`,`+r.bind(23)+`,`+r.bind(24)+`,`+r.bind(25)+`,`+
		r.bind(26)+`,`+r.bind(27)+`,`+r.bind(28)+`,`+r.bind(29)+`,`+r.bind(30)+`,`+
		r.bind(31)+`,`+r.bind(32)+`,`+r.bind(33)+`,`+r.bind(34)+`,`+r.bind(35)+`,`+
		r.bind(36)+`,`+r.bind(37)+`,`+r.bind(38)+`,`+r.bind(39)+`,`+r.bind(40)+`,`+
		r.bind(41)+`,`+r.bind(42)+`,`+r.jsonBind(43)+`)
ON CONFLICT(attempt_id) DO NOTHING`,
		item.ID, item.ExecutionID, item.PlanID, item.WorkItemID, item.SpecID,
		item.AssignmentID, item.AttemptID, item.RuntimeAttemptID,
		nullString(item.DispatchID), item.CommandID, item.DedupeKey, item.ScopeKind,
		item.ScopeSessionKey, nullString(item.RoomID), nullString(item.ConversationID),
		item.ExecutorKind, item.TargetKind, nullString(item.TargetAgentID),
		nullString(item.RuntimeSessionKey), nullString(item.RoomSessionID),
		nullString(item.SDKSessionID), nullString(item.RuntimeRoundID),
		nullString(item.RootRoundID), nullString(item.AgentRoundID),
		nullString(item.ChildSessionID), nullString(item.SDKTaskID),
		nullString(item.ToolUseID), item.Status, item.Reason,
		nullString(item.LimitationCode), nullString(string(item.Outcome)),
		nullString(item.Receipt), item.DeliveryAttempts, item.Version,
		r.timestamp(item.AvailableAt), nullString(item.LeaseOwner),
		nullTime(item.LeaseExpiresAt), r.timestamp(item.CreatedAt),
		r.timestamp(item.UpdatedAt), nullTime(item.ClaimedAt),
		nullTime(item.DeliveredAt), nullString(item.LastError), metadataJSON,
	)
	return err
}

// ListAvailableCancellationDispatches 返回 due pending 或 lease 已过期的 rows。
func (r *Repository) ListAvailableCancellationDispatches(
	ctx context.Context,
	limit int,
) ([]protocol.ExecutionCancellationDispatch, error) {
	if limit <= 0 {
		limit = 32
	}
	now := r.currentTime()
	rows, err := r.db.QueryContext(
		ctx,
		r.cancellationDispatchSelect("cancellation.")+`
FROM execution_cancellation_dispatches cancellation
WHERE (
        cancellation.status = 'pending'
        AND cancellation.available_at <= `+r.bind(1)+`
      )
   OR (
        cancellation.status = 'claimed'
        AND cancellation.lease_expires_at <= `+r.bind(2)+`
      )
ORDER BY cancellation.available_at, cancellation.created_at,
         cancellation.cancellation_dispatch_id
LIMIT `+r.bind(3),
		r.timestamp(now),
		r.timestamp(now),
		limit,
	)
	if err != nil {
		return nil, err
	}
	return scanRows(rows, scanCancellationDispatch)
}

// ClaimCancellationDispatch 原子获取或回收一个到期 cancellation lease。
func (r *Repository) ClaimCancellationDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	leaseDuration time.Duration,
) (*protocol.ExecutionCancellationDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	if dispatchID == "" || leaseOwner == "" || expectedVersion <= 0 {
		return nil, fmt.Errorf(
			"%w: cancellation id, expected version and lease owner are required",
			ErrInvariant,
		)
	}
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	now := r.currentTime()
	expires := now.Add(leaseDuration)
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_cancellation_dispatches
SET status = 'claimed',
    version = version + 1,
    delivery_attempts = delivery_attempts + 1,
    lease_owner = `+r.bind(1)+`,
    lease_expires_at = `+r.bind(2)+`,
    claimed_at = `+r.bind(3)+`,
    updated_at = `+r.bind(4)+`
WHERE cancellation_dispatch_id = `+r.bind(5)+`
  AND version = `+r.bind(6)+`
  AND (
      (status = 'pending' AND available_at <= `+r.bind(7)+`)
      OR (status = 'claimed' AND lease_expires_at <= `+r.bind(8)+`)
  )`,
		leaseOwner,
		r.timestamp(expires),
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
	return r.GetCancellationDispatch(ctx, dispatchID)
}

// ResolveCancellationDispatch 区分 provider interrupt、local round cancel、
// stale/already-ended no-op 或显式 not-required/unsupported 能力边界。
func (r *Repository) ResolveCancellationDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	status protocol.ExecutionCancellationDispatchStatus,
	outcome protocol.ExecutionCancellationOutcome,
	limitationCode string,
	receipt string,
) (*protocol.ExecutionCancellationDispatch, error) {
	switch status {
	case protocol.ExecutionCancellationDispatchDelivered,
		protocol.ExecutionCancellationDispatchNotRequired,
		protocol.ExecutionCancellationDispatchUnsupported:
	default:
		return nil, fmt.Errorf("%w: invalid cancellation resolution status", ErrInvariant)
	}
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	limitationCode = strings.TrimSpace(limitationCode)
	receipt = strings.TrimSpace(receipt)
	if dispatchID == "" || leaseOwner == "" || expectedVersion <= 0 || outcome == "" {
		return nil, fmt.Errorf(
			"%w: cancellation resolution identity and outcome are required",
			ErrInvariant,
		)
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_cancellation_dispatches
SET status = `+r.bind(1)+`,
    outcome = `+r.bind(2)+`,
    limitation_code = COALESCE(`+r.bind(3)+`, limitation_code),
    receipt = `+r.bind(4)+`,
    version = version + 1,
    delivered_at = `+r.bind(5)+`,
    updated_at = `+r.bind(6)+`,
    lease_owner = NULL,
    lease_expires_at = NULL,
    last_error = NULL
WHERE cancellation_dispatch_id = `+r.bind(7)+`
  AND version = `+r.bind(8)+`
  AND status = 'claimed'
  AND lease_owner = `+r.bind(9),
		status,
		outcome,
		nullString(limitationCode),
		nullString(receipt),
		r.timestamp(now),
		r.timestamp(now),
		dispatchID,
		expectedVersion,
		leaseOwner,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 1 {
		return r.GetCancellationDispatch(ctx, dispatchID)
	}
	current, getErr := r.GetCancellationDispatch(ctx, dispatchID)
	if getErr != nil {
		return nil, getErr
	}
	if current != nil && current.Status == status && current.Outcome == outcome {
		return current, nil
	}
	return nil, ErrDispatchLease
}

// RetryCancellationDispatch 释放当前 lease，并把同一 row 安排到未来重试。
func (r *Repository) RetryCancellationDispatch(
	ctx context.Context,
	dispatchID string,
	expectedVersion int64,
	leaseOwner string,
	retryAt time.Time,
	cause string,
) (*protocol.ExecutionCancellationDispatch, error) {
	dispatchID = strings.TrimSpace(dispatchID)
	leaseOwner = strings.TrimSpace(leaseOwner)
	cause = strings.TrimSpace(cause)
	if dispatchID == "" || leaseOwner == "" || expectedVersion <= 0 || cause == "" {
		return nil, fmt.Errorf(
			"%w: cancellation retry identity and cause are required",
			ErrInvariant,
		)
	}
	now := r.currentTime()
	if retryAt.IsZero() || retryAt.Before(now) {
		retryAt = now
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_cancellation_dispatches
SET status = 'pending',
    version = version + 1,
    available_at = `+r.bind(1)+`,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = `+r.bind(2)+`,
    last_error = `+r.bind(3)+`
WHERE cancellation_dispatch_id = `+r.bind(4)+`
  AND version = `+r.bind(5)+`
  AND status = 'claimed'
  AND lease_owner = `+r.bind(6),
		r.timestamp(retryAt),
		r.timestamp(now),
		cause,
		dispatchID,
		expectedVersion,
		leaseOwner,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		current, getErr := r.GetCancellationDispatch(ctx, dispatchID)
		if getErr != nil {
			return nil, getErr
		}
		if current != nil &&
			(current.Status == protocol.ExecutionCancellationDispatchDelivered ||
				current.Status == protocol.ExecutionCancellationDispatchNotRequired ||
				current.Status == protocol.ExecutionCancellationDispatchUnsupported) {
			return current, nil
		}
		return nil, ErrDispatchLease
	}
	return r.GetCancellationDispatch(ctx, dispatchID)
}

// GetCancellationDispatch 读取一个 cancellation outbox row。
func (r *Repository) GetCancellationDispatch(
	ctx context.Context,
	dispatchID string,
) (*protocol.ExecutionCancellationDispatch, error) {
	item, err := scanCancellationDispatch(r.db.QueryRowContext(
		ctx,
		r.cancellationDispatchSelect("")+`
FROM execution_cancellation_dispatches
WHERE cancellation_dispatch_id = `+r.bind(1),
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
