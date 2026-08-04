// INPUT: Assignment-bound Attempt start/terminal command、open current spec fence 与 runtime identities。
// OUTPUT: 新建/激活 Attempt、waiting_input 硬门槛、终态证据与审计事件。
// POS: Hook/runtime lifecycle 到 durable Attempt 的 CAS 边界。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// StartAttempt 创建 running Attempt，或激活 Assign 时预建的 pending Attempt。
func (r *Repository) StartAttempt(ctx context.Context, command StartAttemptCommand) (*protocol.ExecutionSnapshot, error) {
	attempt := command.Attempt
	attempt.ID = strings.TrimSpace(attempt.ID)
	attempt.AssignmentID = strings.TrimSpace(attempt.AssignmentID)
	if attempt.ID == "" || attempt.ExecutionID == "" || attempt.AssignmentID == "" {
		return nil, fmt.Errorf("%w: Attempt identity is required", ErrInvariant)
	}
	mutation, err := r.beginMutation(
		ctx, attempt.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventAttemptStarted,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	assignment, err := r.getAssignment(ctx, mutation.tx, attempt.AssignmentID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if assignment == nil || !attemptMatchesAssignment(attempt, *assignment) {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Attempt is outside current Assignment", ErrInvariant)
	}
	state, err := r.getState(ctx, mutation.tx, attempt.WorkItemID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if state == nil || state.ExecutionID != attempt.ExecutionID ||
		state.CurrentSpecID != attempt.SpecID ||
		state.Status != protocol.WorkItemStatusOpen {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Work Item current spec is not open", ErrWorkNotReady)
	}
	if assignment.Status != protocol.WorkAssignmentStatusAssigned &&
		assignment.Status != protocol.WorkAssignmentStatusActive {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Attempt is outside current Assignment", ErrInvariant)
	}
	if err = validateExpectedVersion(command.ExpectedAssignmentVersion, "expected Assignment version"); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	now := r.currentTime()
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE execution_work_assignments
SET status = 'active',
    version = version + 1,
    activated_at = COALESCE(activated_at, `+r.bind(1)+`)
WHERE assignment_id = `+r.bind(2)+`
  AND version = `+r.bind(3)+`
  AND status IN ('assigned', 'active')`,
		r.timestamp(now), assignment.ID, command.ExpectedAssignmentVersion,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	existing, err := r.getAttempt(ctx, mutation.tx, attempt.ID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	var entityVersion int64
	if existing == nil {
		if command.ExpectedAttemptVersion != 0 {
			r.abortMutation(mutation)
			return nil, ErrVersionConflict
		}
		if err = r.normalizeNewRunningAttempt(ctx, mutation.tx, &attempt, *assignment, now); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
		if err = r.insertAttempt(ctx, mutation.tx, attempt); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
		entityVersion = attempt.Version
	} else {
		if !attemptMatchesAssignment(*existing, *assignment) ||
			existing.Status != protocol.WorkAttemptStatusPending {
			r.abortMutation(mutation)
			return nil, fmt.Errorf("%w: only pending Attempt can start", ErrInvariant)
		}
		if err = validateExpectedVersion(command.ExpectedAttemptVersion, "expected Attempt version"); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
		merged := mergeAttemptRuntime(*existing, attempt)
		metadataJSON, marshalErr := marshalMap(merged.Metadata)
		if marshalErr != nil {
			r.abortMutation(mutation)
			return nil, marshalErr
		}
		result, err = mutation.tx.ExecContext(ctx, `
UPDATE execution_attempts
SET executor_agent_id = `+r.bind(1)+`,
    parent_agent_id = `+r.bind(2)+`,
    runtime_session_key = `+r.bind(3)+`,
    room_session_id = `+r.bind(4)+`,
    sdk_session_id = `+r.bind(5)+`,
    runtime_round_id = `+r.bind(6)+`,
    root_round_id = `+r.bind(7)+`,
    agent_round_id = `+r.bind(8)+`,
    child_session_id = `+r.bind(9)+`,
    sdk_task_id = `+r.bind(10)+`,
    tool_use_id = `+r.bind(11)+`,
    status = 'running',
    version = version + 1,
    started_at = `+r.bind(12)+`,
    metadata_json = `+r.jsonBind(13)+`
WHERE attempt_id = `+r.bind(14)+`
  AND version = `+r.bind(15)+`
  AND status = 'pending'`,
			nullString(merged.ExecutorAgentID), nullString(merged.ParentAgentID),
			nullString(merged.RuntimeSessionKey), nullString(merged.RoomSessionID),
			nullString(merged.SDKSessionID), nullString(merged.RuntimeRoundID),
			nullString(merged.RootRoundID), nullString(merged.AgentRoundID),
			nullString(merged.ChildSessionID), nullString(merged.SDKTaskID),
			nullString(merged.ToolUseID), r.timestamp(now), metadataJSON,
			existing.ID, command.ExpectedAttemptVersion,
		)
		if err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
		if err = requireOne(result); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
		entityVersion = command.ExpectedAttemptVersion + 1
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityAttempt,
		EntityID:      attempt.ID,
		EntityVersion: entityVersion,
		PlanID:        assignment.PlanID,
		WorkItemID:    assignment.WorkItemID,
		SpecID:        assignment.SpecID,
		AssignmentID:  assignment.ID,
		AttemptID:     attempt.ID,
	})
}

// FinishAttempt 把 pending/running Attempt 变为不可逆终态。
func (r *Repository) FinishAttempt(ctx context.Context, command FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
	attempt := command.Attempt
	if !terminalAttemptStatus(attempt.Status) {
		return nil, fmt.Errorf("%w: Attempt status %q is not terminal", ErrInvariant, attempt.Status)
	}
	mutation, err := r.beginMutation(
		ctx, attempt.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventAttemptTerminal,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	existing, err := r.getAttempt(ctx, mutation.tx, attempt.ID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if existing == nil || existing.ExecutionID != attempt.ExecutionID ||
		(existing.Status != protocol.WorkAttemptStatusPending &&
			existing.Status != protocol.WorkAttemptStatusRunning) {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Attempt is missing or already terminal", ErrInvariant)
	}
	if err = validateExpectedVersion(command.ExpectedAttemptVersion, "expected Attempt version"); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	merged := mergeAttemptRuntime(*existing, attempt)
	merged.Status = attempt.Status
	merged.FailureReason = attempt.FailureReason
	finishedAt := r.currentTime()
	if attempt.FinishedAt != nil {
		finishedAt = attempt.FinishedAt.UTC()
	}
	metadataJSON, err := marshalMap(merged.Metadata)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE execution_attempts
SET executor_agent_id = `+r.bind(1)+`,
    parent_agent_id = `+r.bind(2)+`,
    runtime_session_key = `+r.bind(3)+`,
    room_session_id = `+r.bind(4)+`,
    sdk_session_id = `+r.bind(5)+`,
    runtime_round_id = `+r.bind(6)+`,
    root_round_id = `+r.bind(7)+`,
    agent_round_id = `+r.bind(8)+`,
    child_session_id = `+r.bind(9)+`,
    sdk_task_id = `+r.bind(10)+`,
    tool_use_id = `+r.bind(11)+`,
	    status = `+r.bind(12)+`,
	    failure_reason = `+r.bind(13)+`,
	    version = version + 1,
	    finished_at = `+r.bind(14)+`,
	    parent_round_exited_at = `+r.bind(15)+`,
	    reconcile_after = `+r.bind(16)+`,
	    metadata_json = `+r.jsonBind(17)+`
	WHERE attempt_id = `+r.bind(18)+`
	  AND version = `+r.bind(19)+`
	  AND status IN ('pending', 'running')`,
		nullString(merged.ExecutorAgentID), nullString(merged.ParentAgentID),
		nullString(merged.RuntimeSessionKey), nullString(merged.RoomSessionID),
		nullString(merged.SDKSessionID), nullString(merged.RuntimeRoundID),
		nullString(merged.RootRoundID), nullString(merged.AgentRoundID),
		nullString(merged.ChildSessionID), nullString(merged.SDKTaskID),
		nullString(merged.ToolUseID), merged.Status, nullString(merged.FailureReason),
		r.timestamp(finishedAt), nullTime(merged.ParentRoundExitedAt),
		nullTime(merged.ReconcileAfter), metadataJSON, merged.ID,
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
		EntityID:      merged.ID,
		EntityVersion: command.ExpectedAttemptVersion + 1,
		PlanID:        merged.PlanID,
		WorkItemID:    merged.WorkItemID,
		SpecID:        merged.SpecID,
		AssignmentID:  merged.AssignmentID,
		AttemptID:     merged.ID,
	})
}

func (r *Repository) normalizeNewRunningAttempt(
	ctx context.Context,
	tx *sql.Tx,
	item *protocol.WorkAttempt,
	assignment protocol.WorkAssignment,
	now time.Time,
) error {
	if !attemptMatchesAssignment(*item, assignment) {
		return fmt.Errorf("%w: Attempt chain differs from Assignment", ErrInvariant)
	}
	if item.ParentAttemptID == "" {
		if item.ExecutorKind != protocol.AttemptExecutorAgent {
			return fmt.Errorf("%w: root Attempt must use agent executor", ErrInvariant)
		}
		if item.ExecutorAgentID == "" {
			item.ExecutorAgentID = assignment.OwnerAgentID
		}
		if item.ExecutorAgentID != assignment.OwnerAgentID {
			return fmt.Errorf("%w: root Attempt executor must be Assignment owner", ErrInvariant)
		}
		var current int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(1) FROM execution_attempts
WHERE assignment_id = `+r.bind(1)+`
  AND parent_attempt_id IS NULL
  AND status IN ('pending', 'running')`,
			assignment.ID,
		).Scan(&current); err != nil {
			return err
		}
		if current != 0 {
			return fmt.Errorf("%w: Assignment already has a current root Attempt", ErrInvariant)
		}
	} else {
		if item.ExecutorKind != protocol.AttemptExecutorSubagent ||
			strings.TrimSpace(item.ParentAgentID) == "" {
			return fmt.Errorf("%w: child Attempt requires subagent executor and parent agent", ErrInvariant)
		}
		item.ToolUseID = strings.TrimSpace(item.ToolUseID)
		if item.ToolUseID == "" {
			return fmt.Errorf("%w: child Attempt requires an exact tool_use_id", ErrInvariant)
		}
		parent, err := r.getAttempt(ctx, tx, item.ParentAttemptID)
		if err != nil {
			return err
		}
		if parent == nil || parent.AssignmentID != assignment.ID ||
			parent.Status != protocol.WorkAttemptStatusRunning {
			return fmt.Errorf("%w: parent Attempt is not running in this Assignment", ErrInvariant)
		}
		var duplicateBinding int
		if err = tx.QueryRowContext(ctx, `
SELECT COUNT(1) FROM execution_attempts
WHERE parent_attempt_id = `+r.bind(1)+`
  AND tool_use_id = `+r.bind(2)+`
  AND status IN ('pending', 'running')`,
			item.ParentAttemptID,
			item.ToolUseID,
		).Scan(&duplicateBinding); err != nil {
			return err
		}
		if duplicateBinding != 0 {
			return fmt.Errorf("%w: parent Attempt already has a child for this tool_use_id", ErrInvariant)
		}
	}
	item.Status = protocol.WorkAttemptStatusRunning
	item.Version = 1
	item.CreatedAt = timeOr(item.CreatedAt, now)
	startedAt := now
	item.StartedAt = &startedAt
	item.FinishedAt = nil
	return nil
}

func attemptMatchesAssignment(item protocol.WorkAttempt, assignment protocol.WorkAssignment) bool {
	return item.ExecutionID == assignment.ExecutionID &&
		item.PlanID == assignment.PlanID &&
		item.WorkItemID == assignment.WorkItemID &&
		item.SpecID == assignment.SpecID &&
		item.AssignmentID == assignment.ID
}

func mergeAttemptRuntime(base protocol.WorkAttempt, update protocol.WorkAttempt) protocol.WorkAttempt {
	if update.ExecutorAgentID != "" {
		base.ExecutorAgentID = update.ExecutorAgentID
	}
	if update.ParentAgentID != "" {
		base.ParentAgentID = update.ParentAgentID
	}
	if update.RuntimeSessionKey != "" {
		base.RuntimeSessionKey = update.RuntimeSessionKey
	}
	if update.RoomSessionID != "" {
		base.RoomSessionID = update.RoomSessionID
	}
	if update.SDKSessionID != "" {
		base.SDKSessionID = update.SDKSessionID
	}
	if update.RuntimeRoundID != "" {
		base.RuntimeRoundID = update.RuntimeRoundID
	}
	if update.RootRoundID != "" {
		base.RootRoundID = update.RootRoundID
	}
	if update.AgentRoundID != "" {
		base.AgentRoundID = update.AgentRoundID
	}
	if update.ChildSessionID != "" {
		base.ChildSessionID = update.ChildSessionID
	}
	if update.SDKTaskID != "" {
		base.SDKTaskID = update.SDKTaskID
	}
	if update.ToolUseID != "" {
		base.ToolUseID = update.ToolUseID
	}
	if update.Metadata != nil {
		base.Metadata = update.Metadata
	}
	return base
}

func terminalAttemptStatus(status protocol.WorkAttemptStatus) bool {
	switch status {
	case protocol.WorkAttemptStatusSucceeded,
		protocol.WorkAttemptStatusFailed,
		protocol.WorkAttemptStatusInterrupted,
		protocol.WorkAttemptStatusCancelled,
		protocol.WorkAttemptStatusTimedOut:
		return true
	default:
		return false
	}
}

func (r *Repository) insertAttempt(ctx context.Context, tx *sql.Tx, item protocol.WorkAttempt) error {
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_attempts (
    attempt_id, execution_id, plan_id, work_item_id, spec_id, assignment_id,
    dispatch_id, parent_attempt_id, executor_kind, executor_agent_id, parent_agent_id,
    runtime_session_key, room_session_id, sdk_session_id, runtime_round_id,
    root_round_id, agent_round_id, child_session_id, sdk_task_id, tool_use_id,
	    status, failure_reason, version, created_at, started_at, finished_at,
	    parent_round_exited_at, reconcile_after, metadata_json
	) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+
		r.bind(16)+`,`+r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`,`+r.bind(20)+`,`+
		r.bind(21)+`,`+r.bind(22)+`,`+r.bind(23)+`,`+r.bind(24)+`,`+r.bind(25)+`,`+
		r.bind(26)+`,`+r.bind(27)+`,`+r.bind(28)+`,`+r.jsonBind(29)+`)`,
		item.ID, item.ExecutionID, item.PlanID, item.WorkItemID, item.SpecID, item.AssignmentID,
		nullString(item.DispatchID), nullString(item.ParentAttemptID), item.ExecutorKind,
		nullString(item.ExecutorAgentID), nullString(item.ParentAgentID),
		nullString(item.RuntimeSessionKey), nullString(item.RoomSessionID), nullString(item.SDKSessionID),
		nullString(item.RuntimeRoundID), nullString(item.RootRoundID), nullString(item.AgentRoundID),
		nullString(item.ChildSessionID), nullString(item.SDKTaskID), nullString(item.ToolUseID),
		item.Status, nullString(item.FailureReason), item.Version, r.timestamp(item.CreatedAt),
		nullTime(item.StartedAt), nullTime(item.FinishedAt),
		nullTime(item.ParentRoundExitedAt), nullTime(item.ReconcileAfter), metadataJSON,
	)
	return err
}
