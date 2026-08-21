// INPUT: canonical sealed ExecutionPlanProposal and exact binding/access/version CAS commands.
// OUTPUT: durable proposal identity plus active binding, materialization receipt, confirmation progress and replay-safe state.
// POS: non-authoritative proposal aggregate's SQL mutation boundary; authoritative Execution/Plan writes stay elsewhere.
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateOrGetPlanProposal inserts one immutable sealed proposal or returns an exact
// deterministic-ID replay. The document is persisted only as canonical typed JSON.
func (r *Repository) CreateOrGetPlanProposal(
	ctx context.Context,
	command CreateOrGetPlanProposalCommand,
) (*protocol.ExecutionPlanProposal, error) {
	item, documentJSON, err := normalizeAndValidatePlanProposal(command.Proposal, r.currentTime())
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
INSERT INTO execution_plan_proposals (
    proposal_id, owner_user_id, session_key, scope_kind,
    room_id, conversation_id, coordinator_agent_id,
    root_round_id, runtime_round_id, agent_round_id,
    target_execution_id, target_execution_version, base_plan_id,
    goal_id, goal_objective_revision, goal_activation_origin,
    goal_activation_reason, goal_reserved_execution_id, replaces_execution_id,
    document_json, content_digest, status, version,
    reserved_execution_id, materialization_command_id,
    materialized_execution_id, materialized_plan_id,
    confirmation_state, attempt_count, next_attempt_at, last_error,
    created_at, updated_at, materialized_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+
		r.bind(5)+`,`+r.bind(6)+`,`+r.bind(7)+`,`+
		r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.bind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+
		r.bind(14)+`,`+r.bind(15)+`,`+r.bind(16)+`,`+
		r.bind(17)+`,`+r.bind(18)+`,`+r.bind(19)+`,`+
		r.jsonBind(20)+`,`+r.bind(21)+`,`+r.bind(22)+`,`+r.bind(23)+`,`+
		r.bind(24)+`,`+r.bind(25)+`,`+
		r.bind(26)+`,`+r.bind(27)+`,`+
		r.bind(28)+`,`+r.bind(29)+`,`+r.bind(30)+`,`+r.bind(31)+`,`+
		r.bind(32)+`,`+r.bind(33)+`,`+r.bind(34)+`)
ON CONFLICT (proposal_id) DO NOTHING`,
		item.ID, item.OwnerUserID, item.SessionKey, item.ScopeKind,
		nullString(item.RoomID), nullString(item.ConversationID), item.CoordinatorAgentID,
		nullString(item.RootRoundID), nullString(item.RuntimeRoundID), nullString(item.AgentRoundID),
		nullString(item.TargetExecutionID), item.TargetExecutionVersion, nullString(item.BasePlanID),
		nullString(item.GoalID), item.GoalObjectiveRevision, nullString(string(item.GoalActivationOrigin)),
		nullString(string(item.GoalActivationReason)), nullString(item.GoalReservedExecutionID),
		nullString(item.ReplacesExecutionID),
		documentJSON, item.ContentDigest, item.Status, item.Version,
		nullString(item.ReservedExecutionID), nullString(item.MaterializationCommandID),
		nullString(item.MaterializedExecutionID), nullString(item.MaterializedPlanID),
		item.ConfirmationState, item.AttemptCount, r.planProposalTimestamp(item.NextAttemptAt), nullString(item.LastError),
		r.timestamp(item.CreatedAt), r.timestamp(item.UpdatedAt), r.planProposalTimestamp(item.MaterializedAt),
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	existing, err := r.getPlanProposal(ctx, tx, item.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("%w: sealed proposal disappeared after create", ErrInvariant)
	}
	if !planProposalCreateReplayMatches(*existing, item) {
		return nil, fmt.Errorf(
			"%w: deterministic proposal id %q identifies another immutable proposal",
			ErrCommandConflict,
			item.ID,
		)
	}
	if err = r.bindPreparedPlanProposal(ctx, tx, *existing, affected == 1); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return existing, nil
}

// MarkPlanProposalMaterializing reserves the exact authoritative command/Execution
// identities before any Execution Orchestration mutation is attempted.
func (r *Repository) MarkPlanProposalMaterializing(
	ctx context.Context,
	command MarkPlanProposalMaterializingCommand,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalAccess(command.Access)
	if err != nil {
		return nil, err
	}
	if err = validateExpectedVersion(command.ExpectedVersion, "expected proposal version"); err != nil {
		return nil, err
	}
	command.ReservedExecutionID = strings.TrimSpace(command.ReservedExecutionID)
	command.MaterializationCommandID = strings.TrimSpace(command.MaterializationCommandID)
	command.GoalID = strings.TrimSpace(command.GoalID)
	command.ReplacesExecutionID = strings.TrimSpace(command.ReplacesExecutionID)
	command.NextAttemptAt = utcTimePointer(command.NextAttemptAt)
	if command.ReservedExecutionID == "" || command.MaterializationCommandID == "" {
		return nil, fmt.Errorf("%w: materialization reservation and command identities are required", ErrInvariant)
	}
	if err = validatePlanProposalStringWidth("reserved execution id", command.ReservedExecutionID, 64); err != nil {
		return nil, err
	}
	if err = validatePlanProposalStringWidth("materialization command id", command.MaterializationCommandID, 128); err != nil {
		return nil, err
	}
	if err = validatePlanProposalStringWidth("Goal id", command.GoalID, 64); err != nil {
		return nil, err
	}
	if err = validatePlanProposalStringWidth("replaces execution id", command.ReplacesExecutionID, 64); err != nil {
		return nil, err
	}
	current, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if planProposalMaterializingReplayMatches(*current, command) {
		return nil, ErrPlanProposalNotDue
	}
	now := r.currentTime()
	if command.NextAttemptAt == nil || !command.NextAttemptAt.After(now) {
		return nil, fmt.Errorf(
			"%w: materialization reservation requires a future recovery deadline",
			ErrInvariant,
		)
	}
	if current.Version != command.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	if current.Status != protocol.ExecutionPlanProposalStatusSealed {
		return nil, fmt.Errorf("%w: only a sealed proposal can begin materialization", ErrInvariant)
	}
	if err = validateProposalMaterializationReservation(*current, command); err != nil {
		return nil, err
	}
	if used, findErr := r.getPlanProposalByMaterializationCommand(
		ctx,
		r.db,
		command.MaterializationCommandID,
	); findErr != nil {
		return nil, findErr
	} else if used != nil && used.ID != current.ID {
		return nil, fmt.Errorf("%w: materialization command is already reserved", ErrCommandConflict)
	}
	confirmation := protocol.ExecutionPlanProposalConfirmationNone
	if current.GoalID != "" {
		confirmation = protocol.ExecutionPlanProposalConfirmationPending
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_plan_proposals
SET status = 'materializing',
    reserved_execution_id = `+r.bind(1)+`,
    materialization_command_id = `+r.bind(2)+`,
    goal_activation_origin = `+r.bind(3)+`,
    goal_activation_reason = `+r.bind(4)+`,
    replaces_execution_id = `+r.bind(5)+`,
    confirmation_state = `+r.bind(6)+`,
    attempt_count = attempt_count + 1,
    next_attempt_at = `+r.bind(7)+`,
    last_error = NULL,
    version = version + 1,
    updated_at = `+r.bind(8)+`
WHERE proposal_id = `+r.bind(9)+`
  AND owner_user_id = `+r.bind(10)+`
  AND session_key = `+r.bind(11)+`
  AND scope_kind = `+r.bind(12)+`
  AND COALESCE(room_id, '') = `+r.bind(13)+`
  AND COALESCE(conversation_id, '') = `+r.bind(14)+`
  AND coordinator_agent_id = `+r.bind(15)+`
  AND version = `+r.bind(16)+`
  AND status = 'sealed'`,
		command.ReservedExecutionID,
		command.MaterializationCommandID,
		nullString(string(command.GoalActivationOrigin)),
		nullString(string(command.GoalActivationReason)),
		nullString(command.ReplacesExecutionID),
		confirmation,
		r.planProposalTimestamp(command.NextAttemptAt),
		r.timestamp(now),
		access.ProposalID,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		command.ExpectedVersion,
	)
	if err != nil {
		if used, findErr := r.getPlanProposalByMaterializationCommand(
			ctx,
			r.db,
			command.MaterializationCommandID,
		); findErr == nil && used != nil && used.ID != access.ProposalID {
			return nil, fmt.Errorf("%w: materialization command is already reserved", ErrCommandConflict)
		}
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	updated, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if affected == 1 {
		return updated, nil
	}
	if planProposalMaterializingReplayMatches(*updated, command) {
		return nil, ErrPlanProposalNotDue
	}
	return nil, ErrVersionConflict
}

// ClaimPlanProposalMaterializing acquires one bounded execution lease after a
// crashed/expired attempt. The initial sealed->materializing transition already
// owns its first lease and does not call this method.
func (r *Repository) ClaimPlanProposalMaterializing(
	ctx context.Context,
	command ClaimPlanProposalMaterializingCommand,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalAccess(command.Access)
	if err != nil {
		return nil, err
	}
	if err = validateExpectedVersion(command.ExpectedVersion, "expected proposal version"); err != nil {
		return nil, err
	}
	command.ClaimAt = command.ClaimAt.UTC()
	command.LeaseUntil = command.LeaseUntil.UTC()
	if command.ClaimAt.IsZero() || !command.LeaseUntil.After(command.ClaimAt) {
		return nil, fmt.Errorf("%w: a future materialization lease is required", ErrInvariant)
	}
	current, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if current.Version != command.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	if current.Status != protocol.ExecutionPlanProposalStatusMaterializing {
		return nil, fmt.Errorf("%w: only a materializing proposal can be claimed", ErrInvariant)
	}
	if current.NextAttemptAt != nil && current.NextAttemptAt.After(command.ClaimAt) {
		return nil, ErrPlanProposalNotDue
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_plan_proposals
SET attempt_count = attempt_count + 1,
    next_attempt_at = `+r.bind(1)+`,
    last_error = NULL,
    version = version + 1,
    updated_at = `+r.bind(2)+`
WHERE proposal_id = `+r.bind(3)+`
  AND owner_user_id = `+r.bind(4)+`
  AND session_key = `+r.bind(5)+`
  AND scope_kind = `+r.bind(6)+`
  AND COALESCE(room_id, '') = `+r.bind(7)+`
  AND COALESCE(conversation_id, '') = `+r.bind(8)+`
  AND coordinator_agent_id = `+r.bind(9)+`
  AND version = `+r.bind(10)+`
  AND status = 'materializing'
  AND (next_attempt_at IS NULL OR next_attempt_at <= `+r.bind(11)+`)`,
		r.timestamp(command.LeaseUntil),
		r.timestamp(now),
		access.ProposalID,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		command.ExpectedVersion,
		r.timestamp(command.ClaimAt),
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	updated, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if affected == 1 {
		return updated, nil
	}
	if updated.Status == protocol.ExecutionPlanProposalStatusMaterializing &&
		updated.NextAttemptAt != nil && updated.NextAttemptAt.After(command.ClaimAt) {
		return nil, ErrPlanProposalNotDue
	}
	return nil, ErrVersionConflict
}

// MarkPlanProposalMaterialized records the exact authoritative Execution/Plan receipt.
func (r *Repository) MarkPlanProposalMaterialized(
	ctx context.Context,
	command MarkPlanProposalMaterializedCommand,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalAccess(command.Access)
	if err != nil {
		return nil, err
	}
	if err = validateExpectedVersion(command.ExpectedVersion, "expected proposal version"); err != nil {
		return nil, err
	}
	command.MaterializedExecutionID = strings.TrimSpace(command.MaterializedExecutionID)
	command.MaterializedPlanID = strings.TrimSpace(command.MaterializedPlanID)
	command.NextAttemptAt = utcTimePointer(command.NextAttemptAt)
	if command.MaterializedExecutionID == "" || command.MaterializedPlanID == "" {
		return nil, fmt.Errorf("%w: complete materialization receipt is required", ErrInvariant)
	}
	if err = validatePlanProposalStringWidth("materialized execution id", command.MaterializedExecutionID, 64); err != nil {
		return nil, err
	}
	if err = validatePlanProposalStringWidth("materialized plan id", command.MaterializedPlanID, 64); err != nil {
		return nil, err
	}
	current, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if planProposalMaterializedReplayMatches(*current, command) {
		return current, nil
	}
	if current.Version != command.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	if current.Status != protocol.ExecutionPlanProposalStatusMaterializing &&
		current.Status != protocol.ExecutionPlanProposalStatusBlocked {
		return nil, fmt.Errorf("%w: only a materializing or receipt-proven blocked proposal can record a receipt", ErrInvariant)
	}
	if command.MaterializedExecutionID != current.ReservedExecutionID {
		return nil, fmt.Errorf("%w: materialized Execution differs from its reservation", ErrInvariant)
	}
	if current.Status == protocol.ExecutionPlanProposalStatusBlocked {
		receiptPlanID, receiptErr := r.FindPlanMaterializationReceipt(
			ctx,
			current.ReservedExecutionID,
			current.MaterializationCommandID,
		)
		if receiptErr != nil {
			return nil, receiptErr
		}
		if strings.TrimSpace(receiptPlanID) == "" ||
			strings.TrimSpace(receiptPlanID) != command.MaterializedPlanID {
			return nil, fmt.Errorf(
				"%w: blocked proposal can only converge through its exact authoritative command receipt",
				ErrInvariant,
			)
		}
	}
	if current.ConfirmationState == protocol.ExecutionPlanProposalConfirmationNone && command.NextAttemptAt != nil {
		return nil, fmt.Errorf("%w: Goal-free receipt cannot schedule confirmation", ErrInvariant)
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_plan_proposals
SET status = 'materialized',
    materialized_execution_id = `+r.bind(1)+`,
    materialized_plan_id = `+r.bind(2)+`,
    materialized_at = `+r.bind(3)+`,
    next_attempt_at = `+r.bind(4)+`,
    last_error = NULL,
    version = version + 1,
    updated_at = `+r.bind(5)+`
WHERE proposal_id = `+r.bind(6)+`
  AND owner_user_id = `+r.bind(7)+`
  AND session_key = `+r.bind(8)+`
  AND scope_kind = `+r.bind(9)+`
  AND COALESCE(room_id, '') = `+r.bind(10)+`
  AND COALESCE(conversation_id, '') = `+r.bind(11)+`
  AND coordinator_agent_id = `+r.bind(12)+`
  AND version = `+r.bind(13)+`
  AND status IN ('materializing', 'blocked')`,
		command.MaterializedExecutionID,
		command.MaterializedPlanID,
		r.timestamp(now),
		r.planProposalTimestamp(command.NextAttemptAt),
		r.timestamp(now),
		access.ProposalID,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		command.ExpectedVersion,
	)
	if err != nil {
		return nil, err
	}
	return r.finishPlanProposalCAS(ctx, access, result, func(item protocol.ExecutionPlanProposal) bool {
		return planProposalMaterializedReplayMatches(item, command)
	})
}

// SchedulePlanProposalRetry durably applies backoff after a transient authoritative
// materialization failure without losing the stable command/Execution reservation.
func (r *Repository) SchedulePlanProposalRetry(
	ctx context.Context,
	command SchedulePlanProposalRetryCommand,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalAccess(command.Access)
	if err != nil {
		return nil, err
	}
	if err = validateExpectedVersion(command.ExpectedVersion, "expected proposal version"); err != nil {
		return nil, err
	}
	command.LastError = strings.TrimSpace(command.LastError)
	command.NextAttemptAt = utcTimePointer(command.NextAttemptAt)
	if command.LastError == "" || command.NextAttemptAt == nil {
		return nil, fmt.Errorf(
			"%w: materialization retry requires an error and a future retry time",
			ErrInvariant,
		)
	}
	current, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if planProposalRetryReplayMatches(*current, command) {
		return current, nil
	}
	now := r.currentTime()
	if !command.NextAttemptAt.After(now) {
		return nil, fmt.Errorf(
			"%w: materialization retry requires an error and a future retry time",
			ErrInvariant,
		)
	}
	if current.Version != command.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	if current.Status != protocol.ExecutionPlanProposalStatusMaterializing {
		return nil, fmt.Errorf("%w: only a materializing proposal can schedule materialization retry", ErrInvariant)
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_plan_proposals
SET attempt_count = attempt_count + 1,
    next_attempt_at = `+r.bind(1)+`,
    last_error = `+r.bind(2)+`,
    version = version + 1,
    updated_at = `+r.bind(3)+`
WHERE proposal_id = `+r.bind(4)+`
  AND owner_user_id = `+r.bind(5)+`
  AND session_key = `+r.bind(6)+`
  AND scope_kind = `+r.bind(7)+`
  AND COALESCE(room_id, '') = `+r.bind(8)+`
  AND COALESCE(conversation_id, '') = `+r.bind(9)+`
  AND coordinator_agent_id = `+r.bind(10)+`
  AND version = `+r.bind(11)+`
  AND status = 'materializing'`,
		r.planProposalTimestamp(command.NextAttemptAt),
		command.LastError,
		r.timestamp(now),
		access.ProposalID,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		command.ExpectedVersion,
	)
	if err != nil {
		return nil, err
	}
	return r.finishPlanProposalCAS(ctx, access, result, func(item protocol.ExecutionPlanProposal) bool {
		return planProposalRetryReplayMatches(item, command)
	})
}

// MarkPlanProposalConfirmation records one exact Goal confirmation retry or success.
func (r *Repository) MarkPlanProposalConfirmation(
	ctx context.Context,
	command MarkPlanProposalConfirmationCommand,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalAccess(command.Access)
	if err != nil {
		return nil, err
	}
	if err = validateExpectedVersion(command.ExpectedVersion, "expected proposal version"); err != nil {
		return nil, err
	}
	command.LastError = strings.TrimSpace(command.LastError)
	command.NextAttemptAt = utcTimePointer(command.NextAttemptAt)
	switch command.ConfirmationState {
	case protocol.ExecutionPlanProposalConfirmationPending:
		if command.LastError == "" || command.NextAttemptAt == nil {
			return nil, fmt.Errorf("%w: pending confirmation requires error and retry time", ErrInvariant)
		}
	case protocol.ExecutionPlanProposalConfirmationConfirmed:
		if command.LastError != "" || command.NextAttemptAt != nil {
			return nil, fmt.Errorf("%w: confirmed proposal cannot retain retry state", ErrInvariant)
		}
	default:
		return nil, fmt.Errorf("%w: confirmation state must be pending or confirmed", ErrInvariant)
	}
	current, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if planProposalConfirmationReplayMatches(*current, command) {
		return current, nil
	}
	now := r.currentTime()
	if command.ConfirmationState == protocol.ExecutionPlanProposalConfirmationPending &&
		!command.NextAttemptAt.After(now) {
		return nil, fmt.Errorf("%w: pending confirmation requires a future retry time", ErrInvariant)
	}
	if current.Version != command.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	if current.Status != protocol.ExecutionPlanProposalStatusMaterialized || current.GoalID == "" ||
		current.ConfirmationState != protocol.ExecutionPlanProposalConfirmationPending {
		return nil, fmt.Errorf("%w: proposal has no pending materialized Goal confirmation", ErrInvariant)
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_plan_proposals
SET confirmation_state = `+r.bind(1)+`,
    attempt_count = attempt_count + 1,
    next_attempt_at = `+r.bind(2)+`,
    last_error = `+r.bind(3)+`,
    version = version + 1,
    updated_at = `+r.bind(4)+`
WHERE proposal_id = `+r.bind(5)+`
  AND owner_user_id = `+r.bind(6)+`
  AND session_key = `+r.bind(7)+`
  AND scope_kind = `+r.bind(8)+`
  AND COALESCE(room_id, '') = `+r.bind(9)+`
  AND COALESCE(conversation_id, '') = `+r.bind(10)+`
  AND coordinator_agent_id = `+r.bind(11)+`
  AND version = `+r.bind(12)+`
  AND status = 'materialized'
  AND confirmation_state = 'pending'`,
		command.ConfirmationState,
		r.planProposalTimestamp(command.NextAttemptAt),
		nullString(command.LastError),
		r.timestamp(now),
		access.ProposalID,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		command.ExpectedVersion,
	)
	if err != nil {
		return nil, err
	}
	return r.finishPlanProposalCAS(ctx, access, result, func(item protocol.ExecutionPlanProposal) bool {
		return planProposalConfirmationReplayMatches(item, command)
	})
}

// MarkPlanProposalBlocked makes a deterministic pre-receipt failure durable and visible.
func (r *Repository) MarkPlanProposalBlocked(
	ctx context.Context,
	command MarkPlanProposalBlockedCommand,
) (*protocol.ExecutionPlanProposal, error) {
	access, err := normalizePlanProposalAccess(command.Access)
	if err != nil {
		return nil, err
	}
	if err = validateExpectedVersion(command.ExpectedVersion, "expected proposal version"); err != nil {
		return nil, err
	}
	command.LastError = strings.TrimSpace(command.LastError)
	if command.LastError == "" {
		return nil, fmt.Errorf("%w: blocked proposal requires a durable error", ErrInvariant)
	}
	current, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if current.Status == protocol.ExecutionPlanProposalStatusBlocked && current.LastError == command.LastError {
		return current, nil
	}
	if current.Version != command.ExpectedVersion {
		return nil, ErrVersionConflict
	}
	if current.Status != protocol.ExecutionPlanProposalStatusSealed &&
		current.Status != protocol.ExecutionPlanProposalStatusMaterializing {
		return nil, fmt.Errorf("%w: materialized proposal cannot be blocked", ErrInvariant)
	}
	now := r.currentTime()
	result, err := r.db.ExecContext(ctx, `
UPDATE execution_plan_proposals
SET status = 'blocked',
    next_attempt_at = NULL,
    last_error = `+r.bind(1)+`,
    version = version + 1,
    updated_at = `+r.bind(2)+`
WHERE proposal_id = `+r.bind(3)+`
  AND owner_user_id = `+r.bind(4)+`
  AND session_key = `+r.bind(5)+`
  AND scope_kind = `+r.bind(6)+`
  AND COALESCE(room_id, '') = `+r.bind(7)+`
  AND COALESCE(conversation_id, '') = `+r.bind(8)+`
  AND coordinator_agent_id = `+r.bind(9)+`
  AND version = `+r.bind(10)+`
  AND status IN ('sealed', 'materializing')`,
		command.LastError,
		r.timestamp(now),
		access.ProposalID,
		access.OwnerUserID,
		access.SessionKey,
		access.ScopeKind,
		access.RoomID,
		access.ConversationID,
		access.CoordinatorAgentID,
		command.ExpectedVersion,
	)
	if err != nil {
		return nil, err
	}
	return r.finishPlanProposalCAS(ctx, access, result, func(item protocol.ExecutionPlanProposal) bool {
		return item.Status == protocol.ExecutionPlanProposalStatusBlocked && item.LastError == command.LastError
	})
}

func (r *Repository) loadPlanProposalForMutation(
	ctx context.Context,
	access PlanProposalAccess,
) (*protocol.ExecutionPlanProposal, error) {
	item, err := r.GetPlanProposal(ctx, GetPlanProposalQuery{Access: access})
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, sql.ErrNoRows
	}
	return item, nil
}

func (r *Repository) finishPlanProposalCAS(
	ctx context.Context,
	access PlanProposalAccess,
	result sql.Result,
	replayMatches func(protocol.ExecutionPlanProposal) bool,
) (*protocol.ExecutionPlanProposal, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	current, err := r.loadPlanProposalForMutation(ctx, access)
	if err != nil {
		return nil, err
	}
	if affected == 1 || replayMatches(*current) {
		return current, nil
	}
	return nil, ErrVersionConflict
}

func (r *Repository) getPlanProposalByMaterializationCommand(
	ctx context.Context,
	queryer sqlQueryer,
	commandID string,
) (*protocol.ExecutionPlanProposal, error) {
	item, err := scanPlanProposal(queryer.QueryRowContext(
		ctx,
		r.planProposalSelect()+`WHERE materialization_command_id = `+r.bind(1),
		strings.TrimSpace(commandID),
	))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func validateProposalMaterializationReservation(
	item protocol.ExecutionPlanProposal,
	command MarkPlanProposalMaterializingCommand,
) error {
	if command.GoalID != item.GoalID || command.GoalObjectiveRevision != item.GoalObjectiveRevision {
		return fmt.Errorf("%w: materialization Goal fence differs from sealed proposal", ErrCommandConflict)
	}
	if item.GoalID == "" {
		if command.GoalActivationOrigin != "" || command.GoalActivationReason != "" {
			return fmt.Errorf("%w: Goal-free materialization carries Goal activation", ErrInvariant)
		}
	} else {
		if !validProposalGoalActivationOrigin(command.GoalActivationOrigin) ||
			!validProposalGoalActivationReason(command.GoalActivationReason) {
			return fmt.Errorf("%w: Goal materialization requires valid activation origin and reason", ErrInvariant)
		}
		if item.GoalActivationOrigin != "" && item.GoalActivationOrigin != command.GoalActivationOrigin {
			return fmt.Errorf("%w: Goal activation origin differs from sealed proposal", ErrCommandConflict)
		}
		if item.GoalActivationReason != "" && item.GoalActivationReason != command.GoalActivationReason {
			return fmt.Errorf("%w: Goal activation reason differs from sealed proposal", ErrCommandConflict)
		}
	}
	switch item.Document.Operation {
	case protocol.ExecutionPlanProposalCreate:
		if command.ReplacesExecutionID != item.ReplacesExecutionID {
			return fmt.Errorf(
				"%w: create predecessor differs from the sealed Goal transition",
				ErrCommandConflict,
			)
		}
		if item.GoalReservedExecutionID != "" &&
			command.ReservedExecutionID != item.GoalReservedExecutionID {
			return fmt.Errorf(
				"%w: create materialization differs from the Goal-reserved Execution identity",
				ErrCommandConflict,
			)
		}
	case protocol.ExecutionPlanProposalReplan:
		if command.ReservedExecutionID != item.TargetExecutionID || command.ReplacesExecutionID != "" {
			return fmt.Errorf("%w: replan must reserve its target Execution", ErrInvariant)
		}
	case protocol.ExecutionPlanProposalReplace:
		if command.ReservedExecutionID == item.TargetExecutionID ||
			command.ReplacesExecutionID != item.TargetExecutionID {
			return fmt.Errorf("%w: replace must reserve a successor linked to its target", ErrInvariant)
		}
	default:
		return fmt.Errorf("%w: invalid sealed proposal operation", ErrInvariant)
	}
	return nil
}

func planProposalCreateReplayMatches(
	existing protocol.ExecutionPlanProposal,
	requested protocol.ExecutionPlanProposal,
) bool {
	return existing.ID == requested.ID &&
		existing.ContentDigest == requested.ContentDigest &&
		existing.RootRoundID == requested.RootRoundID &&
		existing.RuntimeRoundID == requested.RuntimeRoundID &&
		existing.AgentRoundID == requested.AgentRoundID &&
		existing.GoalActivationOrigin == requested.GoalActivationOrigin &&
		existing.GoalActivationReason == requested.GoalActivationReason &&
		existing.ReplacesExecutionID == requested.ReplacesExecutionID
}

func planProposalMaterializingReplayMatches(
	item protocol.ExecutionPlanProposal,
	command MarkPlanProposalMaterializingCommand,
) bool {
	return item.Status == protocol.ExecutionPlanProposalStatusMaterializing &&
		item.ReservedExecutionID == command.ReservedExecutionID &&
		item.MaterializationCommandID == command.MaterializationCommandID &&
		item.GoalID == command.GoalID &&
		item.GoalObjectiveRevision == command.GoalObjectiveRevision &&
		item.GoalActivationOrigin == command.GoalActivationOrigin &&
		item.GoalActivationReason == command.GoalActivationReason &&
		item.ReplacesExecutionID == command.ReplacesExecutionID
}

func planProposalMaterializedReplayMatches(
	item protocol.ExecutionPlanProposal,
	command MarkPlanProposalMaterializedCommand,
) bool {
	return item.Status == protocol.ExecutionPlanProposalStatusMaterialized &&
		item.MaterializedExecutionID == command.MaterializedExecutionID &&
		item.MaterializedPlanID == command.MaterializedPlanID
}

func planProposalRetryReplayMatches(
	item protocol.ExecutionPlanProposal,
	command SchedulePlanProposalRetryCommand,
) bool {
	return item.Status == protocol.ExecutionPlanProposalStatusMaterializing &&
		item.LastError == command.LastError &&
		equalTimePointers(item.NextAttemptAt, command.NextAttemptAt)
}

func planProposalConfirmationReplayMatches(
	item protocol.ExecutionPlanProposal,
	command MarkPlanProposalConfirmationCommand,
) bool {
	if item.Status != protocol.ExecutionPlanProposalStatusMaterialized ||
		item.ConfirmationState != command.ConfirmationState {
		return false
	}
	if command.ConfirmationState == protocol.ExecutionPlanProposalConfirmationConfirmed {
		return true
	}
	return item.LastError == command.LastError && equalTimePointers(item.NextAttemptAt, command.NextAttemptAt)
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	// PostgreSQL timestamps have microsecond precision. Normalize before writes and
	// replay comparison so a successful command stays idempotent after a DB round trip.
	normalized := value.UTC().Truncate(time.Microsecond)
	return &normalized
}

func (r *Repository) planProposalTimestamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return r.timestamp(value.UTC())
}

func equalTimePointers(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}
