// INPUT: Execution aggregate root、Goal binding 与 completion command。
// OUTPUT: Execution 创建/查询、managed WorkGraph 当前/历史选择、无损 Goal 绑定及 completion receipt 同事务终结。
// POS: Execution 生命周期的 SQL 真相边界。
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Create 原子创建 Execution 与 execution_created event。
func (r *Repository) Create(ctx context.Context, command CreateCommand) (*protocol.ExecutionSnapshot, error) {
	if err := validateMeta(command.Meta); err != nil {
		return nil, err
	}
	item := command.Execution
	item.ID = strings.TrimSpace(item.ID)
	item.OwnerUserID = strings.TrimSpace(item.OwnerUserID)
	item.SessionKey = strings.TrimSpace(item.SessionKey)
	item.Objective = strings.TrimSpace(item.Objective)
	if item.ID == "" || item.OwnerUserID == "" || item.SessionKey == "" || item.Objective == "" {
		return nil, fmt.Errorf("%w: execution identity, owner, session and objective are required", ErrInvariant)
	}
	if err := protocol.ValidateExecutionProjectionLimit(
		"completion_criteria",
		len(item.CompletionCriteria),
	); err != nil {
		return nil, err
	}
	if item.ScopeKind == protocol.ExecutionScopeDM {
		if strings.TrimSpace(item.RoomID) != "" || strings.TrimSpace(item.ConversationID) != "" {
			return nil, fmt.Errorf("%w: DM execution cannot carry Room identity", ErrInvariant)
		}
	} else if item.ScopeKind == protocol.ExecutionScopeRoom {
		if strings.TrimSpace(item.RoomID) == "" || strings.TrimSpace(item.ConversationID) == "" {
			return nil, fmt.Errorf("%w: Room execution requires room and conversation identity", ErrInvariant)
		}
	} else {
		return nil, fmt.Errorf("%w: execution scope %q is invalid", ErrInvariant, item.ScopeKind)
	}
	if item.Version != 0 && item.Version != 1 {
		return nil, fmt.Errorf("%w: new execution version must be 1", ErrInvariant)
	}
	item.Version = 1
	now := r.currentTime()
	item.CreatedAt = timeOr(item.CreatedAt, now)
	item.UpdatedAt = timeOr(item.UpdatedAt, item.CreatedAt)
	criteriaJSON, err := marshalSlice(item.CompletionCriteria)
	if err != nil {
		return nil, err
	}
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return nil, err
	}
	if existing, findErr := r.findEventByCommand(ctx, r.db, item.ID, command.Meta.CommandID); findErr != nil {
		return nil, findErr
	} else if existing != nil {
		if existing.Type != protocol.ExecutionEventCreated {
			return nil, ErrCommandConflict
		}
		return r.GetSnapshot(ctx, item.ID)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
INSERT INTO executions (
    execution_id, owner_user_id, session_key, scope_kind,
    room_id, conversation_id, coordinator_agent_id, origin, objective,
    completion_criteria_json, goal_id, goal_objective_revision,
    goal_activation_origin, goal_activation_reason, recovery_of_execution_id,
    replaces_execution_id, root_round_id, trigger_message_id, status, version,
    created_at, updated_at, completed_at, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.jsonBind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.bind(15)+`,`+
		r.bind(16)+`,`+r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`,`+r.bind(20)+`,`+
		r.bind(21)+`,`+r.bind(22)+`,`+r.bind(23)+`,`+r.jsonBind(24)+`)`,
		item.ID, item.OwnerUserID, item.SessionKey, item.ScopeKind,
		nullString(item.RoomID), nullString(item.ConversationID), nullString(item.CoordinatorAgentID),
		item.Origin, item.Objective, criteriaJSON, nullString(item.GoalID),
		item.GoalObjectiveRevision, nullString(string(item.GoalActivationOrigin)),
		nullString(string(item.GoalActivationReason)), nullString(item.RecoveryOfExecutionID),
		nullString(item.ReplacesExecutionID), nullString(item.RootRoundID), nullString(item.TriggerMessageID),
		item.Status, item.Version,
		r.timestamp(item.CreatedAt), r.timestamp(item.UpdatedAt), nullTime(item.CompletedAt), metadataJSON,
	)
	if err != nil {
		_ = tx.Rollback()
		if existing, findErr := r.findEventByCommand(ctx, r.db, item.ID, command.Meta.CommandID); findErr == nil && existing != nil {
			if existing.Type == protocol.ExecutionEventCreated {
				return r.GetSnapshot(ctx, item.ID)
			}
			return nil, ErrCommandConflict
		}
		return nil, err
	}
	if err = r.ensureGoalConfirmationReceiptTx(ctx, tx, item); err != nil {
		return nil, err
	}
	event := protocol.ExecutionEvent{
		ID:             command.Meta.EventID,
		ExecutionID:    item.ID,
		Type:           protocol.ExecutionEventCreated,
		EntityType:     protocol.ExecutionEntityExecution,
		EntityID:       item.ID,
		EntityVersion:  item.Version,
		ActorKind:      command.Meta.ActorKind,
		ActorID:        command.Meta.ActorID,
		CommandID:      command.Meta.CommandID,
		GoalID:         item.GoalID,
		RootRoundID:    command.Meta.RootRoundID,
		RuntimeRoundID: command.Meta.RuntimeRoundID,
		AgentRoundID:   command.Meta.AgentRoundID,
		Payload:        command.Meta.Payload,
		CreatedAt:      timeOr(command.Meta.CreatedAt, now),
	}
	if err = r.insertEvent(ctx, tx, event); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSnapshot(ctx, item.ID)
}

// Get 按稳定 ID 读取 Execution。
func (r *Repository) Get(ctx context.Context, executionID string) (*protocol.Execution, error) {
	return r.getExecution(ctx, r.db, executionID)
}

// FindCurrent 返回 session 最近的未终结 Execution。
func (r *Repository) FindCurrent(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.Execution, error) {
	item, err := scanExecution(r.db.QueryRowContext(ctx, r.executionSelect()+`
WHERE owner_user_id = `+r.bind(1)+`
  AND session_key = `+r.bind(2)+`
  AND status IN ('active', 'waiting', 'paused')
ORDER BY updated_at DESC, execution_id DESC
LIMIT 1`,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(sessionKey),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindCurrentManaged 返回 session 最近的未终结 managed Execution。只有 active
// Plan 已包含 Work Item 时才算正式 WorkGraph，transient Execution 不得替换旧图。
func (r *Repository) FindCurrentManaged(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.Execution, error) {
	return r.findManagedExecution(ctx, ownerUserID, sessionKey, true)
}

// FindLatestManaged 返回 session 最近一次正式 WorkGraph，包含 terminal 结果供 UI
// 回看；后续 transient Execution 或 planless runtime round 不参与替换。
func (r *Repository) FindLatestManaged(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
) (*protocol.Execution, error) {
	return r.findManagedExecution(ctx, ownerUserID, sessionKey, false)
}

// ListManaged 返回 session 的 managed Execution 历史，供用户回看并提炼 Workflow。
func (r *Repository) ListManaged(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	limit int,
) ([]protocol.Execution, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, r.executionSelect()+`
WHERE executions.owner_user_id = `+r.bind(1)+`
  AND executions.session_key = `+r.bind(2)+`
  AND EXISTS (
      SELECT 1
      FROM execution_plan_revisions plan
      JOIN execution_plan_items item
        ON item.plan_id = plan.plan_id
       AND item.execution_id = plan.execution_id
      WHERE plan.execution_id = executions.execution_id
        AND plan.status = 'active'
  )
ORDER BY executions.updated_at DESC, executions.execution_id DESC
LIMIT `+r.bind(3),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(sessionKey),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]protocol.Execution, 0, limit)
	for rows.Next() {
		item, scanErr := scanExecution(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) findManagedExecution(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	currentOnly bool,
) (*protocol.Execution, error) {
	statusPredicate := ""
	if currentOnly {
		statusPredicate = `
  AND executions.status IN ('active', 'waiting', 'paused')`
	}
	item, err := scanExecution(r.db.QueryRowContext(ctx, r.executionSelect()+`
WHERE executions.owner_user_id = `+r.bind(1)+`
  AND executions.session_key = `+r.bind(2)+statusPredicate+`
  AND EXISTS (
      SELECT 1
      FROM execution_plan_revisions plan
      JOIN execution_plan_items item
        ON item.plan_id = plan.plan_id
       AND item.execution_id = plan.execution_id
      WHERE plan.execution_id = executions.execution_id
        AND plan.status = 'active'
  )
ORDER BY executions.updated_at DESC, executions.execution_id DESC
LIMIT 1`,
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(sessionKey),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindCurrentByGoal 返回与指定 Goal objective revision 绑定的唯一未终结 Execution。
func (r *Repository) FindCurrentByGoal(
	ctx context.Context,
	goalID string,
	goalObjectiveRevision int64,
) (*protocol.Execution, error) {
	item, err := scanExecution(r.db.QueryRowContext(ctx, r.executionSelect()+`
WHERE goal_id = `+r.bind(1)+`
  AND goal_objective_revision = `+r.bind(2)+`
  AND status IN ('active', 'waiting', 'paused')
LIMIT 1`,
		strings.TrimSpace(goalID),
		goalObjectiveRevision,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByGoalRevision 返回与指定 Goal objective revision 绑定的最近 Execution，
// 包含 terminal 状态，供 Goal binding inspector 校验历史双向绑定。
func (r *Repository) FindByGoalRevision(
	ctx context.Context,
	goalID string,
	goalObjectiveRevision int64,
) (*protocol.Execution, error) {
	item, err := scanExecution(r.db.QueryRowContext(ctx, r.executionSelect()+`
WHERE goal_id = `+r.bind(1)+`
  AND goal_objective_revision = `+r.bind(2)+`
ORDER BY updated_at DESC, execution_id DESC
LIMIT 1`,
		strings.TrimSpace(goalID),
		goalObjectiveRevision,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// BindGoal 无损绑定 Goal identity，不重建 Plan 或 Work Item。
func (r *Repository) BindGoal(ctx context.Context, command BindGoalCommand) (*protocol.ExecutionSnapshot, error) {
	item := command.Execution
	item.ID = strings.TrimSpace(item.ID)
	item.GoalID = strings.TrimSpace(item.GoalID)
	if item.ID == "" || item.GoalID == "" || item.GoalObjectiveRevision <= 0 ||
		item.GoalActivationOrigin == "" || item.GoalActivationReason == "" {
		return nil, fmt.Errorf("%w: complete Goal binding is required", ErrInvariant)
	}
	mutation, err := r.beginMutation(
		ctx, item.ID, command.ExpectedExecutionVersion, command.Meta, protocol.ExecutionEventPromoted,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	// Binding an existing transient Execution must acquire the same unique
	// Goal-revision identity claim as CreateWithPlan. Otherwise two existing
	// Executions could race to bind one Goal revision through different paths.
	storedExecution, loadErr := r.getExecution(ctx, mutation.tx, item.ID)
	if loadErr != nil || storedExecution == nil {
		r.abortMutation(mutation)
		if loadErr != nil {
			return nil, loadErr
		}
		return nil, sql.ErrNoRows
	}
	claimExecution := item
	claimExecution.OwnerUserID = storedExecution.OwnerUserID
	if err = r.claimGoalExecutionMaterialization(
		ctx,
		mutation.tx,
		claimExecution,
		command.Meta.CommandID,
		timeOr(command.Meta.CreatedAt, r.currentTime()),
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE executions
SET goal_id = `+r.bind(1)+`,
    goal_objective_revision = `+r.bind(2)+`,
    goal_activation_origin = `+r.bind(3)+`,
    goal_activation_reason = `+r.bind(4)+`
WHERE execution_id = `+r.bind(5)+`
  AND version = `+r.bind(6)+`
  AND goal_id IS NULL`,
		item.GoalID, item.GoalObjectiveRevision, item.GoalActivationOrigin,
		item.GoalActivationReason, item.ID, command.ExpectedExecutionVersion+1,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	storedExecution.GoalID = item.GoalID
	storedExecution.GoalObjectiveRevision = item.GoalObjectiveRevision
	storedExecution.GoalActivationOrigin = item.GoalActivationOrigin
	storedExecution.GoalActivationReason = item.GoalActivationReason
	if err = r.ensureGoalConfirmationReceiptTx(ctx, mutation.tx, *storedExecution); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityExecution,
		EntityID:      item.ID,
		EntityVersion: command.ExpectedExecutionVersion + 1,
		GoalID:        item.GoalID,
	})
}

// Complete 在全部 CompletionBlockers 消失后完成 Execution。
func (r *Repository) Complete(ctx context.Context, command CompleteCommand) (*protocol.ExecutionSnapshot, error) {
	mutation, err := r.beginMutation(
		ctx, command.ExecutionID, command.ExpectedExecutionVersion, command.Meta, protocol.ExecutionEventCompleted,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	snapshot, err := r.getSnapshot(ctx, mutation.tx, command.ExecutionID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if snapshot == nil {
		r.abortMutation(mutation)
		return nil, sql.ErrNoRows
	}
	if len(snapshot.CompletionBlockers) > 0 {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: %s", ErrCompletionBlocked, strings.Join(snapshot.CompletionBlockers, ", "))
	}
	completedAt := timeOr(command.CompletedAt, r.currentTime())
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE executions
SET status = 'completed',
    completed_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND version = `+r.bind(3)+`
  AND status IN ('active', 'waiting')`,
		r.timestamp(completedAt),
		command.ExecutionID,
		command.ExpectedExecutionVersion+1,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = r.completeCompletionAuditTx(
		ctx,
		mutation.tx,
		command.ExecutionID,
		completedAt,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityExecution,
		EntityID:      command.ExecutionID,
		EntityVersion: command.ExpectedExecutionVersion + 1,
	})
}

func requireOne(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrVersionConflict
	}
	return nil
}
