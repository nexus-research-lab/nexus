// INPUT: coordinator-authorized transient Execution abandon/replace commands and complete initial Plan graphs.
// OUTPUT: atomic Execution+Plan creation, graph terminalization, successor linkage, and idempotent audit events.
// POS: objective-boundary lifecycle transitions that cannot be expressed as an in-Execution Plan revision.
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const successorExecutionPayloadKey = "successor_execution_id"

// CreateWithPlan atomically creates an Execution and its first active Plan.
func (r *Repository) CreateWithPlan(
	ctx context.Context,
	command CreateWithPlanCommand,
) (*protocol.ExecutionSnapshot, error) {
	if err := validateMeta(command.Meta); err != nil {
		return nil, err
	}
	if err := validateMeta(command.Plan.Meta); err != nil {
		return nil, err
	}
	now := r.currentTime()
	command.Meta.CreatedAt = timeOr(command.Meta.CreatedAt, now)
	command.Plan.Meta.CreatedAt = timeOr(command.Plan.Meta.CreatedAt, now)
	execution, criteriaJSON, metadataJSON, err := prepareExecutionInsert(command.Execution, now)
	if err != nil {
		return nil, err
	}
	if execution.ReplacesExecutionID != "" && execution.GoalID == "" {
		return nil, fmt.Errorf("%w: only a Goal revision successor may replace a predecessor during initial Plan creation", ErrInvariant)
	}
	plan, workItems, dependencies, err := prepareInitialPlan(command.Plan, execution.ID, now)
	if err != nil {
		return nil, err
	}
	if existing, findErr := r.findEventByCommand(
		ctx,
		r.db,
		execution.ID,
		command.Meta.CommandID,
	); findErr != nil {
		return nil, findErr
	} else if existing != nil {
		if existing.Type != protocol.ExecutionEventCreated {
			return nil, ErrCommandConflict
		}
		return r.GetSnapshot(ctx, execution.ID)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if execution.GoalID != "" &&
		execution.GoalObjectiveRevision > 1 &&
		execution.ReplacesExecutionID == "" {
		if err = r.validateFencedGoalRevisionSuccessor(ctx, tx, execution); err != nil {
			return nil, err
		}
	}
	if err = r.claimGoalExecutionMaterialization(
		ctx,
		tx,
		execution,
		command.Meta.CommandID,
		command.Meta.CreatedAt,
	); err != nil {
		return nil, err
	}
	if execution.ReplacesExecutionID != "" {
		if err = r.validateGoalRevisionSuccessor(ctx, tx, execution); err != nil {
			return nil, err
		}
	}
	if err = r.insertExecutionRow(ctx, tx, execution, criteriaJSON, metadataJSON); err != nil {
		return nil, err
	}
	if err = r.ensureGoalConfirmationReceiptTx(ctx, tx, execution); err != nil {
		return nil, err
	}
	if err = r.persistInitialPlan(ctx, tx, plan, workItems, dependencies); err != nil {
		return nil, err
	}
	if err = r.insertEvent(ctx, tx, executionEvent(
		command.Meta,
		execution.ID,
		protocol.ExecutionEventCreated,
		execution.ID,
		execution.Version,
		nil,
	)); err != nil {
		return nil, err
	}
	if err = r.insertEvent(ctx, tx, planEvent(command.Plan.Meta, execution.ID, plan)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSnapshot(ctx, execution.ID)
}

// ReplaceWithPlan atomically supersedes one current transient Execution and
// creates its successor with a complete first active Plan.
func (r *Repository) ReplaceWithPlan(
	ctx context.Context,
	command ReplaceWithPlanCommand,
) (*protocol.ExecutionSnapshot, error) {
	command.ExecutionID = strings.TrimSpace(command.ExecutionID)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ExecutionID == "" || command.Reason == "" {
		return nil, fmt.Errorf("%w: replacement requires current Execution and reason", ErrInvariant)
	}
	if err := validateExpectedVersion(command.ExpectedExecutionVersion, "expected execution version"); err != nil {
		return nil, err
	}
	if err := validateMeta(command.Meta); err != nil {
		return nil, err
	}
	if err := validateMeta(command.SuccessorMeta); err != nil {
		return nil, err
	}
	if err := validateMeta(command.Plan.Meta); err != nil {
		return nil, err
	}
	if replay, err := r.replayedSuccessor(ctx, r.db, command.ExecutionID, command.Meta.CommandID); err != nil {
		return nil, err
	} else if replay != "" {
		return r.GetSnapshot(ctx, replay)
	}

	now := r.currentTime()
	command.Meta.CreatedAt = timeOr(command.Meta.CreatedAt, now)
	command.SuccessorMeta.CreatedAt = timeOr(command.SuccessorMeta.CreatedAt, now)
	command.Plan.Meta.CreatedAt = timeOr(command.Plan.Meta.CreatedAt, now)
	successor, criteriaJSON, metadataJSON, err := prepareExecutionInsert(command.Successor, now)
	if err != nil {
		return nil, err
	}
	if successor.ReplacesExecutionID != command.ExecutionID {
		return nil, fmt.Errorf("%w: successor predecessor link is invalid", ErrInvariant)
	}
	plan, workItems, dependencies, err := prepareInitialPlan(command.Plan, successor.ID, now)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if replay, replayErr := r.replayedSuccessor(
		ctx,
		tx,
		command.ExecutionID,
		command.Meta.CommandID,
	); replayErr != nil {
		return nil, replayErr
	} else if replay != "" {
		_ = tx.Rollback()
		return r.GetSnapshot(ctx, replay)
	}
	current, err := r.getExecution(ctx, tx, command.ExecutionID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, sql.ErrNoRows
	}
	if current.Version != command.ExpectedExecutionVersion ||
		!currentExecutionStatus(current.Status) {
		return nil, ErrVersionConflict
	}
	if strings.TrimSpace(current.GoalID) != "" {
		return nil, fmt.Errorf("%w: Goal-bound Execution cannot use transient replacement", ErrInvariant)
	}
	if err = validateSuccessorScope(*current, successor); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE executions
SET status = 'superseded',
    version = version + 1,
    updated_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND version = `+r.bind(3)+`
  AND goal_id IS NULL
  AND status IN ('active', 'waiting', 'paused')`,
		r.timestamp(now), current.ID, command.ExpectedExecutionVersion,
	)
	if err != nil {
		return nil, err
	}
	if err = requireOne(result); err != nil {
		return nil, err
	}
	if err = r.terminalizeExecutionGraph(
		ctx,
		tx,
		current.ID,
		protocol.ExecutionStatusSuperseded,
		command.Meta.CommandID,
		command.Reason,
		now,
	); err != nil {
		return nil, err
	}
	if err = r.insertExecutionRow(ctx, tx, successor, criteriaJSON, metadataJSON); err != nil {
		return nil, err
	}
	if err = r.ensureGoalConfirmationReceiptTx(ctx, tx, successor); err != nil {
		return nil, err
	}
	if err = r.persistInitialPlan(ctx, tx, plan, workItems, dependencies); err != nil {
		return nil, err
	}
	oldPayload := map[string]any{
		"reason":                     command.Reason,
		successorExecutionPayloadKey: successor.ID,
	}
	if err = r.insertEvent(ctx, tx, executionEvent(
		command.Meta,
		current.ID,
		protocol.ExecutionEventSuperseded,
		current.ID,
		command.ExpectedExecutionVersion+1,
		oldPayload,
	)); err != nil {
		return nil, err
	}
	newPayload := map[string]any{
		"reason":                command.Reason,
		"replaces_execution_id": current.ID,
	}
	if err = r.insertEvent(ctx, tx, executionEvent(
		command.SuccessorMeta,
		successor.ID,
		protocol.ExecutionEventCreated,
		successor.ID,
		successor.Version,
		newPayload,
	)); err != nil {
		return nil, err
	}
	if err = r.insertEvent(ctx, tx, planEvent(command.Plan.Meta, successor.ID, plan)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSnapshot(ctx, successor.ID)
}

// Abandon atomically cancels one current transient Execution without creating a successor.
func (r *Repository) Abandon(
	ctx context.Context,
	command AbandonCommand,
) (*protocol.ExecutionSnapshot, error) {
	command.ExecutionID = strings.TrimSpace(command.ExecutionID)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ExecutionID == "" || command.Reason == "" {
		return nil, fmt.Errorf("%w: abandon requires current Execution and reason", ErrInvariant)
	}
	command.Meta.Payload = mergeStateEventPayload(command.Meta.Payload, map[string]any{
		"reason": command.Reason,
	})
	mutation, err := r.beginMutation(
		ctx,
		command.ExecutionID,
		command.ExpectedExecutionVersion,
		command.Meta,
		protocol.ExecutionEventCancelled,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	current, err := r.getExecution(ctx, mutation.tx, command.ExecutionID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if current == nil {
		r.abortMutation(mutation)
		return nil, sql.ErrNoRows
	}
	if strings.TrimSpace(current.GoalID) != "" {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Goal-bound Execution cannot be abandoned", ErrInvariant)
	}
	now := r.currentTime()
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE executions
SET status = 'cancelled'
WHERE execution_id = `+r.bind(1)+`
  AND version = `+r.bind(2)+`
  AND goal_id IS NULL
  AND status IN ('active', 'waiting', 'paused')`,
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
	if err = r.terminalizeExecutionGraph(
		ctx,
		mutation.tx,
		command.ExecutionID,
		protocol.ExecutionStatusCancelled,
		command.Meta.CommandID,
		command.Reason,
		now,
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

func prepareExecutionInsert(
	item protocol.Execution,
	now time.Time,
) (protocol.Execution, string, string, error) {
	item.ID = strings.TrimSpace(item.ID)
	item.OwnerUserID = strings.TrimSpace(item.OwnerUserID)
	item.SessionKey = strings.TrimSpace(item.SessionKey)
	item.RoomID = strings.TrimSpace(item.RoomID)
	item.ConversationID = strings.TrimSpace(item.ConversationID)
	item.CoordinatorAgentID = strings.TrimSpace(item.CoordinatorAgentID)
	item.Objective = strings.TrimSpace(item.Objective)
	item.GoalID = strings.TrimSpace(item.GoalID)
	item.RecoveryOfExecutionID = strings.TrimSpace(item.RecoveryOfExecutionID)
	item.ReplacesExecutionID = strings.TrimSpace(item.ReplacesExecutionID)
	item.RootRoundID = strings.TrimSpace(item.RootRoundID)
	item.TriggerMessageID = strings.TrimSpace(item.TriggerMessageID)
	if err := protocol.ValidateExecutionProjectionLimit(
		"completion_criteria",
		len(item.CompletionCriteria),
	); err != nil {
		return protocol.Execution{}, "", "", err
	}
	criteria := make([]string, 0, len(item.CompletionCriteria))
	for _, criterion := range item.CompletionCriteria {
		if criterion = strings.TrimSpace(criterion); criterion != "" {
			criteria = append(criteria, criterion)
		}
	}
	item.CompletionCriteria = criteria
	if item.ID == "" || item.OwnerUserID == "" || item.SessionKey == "" ||
		item.CoordinatorAgentID == "" || item.Objective == "" || len(item.CompletionCriteria) == 0 {
		return protocol.Execution{}, "", "", fmt.Errorf(
			"%w: execution identity, owner, session, coordinator, objective and completion criteria are required",
			ErrInvariant,
		)
	}
	if item.ID == item.ReplacesExecutionID {
		return protocol.Execution{}, "", "", fmt.Errorf("%w: Execution cannot replace itself", ErrInvariant)
	}
	switch item.ScopeKind {
	case protocol.ExecutionScopeDM:
		if item.RoomID != "" || item.ConversationID != "" {
			return protocol.Execution{}, "", "", fmt.Errorf("%w: DM execution cannot carry Room identity", ErrInvariant)
		}
	case protocol.ExecutionScopeRoom:
		if item.RoomID == "" || item.ConversationID == "" {
			return protocol.Execution{}, "", "", fmt.Errorf("%w: Room execution requires room and conversation identity", ErrInvariant)
		}
	default:
		return protocol.Execution{}, "", "", fmt.Errorf("%w: execution scope %q is invalid", ErrInvariant, item.ScopeKind)
	}
	if item.Status == "" {
		item.Status = protocol.ExecutionStatusActive
	}
	if item.Status != protocol.ExecutionStatusActive {
		return protocol.Execution{}, "", "", fmt.Errorf("%w: new Execution must be active", ErrInvariant)
	}
	if item.Version != 0 && item.Version != 1 {
		return protocol.Execution{}, "", "", fmt.Errorf("%w: new execution version must be 1", ErrInvariant)
	}
	item.Version = 1
	item.CreatedAt = timeOr(item.CreatedAt, now)
	item.UpdatedAt = timeOr(item.UpdatedAt, item.CreatedAt)
	criteriaJSON, err := marshalSlice(item.CompletionCriteria)
	if err != nil {
		return protocol.Execution{}, "", "", err
	}
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return protocol.Execution{}, "", "", err
	}
	return item, criteriaJSON, metadataJSON, nil
}

func (r *Repository) insertExecutionRow(
	ctx context.Context,
	tx *sql.Tx,
	item protocol.Execution,
	criteriaJSON string,
	metadataJSON string,
) error {
	_, err := tx.ExecContext(ctx, `
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
		nullString(item.RoomID), nullString(item.ConversationID), item.CoordinatorAgentID,
		item.Origin, item.Objective, criteriaJSON, nullString(item.GoalID),
		item.GoalObjectiveRevision, nullString(string(item.GoalActivationOrigin)),
		nullString(string(item.GoalActivationReason)), nullString(item.RecoveryOfExecutionID),
		nullString(item.ReplacesExecutionID), nullString(item.RootRoundID),
		nullString(item.TriggerMessageID), item.Status, item.Version,
		r.timestamp(item.CreatedAt), r.timestamp(item.UpdatedAt), nullTime(item.CompletedAt), metadataJSON,
	)
	return err
}

func prepareInitialPlan(
	command WritePlanCommand,
	executionID string,
	now time.Time,
) (
	protocol.ExecutionPlanRevision,
	[]PlanWorkItem,
	[]protocol.ExecutionPlanDependency,
	error,
) {
	plan := command.Plan
	plan.ID = strings.TrimSpace(plan.ID)
	plan.ExecutionID = strings.TrimSpace(plan.ExecutionID)
	if plan.ID == "" || plan.ExecutionID != executionID || plan.Revision != 1 ||
		plan.Status != protocol.PlanRevisionStatusActive || strings.TrimSpace(plan.BasePlanID) != "" ||
		command.ExpectedPlanVersion != 0 || command.SupersedeActiveWork {
		return protocol.ExecutionPlanRevision{}, nil, nil, fmt.Errorf(
			"%w: first Plan must be a new active revision 1 without a base",
			ErrInvariant,
		)
	}
	normalized, dependencies, err := normalizeAndValidatePlan(
		plan,
		command.WorkItems,
		command.Dependencies,
		now,
	)
	if err != nil {
		return protocol.ExecutionPlanRevision{}, nil, nil, err
	}
	plan.Version = 1
	plan.CreatedAt = timeOr(plan.CreatedAt, now)
	activatedAt := now
	plan.ActivatedAt = &activatedAt
	return plan, normalized, dependencies, nil
}

func (r *Repository) persistInitialPlan(
	ctx context.Context,
	tx *sql.Tx,
	plan protocol.ExecutionPlanRevision,
	workItems []PlanWorkItem,
	dependencies []protocol.ExecutionPlanDependency,
) error {
	if err := r.insertPlan(ctx, tx, plan); err != nil {
		return err
	}
	for _, work := range workItems {
		if err := r.ensureWorkItem(ctx, tx, work.WorkItem); err != nil {
			return err
		}
		if err := r.ensureSpec(ctx, tx, work.Spec); err != nil {
			return err
		}
	}
	for _, work := range parentFirst(workItems) {
		if err := r.insertPlanItem(ctx, tx, work.Item); err != nil {
			return err
		}
	}
	for _, dependency := range dependencies {
		if err := r.insertDependency(ctx, tx, dependency); err != nil {
			return err
		}
	}
	for _, work := range workItems {
		for _, claim := range work.OutputClaims {
			if err := r.insertClaim(ctx, tx, claim); err != nil {
				return err
			}
		}
	}
	for _, work := range workItems {
		if err := r.ensureState(ctx, tx, work, true); err != nil {
			return err
		}
	}
	return nil
}

func validateSuccessorScope(current protocol.Execution, successor protocol.Execution) error {
	if successor.ID == current.ID ||
		successor.OwnerUserID != current.OwnerUserID ||
		successor.SessionKey != current.SessionKey ||
		successor.ScopeKind != current.ScopeKind ||
		successor.RoomID != current.RoomID ||
		successor.ConversationID != current.ConversationID ||
		successor.CoordinatorAgentID != current.CoordinatorAgentID ||
		successor.GoalID != "" ||
		successor.GoalObjectiveRevision != 0 ||
		successor.GoalActivationOrigin != "" ||
		successor.GoalActivationReason != "" {
		return fmt.Errorf("%w: successor must preserve transient execution scope and coordinator", ErrInvariant)
	}
	return nil
}

func (r *Repository) terminalizeExecutionGraph(
	ctx context.Context,
	tx *sql.Tx,
	executionID string,
	executionStatus protocol.ExecutionStatus,
	commandID string,
	reason string,
	now time.Time,
) error {
	planStatus := protocol.PlanRevisionStatusCancelled
	workStatus := protocol.WorkItemStatusCancelled
	attemptStatus := protocol.WorkAttemptStatusCancelled
	if executionStatus == protocol.ExecutionStatusSuperseded {
		planStatus = protocol.PlanRevisionStatusSuperseded
		workStatus = protocol.WorkItemStatusSuperseded
		attemptStatus = protocol.WorkAttemptStatusInterrupted
	}
	terminalReason := "execution " + string(executionStatus) + ": " + strings.TrimSpace(reason)
	activePlanUpdate := `
UPDATE execution_plan_revisions
SET status = ` + r.bind(1) + `,
    version = version + 1`
	activePlanArgs := []any{planStatus}
	if planStatus == protocol.PlanRevisionStatusSuperseded {
		activePlanUpdate += `,
    superseded_at = ` + r.bind(2)
		activePlanArgs = append(activePlanArgs, r.timestamp(now))
	}
	activePlanUpdate += `
WHERE execution_id = ` + r.bind(len(activePlanArgs)+1) + `
  AND status = 'active'`
	activePlanArgs = append(activePlanArgs, executionID)
	if _, err := tx.ExecContext(ctx, activePlanUpdate, activePlanArgs...); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE execution_plan_revisions
SET status = 'cancelled',
    version = version + 1
WHERE execution_id = `+r.bind(1)+`
  AND status = 'proposed'`,
		executionID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE execution_work_item_states
SET status = `+r.bind(1)+`,
    block_reason = NULL,
    needed_input = NULL,
    version = version + 1,
    updated_at = `+r.bind(2)+`
WHERE execution_id = `+r.bind(3)+`
  AND status IN ('open', 'waiting_input')`,
		workStatus, r.timestamp(now), executionID,
	); err != nil {
		return err
	}
	if err := r.enqueueAttemptCancellations(
		ctx,
		tx,
		cancellationAttemptScope{ExecutionID: executionID},
		commandID,
		terminalReason,
		now,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE execution_attempts
SET status = `+r.bind(1)+`,
    failure_reason = `+r.bind(2)+`,
    version = version + 1,
    finished_at = `+r.bind(3)+`
WHERE execution_id = `+r.bind(4)+`
  AND status IN ('pending', 'running')`,
		attemptStatus, terminalReason, r.timestamp(now), executionID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE execution_dispatches
SET status = 'cancelled',
    last_error = `+r.bind(1)+`,
    version = version + 1,
    updated_at = `+r.bind(2)+`,
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE execution_id = `+r.bind(3)+`
  AND status IN ('pending', 'claimed', 'failed')`,
		terminalReason, r.timestamp(now), executionID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE execution_review_dispatches
SET status = 'cancelled',
    last_error = `+r.bind(1)+`,
    version = version + 1,
    updated_at = `+r.bind(2)+`,
    lease_owner = NULL,
    lease_expires_at = NULL
WHERE execution_id = `+r.bind(3)+`
  AND status IN ('pending', 'claimed', 'failed')`,
		terminalReason, r.timestamp(now), executionID,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
UPDATE execution_work_assignments
SET status = 'released',
    version = version + 1,
    released_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND status IN ('assigned', 'active')`,
		r.timestamp(now), executionID,
	)
	return err
}

func (r *Repository) replayedSuccessor(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
	commandID string,
) (string, error) {
	event, err := r.findEventByCommand(ctx, queryer, executionID, commandID)
	if err != nil || event == nil {
		return "", err
	}
	if event.Type != protocol.ExecutionEventSuperseded {
		return "", ErrCommandConflict
	}
	successor, _ := event.Payload[successorExecutionPayloadKey].(string)
	successor = strings.TrimSpace(successor)
	if successor == "" {
		return "", fmt.Errorf("%w: replacement event has no successor", ErrInvariant)
	}
	return successor, nil
}

func executionEvent(
	meta CommandMeta,
	executionID string,
	eventType protocol.ExecutionEventType,
	entityID string,
	entityVersion int64,
	payload map[string]any,
) protocol.ExecutionEvent {
	return protocol.ExecutionEvent{
		ID:             meta.EventID,
		ExecutionID:    executionID,
		Type:           eventType,
		EntityType:     protocol.ExecutionEntityExecution,
		EntityID:       entityID,
		EntityVersion:  entityVersion,
		ActorKind:      meta.ActorKind,
		ActorID:        meta.ActorID,
		CommandID:      meta.CommandID,
		RootRoundID:    meta.RootRoundID,
		RuntimeRoundID: meta.RuntimeRoundID,
		AgentRoundID:   meta.AgentRoundID,
		Payload:        payload,
		CreatedAt:      meta.CreatedAt,
	}
}

func planEvent(
	meta CommandMeta,
	executionID string,
	plan protocol.ExecutionPlanRevision,
) protocol.ExecutionEvent {
	return protocol.ExecutionEvent{
		ID:             meta.EventID,
		ExecutionID:    executionID,
		Type:           protocol.ExecutionEventPlanActivated,
		EntityType:     protocol.ExecutionEntityPlan,
		EntityID:       plan.ID,
		EntityVersion:  plan.Version,
		ActorKind:      meta.ActorKind,
		ActorID:        meta.ActorID,
		CommandID:      meta.CommandID,
		PlanID:         plan.ID,
		RootRoundID:    meta.RootRoundID,
		RuntimeRoundID: meta.RuntimeRoundID,
		AgentRoundID:   meta.AgentRoundID,
		Payload:        meta.Payload,
		CreatedAt:      meta.CreatedAt,
	}
}

func currentExecutionStatus(status protocol.ExecutionStatus) bool {
	return status == protocol.ExecutionStatusActive ||
		status == protocol.ExecutionStatusWaiting ||
		status == protocol.ExecutionStatusPaused
}
