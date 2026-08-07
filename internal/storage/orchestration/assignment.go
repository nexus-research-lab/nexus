// INPUT: Ready Work Item 的 Assignment、可选 Dispatch/root Attempt 与 takeover command。
// OUTPUT: 同事务 current owner、outbox、pending execution 与审计事件。
// POS: WorkGraph responsibility 到 Room/runtime 投递的原子边界。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Assign 为 Ready Work Item 创建唯一 current Assignment。
func (r *Repository) Assign(ctx context.Context, command AssignCommand) (*protocol.ExecutionSnapshot, error) {
	assignment := command.Assignment
	if err := normalizeAssignment(&assignment, r.currentTime()); err != nil {
		return nil, err
	}
	mutation, err := r.beginMutation(
		ctx, assignment.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventWorkAssigned,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	if err = r.validateAssignmentTarget(ctx, mutation.tx, assignment, true); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	dispatch, err := r.normalizeDispatch(command.Dispatch, assignment, command.Meta, r.currentTime())
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	rootAttempt, err := normalizeRootAttempt(command.RootAttempt, assignment, dispatch, r.currentTime())
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = r.insertAssignment(ctx, mutation.tx, assignment); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if dispatch != nil {
		if err = r.insertDispatch(ctx, mutation.tx, *dispatch); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	if rootAttempt != nil {
		if err = r.insertAttempt(ctx, mutation.tx, *rootAttempt); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	event := protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityAssignment,
		EntityID:      assignment.ID,
		EntityVersion: assignment.Version,
		PlanID:        assignment.PlanID,
		WorkItemID:    assignment.WorkItemID,
		SpecID:        assignment.SpecID,
		AssignmentID:  assignment.ID,
	}
	if dispatch != nil {
		event.DispatchID = dispatch.ID
	}
	if rootAttempt != nil {
		event.AttemptID = rootAttempt.ID
	}
	return r.finishMutation(ctx, mutation, command.Meta, event)
}

// Takeover 原子释放 current Assignment，终止其活动投递/执行并创建 replacement。
func (r *Repository) Takeover(ctx context.Context, command TakeoverCommand) (*protocol.ExecutionSnapshot, error) {
	replacement := command.Replacement
	if err := normalizeAssignment(&replacement, r.currentTime()); err != nil {
		return nil, err
	}
	command.CurrentAssignmentID = strings.TrimSpace(command.CurrentAssignmentID)
	if command.CurrentAssignmentID == "" || strings.TrimSpace(replacement.TakeoverReason) == "" {
		return nil, fmt.Errorf("%w: current assignment and takeover reason are required", ErrInvariant)
	}
	mutation, err := r.beginMutation(
		ctx, replacement.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventWorkTakenOver,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	current, err := r.getAssignment(ctx, mutation.tx, command.CurrentAssignmentID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if current == nil || current.ExecutionID != replacement.ExecutionID ||
		current.PlanID != replacement.PlanID || current.WorkItemID != replacement.WorkItemID ||
		current.SpecID != replacement.SpecID {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: replacement is outside current Assignment chain", ErrInvariant)
	}
	if err = validateExpectedVersion(command.ExpectedCurrentAssignmentVersion, "expected current Assignment version"); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = r.rejectUnreviewedSubmission(
		ctx,
		mutation.tx,
		replacement.ExecutionID,
		replacement.WorkItemID,
		replacement.SpecID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = r.validateAssignmentTarget(ctx, mutation.tx, replacement, false); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	now := r.currentTime()
	cancellationReason := "takeover: " + replacement.TakeoverReason
	if err = r.enqueueAttemptCancellations(
		ctx,
		mutation.tx,
		cancellationAttemptScope{
			ExecutionID:  replacement.ExecutionID,
			AssignmentID: current.ID,
		},
		command.Meta.CommandID,
		cancellationReason,
		now,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE execution_work_assignments
SET status = 'released',
    takeover_reason = `+r.bind(1)+`,
    version = version + 1,
    released_at = `+r.bind(2)+`
WHERE assignment_id = `+r.bind(3)+`
  AND version = `+r.bind(4)+`
  AND status IN ('assigned', 'active')`,
		replacement.TakeoverReason, r.timestamp(now), current.ID,
		command.ExpectedCurrentAssignmentVersion,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if _, err = mutation.tx.ExecContext(ctx, `
UPDATE execution_attempts
SET status = 'interrupted',
    failure_reason = `+r.bind(1)+`,
    version = version + 1,
    finished_at = `+r.bind(2)+`
WHERE assignment_id = `+r.bind(3)+`
  AND status IN ('pending', 'running')`,
		cancellationReason, r.timestamp(now), current.ID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if _, err = mutation.tx.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'cancelled',
    last_error = `+r.bind(1)+`,
    version = version + 1,
    updated_at = `+r.bind(2)+`
WHERE assignment_id = `+r.bind(3)+`
  AND status IN ('pending', 'claimed')`,
		cancellationReason, r.timestamp(now), current.ID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	dispatch, err := r.normalizeDispatch(command.Dispatch, replacement, command.Meta, now)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	rootAttempt, err := normalizeRootAttempt(command.RootAttempt, replacement, dispatch, now)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = r.insertAssignment(ctx, mutation.tx, replacement); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if dispatch != nil {
		if err = r.insertDispatch(ctx, mutation.tx, *dispatch); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	if rootAttempt != nil {
		if err = r.insertAttempt(ctx, mutation.tx, *rootAttempt); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	event := protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityAssignment,
		EntityID:      replacement.ID,
		EntityVersion: replacement.Version,
		PlanID:        replacement.PlanID,
		WorkItemID:    replacement.WorkItemID,
		SpecID:        replacement.SpecID,
		AssignmentID:  replacement.ID,
	}
	if dispatch != nil {
		event.DispatchID = dispatch.ID
	}
	if rootAttempt != nil {
		event.AttemptID = rootAttempt.ID
	}
	return r.finishMutation(ctx, mutation, command.Meta, event)
}

func normalizeAssignment(item *protocol.WorkAssignment, now time.Time) error {
	item.ID = strings.TrimSpace(item.ID)
	item.ExecutionID = strings.TrimSpace(item.ExecutionID)
	item.PlanID = strings.TrimSpace(item.PlanID)
	item.WorkItemID = strings.TrimSpace(item.WorkItemID)
	item.SpecID = strings.TrimSpace(item.SpecID)
	item.OwnerAgentID = strings.TrimSpace(item.OwnerAgentID)
	item.AssignedByAgentID = strings.TrimSpace(item.AssignedByAgentID)
	item.ReturnToAgentID = strings.TrimSpace(item.ReturnToAgentID)
	if item.ReturnToAgentID == "" {
		item.ReturnToAgentID = item.AssignedByAgentID
	}
	if item.ID == "" || item.ExecutionID == "" || item.PlanID == "" ||
		item.WorkItemID == "" || item.SpecID == "" || item.OwnerAgentID == "" ||
		item.ReturnToAgentID == "" {
		return fmt.Errorf("%w: Assignment chain, owner and return target are required", ErrInvariant)
	}
	if item.Strategy != protocol.AssignmentStrategySelf &&
		item.Strategy != protocol.AssignmentStrategyRoomMember {
		return fmt.Errorf("%w: Assignment strategy %q is invalid", ErrInvariant, item.Strategy)
	}
	if item.Status == "" {
		item.Status = protocol.WorkAssignmentStatusAssigned
	}
	if item.Status != protocol.WorkAssignmentStatusAssigned {
		return fmt.Errorf("%w: new Assignment must be assigned", ErrInvariant)
	}
	item.Version = 1
	item.AssignedAt = timeOr(item.AssignedAt, now)
	item.ActivatedAt = nil
	item.ReleasedAt = nil
	item.CompletedAt = nil
	return nil
}

func (r *Repository) validateAssignmentTarget(
	ctx context.Context,
	tx *sql.Tx,
	assignment protocol.WorkAssignment,
	requireNoCurrent bool,
) error {
	active, err := r.getActivePlan(ctx, tx, assignment.ExecutionID)
	if err != nil {
		return err
	}
	if active == nil || active.ID != assignment.PlanID {
		return fmt.Errorf("%w: Assignment Plan is not active", ErrWorkNotReady)
	}
	var membershipCount int
	if err = tx.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_plan_items
WHERE plan_id = `+r.bind(1)+`
  AND execution_id = `+r.bind(2)+`
  AND work_item_id = `+r.bind(3)+`
  AND spec_id = `+r.bind(4),
		assignment.PlanID, assignment.ExecutionID, assignment.WorkItemID, assignment.SpecID,
	).Scan(&membershipCount); err != nil {
		return err
	}
	if membershipCount != 1 {
		return fmt.Errorf("%w: Assignment Work Item/spec is outside active Plan", ErrInvariant)
	}
	ready, reason, err := r.workEligible(ctx, tx, assignment.PlanID, assignment.WorkItemID, assignment.SpecID)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("%w: %s", ErrWorkNotReady, reason)
	}
	if requireNoCurrent {
		var current int
		if err = tx.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_work_assignments
WHERE work_item_id = `+r.bind(1)+`
  AND status IN ('assigned', 'active')`,
			assignment.WorkItemID,
		).Scan(&current); err != nil {
			return err
		}
		if current != 0 {
			return fmt.Errorf("%w: Work Item already has a current Assignment", ErrWorkNotReady)
		}
	}
	return nil
}

func (r *Repository) normalizeDispatch(
	source *protocol.ExecutionDispatch,
	assignment protocol.WorkAssignment,
	meta CommandMeta,
	now time.Time,
) (*protocol.ExecutionDispatch, error) {
	if source == nil {
		return nil, nil
	}
	item := *source
	item.ID = strings.TrimSpace(item.ID)
	item.ExecutionID = assignment.ExecutionID
	item.PlanID = assignment.PlanID
	item.WorkItemID = assignment.WorkItemID
	item.SpecID = assignment.SpecID
	item.AssignmentID = assignment.ID
	item.CommandID = strings.TrimSpace(meta.CommandID)
	item.TargetAgentID = strings.TrimSpace(item.TargetAgentID)
	if item.TargetAgentID == "" {
		item.TargetAgentID = assignment.OwnerAgentID
	}
	item.DedupeKey = strings.TrimSpace(item.DedupeKey)
	if item.ID == "" || item.CommandID == "" || item.DedupeKey == "" ||
		item.TargetAgentID != assignment.OwnerAgentID {
		return nil, fmt.Errorf("%w: Dispatch identity, dedupe key and target owner are required", ErrInvariant)
	}
	if item.Status == "" {
		item.Status = protocol.ExecutionDispatchStatusPending
	}
	if item.Status != protocol.ExecutionDispatchStatusPending {
		return nil, fmt.Errorf("%w: new Dispatch must be pending", ErrInvariant)
	}
	item.Version = 1
	item.AvailableAt = timeOr(item.AvailableAt, now)
	item.CreatedAt = timeOr(item.CreatedAt, now)
	item.UpdatedAt = timeOr(item.UpdatedAt, item.CreatedAt)
	item.ClaimedAt = nil
	item.DeliveredAt = nil
	return &item, nil
}

func normalizeRootAttempt(
	source *protocol.WorkAttempt,
	assignment protocol.WorkAssignment,
	dispatch *protocol.ExecutionDispatch,
	now time.Time,
) (*protocol.WorkAttempt, error) {
	if source == nil {
		return nil, nil
	}
	item := *source
	item.ID = strings.TrimSpace(item.ID)
	item.ExecutionID = assignment.ExecutionID
	item.PlanID = assignment.PlanID
	item.WorkItemID = assignment.WorkItemID
	item.SpecID = assignment.SpecID
	item.AssignmentID = assignment.ID
	item.ParentAttemptID = ""
	if dispatch != nil {
		item.DispatchID = dispatch.ID
	} else if item.DispatchID != "" {
		return nil, fmt.Errorf("%w: root Attempt references absent Dispatch", ErrInvariant)
	}
	if item.ID == "" || item.ExecutorKind != protocol.AttemptExecutorAgent {
		return nil, fmt.Errorf("%w: root Attempt requires agent executor and identity", ErrInvariant)
	}
	if item.ExecutorAgentID == "" {
		item.ExecutorAgentID = assignment.OwnerAgentID
	}
	if item.ExecutorAgentID != assignment.OwnerAgentID {
		return nil, fmt.Errorf("%w: root Attempt executor must be Assignment owner", ErrInvariant)
	}
	if item.Status == "" {
		item.Status = protocol.WorkAttemptStatusPending
	}
	if item.Status != protocol.WorkAttemptStatusPending {
		return nil, fmt.Errorf("%w: new root Attempt must be pending", ErrInvariant)
	}
	item.Version = 1
	item.CreatedAt = timeOr(item.CreatedAt, now)
	item.StartedAt = nil
	item.FinishedAt = nil
	return &item, nil
}

func (r *Repository) insertAssignment(ctx context.Context, tx *sql.Tx, item protocol.WorkAssignment) error {
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_work_assignments (
    assignment_id, execution_id, plan_id, work_item_id, spec_id,
    owner_agent_id, assigned_by_agent_id, return_to_agent_id, strategy, status,
    assignment_reason, takeover_reason, version, assigned_at,
    activated_at, released_at, completed_at, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+
		r.bind(16)+`,`+r.bind(17)+`,`+r.jsonBind(18)+`)`,
		item.ID, item.ExecutionID, item.PlanID, item.WorkItemID, item.SpecID,
		item.OwnerAgentID, nullString(item.AssignedByAgentID), nullString(item.ReturnToAgentID),
		item.Strategy, item.Status, nullString(item.AssignmentReason), nullString(item.TakeoverReason),
		item.Version, r.timestamp(item.AssignedAt), nullTime(item.ActivatedAt),
		nullTime(item.ReleasedAt), nullTime(item.CompletedAt), metadataJSON,
	)
	return err
}

func (r *Repository) insertDispatch(ctx context.Context, tx *sql.Tx, item protocol.ExecutionDispatch) error {
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_dispatches (
    dispatch_id, execution_id, plan_id, work_item_id, spec_id, assignment_id,
    command_id, dedupe_key, target_agent_id, kind, status, instruction,
    handoff_id, queue_item_id, delivery_attempts, version, available_at,
    lease_owner, lease_expires_at, created_at, updated_at, claimed_at,
    delivered_at, last_error, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+
		r.bind(16)+`,`+r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`,`+r.bind(20)+`,`+
		r.bind(21)+`,`+r.bind(22)+`,`+r.bind(23)+`,`+r.bind(24)+`,`+r.jsonBind(25)+`)`,
		item.ID, item.ExecutionID, item.PlanID, item.WorkItemID, item.SpecID,
		item.AssignmentID, item.CommandID, item.DedupeKey, item.TargetAgentID,
		item.Kind, item.Status, item.Instruction, nullString(item.HandoffID),
		nullString(item.QueueItemID), item.DeliveryAttempts, item.Version,
		r.timestamp(item.AvailableAt), nullString(item.LeaseOwner), nullTime(item.LeaseExpiresAt),
		r.timestamp(item.CreatedAt), r.timestamp(item.UpdatedAt), nullTime(item.ClaimedAt),
		nullTime(item.DeliveredAt), nullString(item.LastError), metadataJSON,
	)
	return err
}
