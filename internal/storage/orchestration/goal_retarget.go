// INPUT: durable Goal objective transition and exact old Goal-bound Execution revision.
// OUTPUT: replay-safe predecessor fence with a reserved successor identity；transient graph 被 supersede，既有 terminal graph 保持原终态。
// POS: SQL half of the Goal objective revision rebase saga; successor Plan creation remains a later atomic command.
package orchestration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SupersedeGoalRevision fences all late work from the old Goal objective
// revision. Existing terminal history is accepted as an already-closed
// predecessor and is never rewritten.
func (r *Repository) SupersedeGoalRevision(
	ctx context.Context,
	command SupersedeGoalRevisionCommand,
) (*protocol.ExecutionSnapshot, error) {
	command.ExecutionID = strings.TrimSpace(command.ExecutionID)
	command.ExpectedOwnerUserID = strings.TrimSpace(command.ExpectedOwnerUserID)
	command.GoalID = strings.TrimSpace(command.GoalID)
	command.SuccessorExecutionID = strings.TrimSpace(command.SuccessorExecutionID)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ExecutionID == "" || command.GoalID == "" ||
		command.SuccessorExecutionID == "" || command.SuccessorExecutionID == command.ExecutionID ||
		command.OldGoalObjectiveRevision <= 0 ||
		command.NewGoalObjectiveRevision != command.OldGoalObjectiveRevision+1 ||
		command.Reason == "" {
		return nil, fmt.Errorf("%w: complete Goal revision supersede identity is required", ErrInvariant)
	}
	if err := validateExpectedVersion(command.ExpectedExecutionVersion, "expected execution version"); err != nil {
		return nil, err
	}
	if err := validateMeta(command.Meta); err != nil {
		return nil, err
	}
	if existing, err := r.findEventByCommand(
		ctx,
		r.db,
		command.ExecutionID,
		command.Meta.CommandID,
	); err != nil {
		return nil, err
	} else if existing != nil {
		if !goalRevisionSupersedeEventMatches(*existing, command) {
			return nil, ErrCommandConflict
		}
		return r.GetSnapshot(ctx, command.ExecutionID)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, findErr := r.findEventByCommand(
		ctx,
		tx,
		command.ExecutionID,
		command.Meta.CommandID,
	); findErr != nil {
		return nil, findErr
	} else if existing != nil {
		if !goalRevisionSupersedeEventMatches(*existing, command) {
			return nil, ErrCommandConflict
		}
		_ = tx.Rollback()
		return r.GetSnapshot(ctx, command.ExecutionID)
	}
	current, err := r.getExecution(ctx, tx, command.ExecutionID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, sql.ErrNoRows
	}
	if strings.TrimSpace(current.GoalID) != command.GoalID ||
		current.GoalObjectiveRevision != command.OldGoalObjectiveRevision {
		return nil, fmt.Errorf("%w: old Execution is outside the requested Goal objective revision", ErrInvariant)
	}
	if command.ExpectedOwnerUserID != "" &&
		strings.TrimSpace(current.OwnerUserID) != command.ExpectedOwnerUserID {
		return nil, fmt.Errorf("%w: old Execution belongs to another owner", ErrInvariant)
	}
	now := r.currentTime()
	if !currentExecutionStatus(current.Status) {
		if !terminalExecutionStatus(current.Status) {
			return nil, fmt.Errorf("%w: Goal revision predecessor status is invalid", ErrInvariant)
		}
		existingReservation, reservationErr := r.findGoalRevisionSupersedeEvent(
			ctx,
			tx,
			command.ExecutionID,
			command.GoalID,
		)
		if reservationErr != nil {
			return nil, reservationErr
		}
		if existingReservation != nil {
			return nil, fmt.Errorf(
				"%w: terminal Goal revision already reserved another successor",
				ErrInvariant,
			)
		}
		if current.Version != command.ExpectedExecutionVersion {
			return nil, ErrVersionConflict
		}
		// A terminal Execution is already physically closed. Preserve its
		// status and graph history, but claim one aggregate version and append
		// the deterministic supersede reservation required to admit the
		// successor revision.
		result, updateErr := tx.ExecContext(ctx, `
UPDATE executions
SET version = version + 1,
    updated_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND version = `+r.bind(3)+`
  AND goal_id = `+r.bind(4)+`
  AND goal_objective_revision = `+r.bind(5)+`
  AND status IN ('completed', 'failed', 'cancelled', 'superseded')`,
			r.timestamp(now),
			command.ExecutionID,
			command.ExpectedExecutionVersion,
			command.GoalID,
			command.OldGoalObjectiveRevision,
		)
		if updateErr != nil {
			return nil, updateErr
		}
		if err = requireOne(result); err != nil {
			if errors.Is(err, ErrVersionConflict) {
				_ = tx.Rollback()
				if existing, findErr := r.findEventByCommand(
					ctx,
					r.db,
					command.ExecutionID,
					command.Meta.CommandID,
				); findErr != nil {
					return nil, findErr
				} else if existing != nil && goalRevisionSupersedeEventMatches(*existing, command) {
					return r.GetSnapshot(ctx, command.ExecutionID)
				}
			}
			return nil, err
		}
		payload := goalRevisionSupersedePayload(command)
		payload["predecessor_status"] = string(current.Status)
		payload["predecessor_preserved"] = true
		event := executionEvent(
			command.Meta,
			command.ExecutionID,
			protocol.ExecutionEventSuperseded,
			command.ExecutionID,
			current.Version+1,
			payload,
		)
		event.GoalID = command.GoalID
		if err = r.insertEvent(ctx, tx, event); err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetSnapshot(ctx, command.ExecutionID)
	}
	if current.Version != command.ExpectedExecutionVersion {
		return nil, ErrVersionConflict
	}
	result, err := tx.ExecContext(ctx, `
UPDATE executions
SET status = 'superseded',
    version = version + 1,
    updated_at = `+r.bind(1)+`
WHERE execution_id = `+r.bind(2)+`
  AND version = `+r.bind(3)+`
  AND goal_id = `+r.bind(4)+`
  AND goal_objective_revision = `+r.bind(5)+`
  AND status IN ('active', 'waiting', 'paused')`,
		r.timestamp(now),
		command.ExecutionID,
		command.ExpectedExecutionVersion,
		command.GoalID,
		command.OldGoalObjectiveRevision,
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
		command.ExecutionID,
		protocol.ExecutionStatusSuperseded,
		command.Meta.CommandID,
		command.Reason,
		now,
	); err != nil {
		return nil, err
	}
	event := executionEvent(
		command.Meta,
		command.ExecutionID,
		protocol.ExecutionEventSuperseded,
		command.ExecutionID,
		command.ExpectedExecutionVersion+1,
		goalRevisionSupersedePayload(command),
	)
	event.GoalID = command.GoalID
	if err = r.insertEvent(ctx, tx, event); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetSnapshot(ctx, command.ExecutionID)
}

func goalRevisionSupersedePayload(command SupersedeGoalRevisionCommand) map[string]any {
	return map[string]any{
		"reason":                      command.Reason,
		"owner_user_id":               command.ExpectedOwnerUserID,
		successorExecutionPayloadKey:  command.SuccessorExecutionID,
		"old_goal_objective_revision": command.OldGoalObjectiveRevision,
		"new_goal_objective_revision": command.NewGoalObjectiveRevision,
	}
}

func metadataStringValue(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func goalRevisionSupersedeEventMatches(
	event protocol.ExecutionEvent,
	command SupersedeGoalRevisionCommand,
) bool {
	return event.Type == protocol.ExecutionEventSuperseded &&
		strings.TrimSpace(event.GoalID) == command.GoalID &&
		metadataStringValue(event.Payload, successorExecutionPayloadKey) == command.SuccessorExecutionID &&
		protocol.GoalMetadataInt64(event.Payload, "old_goal_objective_revision") ==
			command.OldGoalObjectiveRevision &&
		protocol.GoalMetadataInt64(event.Payload, "new_goal_objective_revision") ==
			command.NewGoalObjectiveRevision &&
		(command.ExpectedOwnerUserID == "" ||
			metadataStringValue(event.Payload, "owner_user_id") == command.ExpectedOwnerUserID)
}

func (r *Repository) validateGoalRevisionSuccessor(
	ctx context.Context,
	queryer sqlQueryer,
	successor protocol.Execution,
) error {
	predecessor, err := r.getExecution(ctx, queryer, successor.ReplacesExecutionID)
	if err != nil {
		return err
	}
	if predecessor == nil {
		return fmt.Errorf("%w: Goal revision predecessor does not exist", ErrInvariant)
	}
	if !terminalExecutionStatus(predecessor.Status) ||
		predecessor.GoalID != successor.GoalID ||
		predecessor.GoalObjectiveRevision <= 0 ||
		successor.GoalObjectiveRevision != predecessor.GoalObjectiveRevision+1 ||
		predecessor.OwnerUserID != successor.OwnerUserID ||
		predecessor.SessionKey != successor.SessionKey ||
		predecessor.ScopeKind != successor.ScopeKind ||
		predecessor.RoomID != successor.RoomID ||
		predecessor.ConversationID != successor.ConversationID {
		return fmt.Errorf(
			"%w: Goal revision successor does not match its terminal predecessor",
			ErrInvariant,
		)
	}
	event, err := r.findGoalRevisionSupersedeEvent(
		ctx,
		queryer,
		predecessor.ID,
		successor.GoalID,
	)
	if err != nil {
		return err
	}
	if event == nil ||
		metadataStringValue(event.Payload, successorExecutionPayloadKey) != successor.ID ||
		protocol.GoalMetadataInt64(event.Payload, "old_goal_objective_revision") !=
			predecessor.GoalObjectiveRevision ||
		protocol.GoalMetadataInt64(event.Payload, "new_goal_objective_revision") !=
			successor.GoalObjectiveRevision {
		return fmt.Errorf(
			"%w: Goal revision successor was not reserved by its predecessor supersede event",
			ErrInvariant,
		)
	}
	return nil
}

func terminalExecutionStatus(status protocol.ExecutionStatus) bool {
	switch status {
	case protocol.ExecutionStatusCompleted,
		protocol.ExecutionStatusFailed,
		protocol.ExecutionStatusCancelled,
		protocol.ExecutionStatusSuperseded:
		return true
	default:
		return false
	}
}

func (r *Repository) findGoalRevisionSupersedeEvent(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
	goalID string,
) (*protocol.ExecutionEvent, error) {
	row := queryer.QueryRowContext(
		ctx,
		eventSelect(r.dialect.JSONText("payload_json"))+`
WHERE execution_id = `+r.bind(1)+`
  AND goal_id = `+r.bind(2)+`
  AND event_type = `+r.bind(3)+`
ORDER BY sequence DESC
LIMIT 1`,
		strings.TrimSpace(executionID),
		strings.TrimSpace(goalID),
		protocol.ExecutionEventSuperseded,
	)
	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}
