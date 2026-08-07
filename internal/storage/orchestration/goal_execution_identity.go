// INPUT: Goal-bound Execution materialization or a retarget fence for an identity that never materialized.
// OUTPUT: one durable materialized/fenced identity claim that makes delayed CreateWithPlan race-safe.
// POS: cross-transaction identity mutex between Goal metadata reservation and SQL Execution creation.
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	goalExecutionIdentityMaterialized = "materialized"
	goalExecutionIdentityFenced       = "fenced"
)

type goalExecutionIdentityClaim struct {
	ExecutionID           string
	GoalID                string
	GoalObjectiveRevision int64
	OwnerUserID           string
	State                 string
	CommandID             string
	SuccessorExecutionID  string
}

func (r *Repository) claimGoalExecutionMaterialization(
	ctx context.Context,
	tx *sql.Tx,
	execution protocol.Execution,
	commandID string,
	now time.Time,
) error {
	if strings.TrimSpace(execution.GoalID) == "" {
		return nil
	}
	claim := goalExecutionIdentityClaim{
		ExecutionID:           strings.TrimSpace(execution.ID),
		GoalID:                strings.TrimSpace(execution.GoalID),
		GoalObjectiveRevision: execution.GoalObjectiveRevision,
		OwnerUserID:           strings.TrimSpace(execution.OwnerUserID),
		State:                 goalExecutionIdentityMaterialized,
		CommandID:             strings.TrimSpace(commandID),
	}
	if claim.ExecutionID == "" || claim.GoalID == "" ||
		claim.GoalObjectiveRevision <= 0 || claim.OwnerUserID == "" ||
		claim.CommandID == "" {
		return fmt.Errorf("%w: complete Goal Execution materialization claim is required", ErrInvariant)
	}
	if err := r.insertGoalExecutionIdentityClaim(ctx, tx, claim, now); err != nil {
		return err
	}
	stored, err := r.getGoalExecutionIdentityClaim(ctx, tx, claim.ExecutionID)
	if err != nil {
		return err
	}
	if stored == nil {
		return r.goalExecutionIdentityRevisionConflict(
			ctx,
			tx,
			claim.GoalID,
			claim.GoalObjectiveRevision,
		)
	}
	if stored.State == goalExecutionIdentityFenced {
		return fmt.Errorf(
			"%w: Goal Execution identity %s was fenced before materialization",
			ErrInvariant,
			claim.ExecutionID,
		)
	}
	if !goalExecutionIdentityClaimMatches(*stored, claim) {
		return ErrCommandConflict
	}
	return nil
}

// FenceGoalExecutionIdentity wins the same durable identity claim used by
// CreateWithPlan. If materialization won the race it returns false; otherwise
// all delayed creation attempts for the reserved ID are permanently rejected.
func (r *Repository) FenceGoalExecutionIdentity(
	ctx context.Context,
	command FenceGoalExecutionIdentityCommand,
) (bool, error) {
	command.ExecutionID = strings.TrimSpace(command.ExecutionID)
	command.ExpectedOwnerUserID = strings.TrimSpace(command.ExpectedOwnerUserID)
	command.GoalID = strings.TrimSpace(command.GoalID)
	command.SuccessorExecutionID = strings.TrimSpace(command.SuccessorExecutionID)
	if command.ExecutionID == "" || command.GoalID == "" ||
		command.GoalObjectiveRevision <= 0 || command.SuccessorExecutionID == "" ||
		command.SuccessorExecutionID == command.ExecutionID {
		return false, fmt.Errorf("%w: complete Goal Execution identity fence is required", ErrInvariant)
	}
	if err := validateMeta(command.Meta); err != nil {
		return false, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	existingExecution, err := r.getExecution(ctx, tx, command.ExecutionID)
	if err != nil {
		return false, err
	}
	if existingExecution != nil {
		return false, nil
	}
	claim := goalExecutionIdentityClaim{
		ExecutionID:           command.ExecutionID,
		GoalID:                command.GoalID,
		GoalObjectiveRevision: command.GoalObjectiveRevision,
		OwnerUserID:           command.ExpectedOwnerUserID,
		State:                 goalExecutionIdentityFenced,
		CommandID:             strings.TrimSpace(command.Meta.CommandID),
		SuccessorExecutionID:  command.SuccessorExecutionID,
	}
	if err = r.insertGoalExecutionIdentityClaim(
		ctx,
		tx,
		claim,
		timeOr(command.Meta.CreatedAt, r.currentTime()),
	); err != nil {
		return false, err
	}
	stored, err := r.getGoalExecutionIdentityClaim(ctx, tx, claim.ExecutionID)
	if err != nil {
		return false, err
	}
	if stored == nil {
		return false, r.goalExecutionIdentityRevisionConflict(
			ctx,
			tx,
			claim.GoalID,
			claim.GoalObjectiveRevision,
		)
	}
	if stored.State == goalExecutionIdentityMaterialized {
		return false, nil
	}
	if !goalExecutionIdentityClaimMatches(*stored, claim) {
		return false, ErrCommandConflict
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) insertGoalExecutionIdentityClaim(
	ctx context.Context,
	executor sqlExecutor,
	claim goalExecutionIdentityClaim,
	now time.Time,
) error {
	_, err := executor.ExecContext(ctx, `
INSERT INTO goal_execution_identity_claims (
    execution_id, goal_id, goal_objective_revision, owner_user_id,
    claim_state, command_id, successor_execution_id, created_at
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+
		r.bind(5)+`,`+r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`)
ON CONFLICT DO NOTHING`,
		claim.ExecutionID,
		claim.GoalID,
		claim.GoalObjectiveRevision,
		nullString(claim.OwnerUserID),
		claim.State,
		claim.CommandID,
		nullString(claim.SuccessorExecutionID),
		r.timestamp(now),
	)
	return err
}

func (r *Repository) getGoalExecutionIdentityClaim(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
) (*goalExecutionIdentityClaim, error) {
	return scanGoalExecutionIdentityClaim(queryer.QueryRowContext(
		ctx,
		`SELECT execution_id, goal_id, goal_objective_revision, owner_user_id,
		        claim_state, command_id, successor_execution_id
		   FROM goal_execution_identity_claims
		  WHERE execution_id = `+r.bind(1),
		strings.TrimSpace(executionID),
	))
}

func (r *Repository) goalExecutionIdentityRevisionConflict(
	ctx context.Context,
	queryer sqlQueryer,
	goalID string,
	revision int64,
) error {
	stored, err := r.getGoalExecutionIdentityClaimByRevision(
		ctx,
		queryer,
		goalID,
		revision,
	)
	if err != nil {
		return err
	}
	if stored == nil {
		return fmt.Errorf("%w: Goal Execution identity claim disappeared", ErrInvariant)
	}
	return fmt.Errorf(
		"%w: Goal objective revision is already claimed by Execution %s",
		ErrCommandConflict,
		stored.ExecutionID,
	)
}

func (r *Repository) validateFencedGoalRevisionSuccessor(
	ctx context.Context,
	queryer sqlQueryer,
	execution protocol.Execution,
) error {
	predecessor, err := r.getGoalExecutionIdentityClaimByRevision(
		ctx,
		queryer,
		execution.GoalID,
		execution.GoalObjectiveRevision-1,
	)
	if err != nil {
		return err
	}
	if predecessor == nil ||
		predecessor.State != goalExecutionIdentityFenced ||
		predecessor.SuccessorExecutionID != execution.ID ||
		(predecessor.OwnerUserID != "" &&
			predecessor.OwnerUserID != strings.TrimSpace(execution.OwnerUserID)) {
		return fmt.Errorf(
			"%w: Goal revision successor was not reserved by a fenced predecessor identity",
			ErrInvariant,
		)
	}
	return nil
}

func (r *Repository) getGoalExecutionIdentityClaimByRevision(
	ctx context.Context,
	queryer sqlQueryer,
	goalID string,
	revision int64,
) (*goalExecutionIdentityClaim, error) {
	return scanGoalExecutionIdentityClaim(queryer.QueryRowContext(
		ctx,
		`SELECT execution_id, goal_id, goal_objective_revision, owner_user_id,
		        claim_state, command_id, successor_execution_id
		   FROM goal_execution_identity_claims
		  WHERE goal_id = `+r.bind(1)+`
		    AND goal_objective_revision = `+r.bind(2),
		strings.TrimSpace(goalID),
		revision,
	))
}

func scanGoalExecutionIdentityClaim(
	row *sql.Row,
) (*goalExecutionIdentityClaim, error) {
	var claim goalExecutionIdentityClaim
	var ownerUserID sql.NullString
	var successorExecutionID sql.NullString
	err := row.Scan(
		&claim.ExecutionID,
		&claim.GoalID,
		&claim.GoalObjectiveRevision,
		&ownerUserID,
		&claim.State,
		&claim.CommandID,
		&successorExecutionID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	claim.OwnerUserID = nullStringValue(ownerUserID)
	claim.SuccessorExecutionID = nullStringValue(successorExecutionID)
	return &claim, nil
}

func goalExecutionIdentityClaimMatches(
	stored goalExecutionIdentityClaim,
	expected goalExecutionIdentityClaim,
) bool {
	return stored.ExecutionID == expected.ExecutionID &&
		stored.GoalID == expected.GoalID &&
		stored.GoalObjectiveRevision == expected.GoalObjectiveRevision &&
		(expected.OwnerUserID == "" || stored.OwnerUserID == expected.OwnerUserID) &&
		stored.State == expected.State &&
		stored.CommandID == expected.CommandID &&
		stored.SuccessorExecutionID == expected.SuccessorExecutionID
}
