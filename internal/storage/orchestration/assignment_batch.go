// INPUT: One authorized assignment batch, exact intent digest and Execution revision.
// OUTPUT: All-or-nothing ownership/attempt/outbox writes and a durable replay receipt.
// POS: Batched dispatch transaction; individual assignments reuse the single-item invariants.
package orchestration

import (
	"context"
	"fmt"
	"maps"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) FindAssignmentBatchReceipt(ctx context.Context, executionID, commandID string) (*protocol.ExecutionEvent, error) {
	return r.findEventByCommand(ctx, r.db, executionID, commandID)
}

func (r *Repository) AssignBatch(ctx context.Context, command AssignBatchCommand) (*protocol.ExecutionSnapshot, error) {
	if len(command.Assignments) == 0 || len(command.Assignments) > protocol.ExecutionProjectionCollectionLimit || command.RequestDigest == "" {
		return nil, fmt.Errorf("%w: assignment batch requires 1-32 items and a request digest", ErrInvariant)
	}
	mutation, err := r.beginMutation(ctx, command.ExecutionID, command.ExpectedExecutionVersion, command.Meta, protocol.ExecutionEventWorkAssigned)
	if err != nil {
		return nil, err
	}
	defer r.abortMutation(mutation)
	if mutation.replayed {
		var reader sqlQueryer = r.db
		if mutation.tx != nil {
			reader = mutation.tx
		}
		receipt, err := r.findEventByCommand(ctx, reader, command.ExecutionID, command.Meta.CommandID)
		if err != nil {
			return nil, err
		}
		if receipt == nil || receipt.EntityType != protocol.ExecutionEntityExecution || receipt.Payload["request_digest"] != command.RequestDigest || receipt.ActorID != command.Meta.ActorID {
			return nil, fmt.Errorf("%w: assignment batch request_id was reused for a different intent", ErrCommandConflict)
		}
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	seen := make(map[string]bool, len(command.Assignments))
	planID := command.Assignments[0].Assignment.PlanID
	for _, item := range command.Assignments {
		if item.Assignment.ExecutionID != command.ExecutionID || item.Assignment.PlanID != planID || seen[item.Assignment.WorkItemID] {
			return nil, fmt.Errorf("%w: duplicate or cross-execution assignment in batch", ErrInvariant)
		}
		if err := validateMeta(item.Meta); err != nil {
			return nil, err
		}
		seen[item.Assignment.WorkItemID] = true
		if err := normalizeAssignment(&item.Assignment, r.currentTime()); err != nil {
			return nil, err
		}
		event, err := r.insertAssignmentChain(ctx, mutation.tx, item)
		if err != nil {
			return nil, err
		}
		event.ID, event.ExecutionID, event.CommandID = item.Meta.EventID, command.ExecutionID, item.Meta.CommandID
		event.Type, event.ActorKind, event.ActorID = protocol.ExecutionEventWorkAssigned, command.Meta.ActorKind, command.Meta.ActorID
		event.RootRoundID, event.RuntimeRoundID, event.AgentRoundID = command.Meta.RootRoundID, command.Meta.RuntimeRoundID, command.Meta.AgentRoundID
		event.CreatedAt = timeOr(command.Meta.CreatedAt, r.currentTime())
		event.Payload = item.Meta.Payload
		if err := r.insertEvent(ctx, mutation.tx, event); err != nil {
			return nil, err
		}
	}
	command.Meta.Payload = maps.Clone(command.Meta.Payload)
	if command.Meta.Payload == nil {
		command.Meta.Payload = map[string]any{}
	}
	command.Meta.Payload["request_digest"] = command.RequestDigest
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType: protocol.ExecutionEntityExecution, EntityID: command.ExecutionID,
		EntityVersion: command.ExpectedExecutionVersion + 1, PlanID: command.Assignments[0].Assignment.PlanID,
	})
}
