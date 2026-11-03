// INPUT: Coordinator-declared ready assignments, current revision and stable command identity.
// OUTPUT: One atomic batch receipt; exact self binding only after the entire batch commits.
// POS: Shared assignment rules with durable intent checking and replay-safe responsibility binding.
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// AssignmentBatchItem is business intent only; identity and fencing belong to the batch.
type AssignmentBatchItem struct {
	WorkItemID      string                         `json:"work_item_id,omitempty"`
	LogicalKey      string                         `json:"logical_key,omitempty"`
	TargetAgentID   string                         `json:"target_agent_id"`
	ReturnToAgentID string                         `json:"return_to_agent_id,omitempty"`
	Strategy        protocol.AssignmentStrategy    `json:"strategy,omitempty"`
	Reason          string                         `json:"reason,omitempty"`
	Instruction     string                         `json:"instruction,omitempty"`
	DispatchKind    protocol.ExecutionDispatchKind `json:"dispatch_kind,omitempty"`
}

type AssignWorkBatchInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	Assignments      []AssignmentBatchItem
}

type assignmentBatchRepository interface {
	AssignBatch(context.Context, orchestrationstore.AssignBatchCommand) (*protocol.ExecutionSnapshot, error)
	FindAssignmentBatchReceipt(context.Context, string, string) (*protocol.ExecutionEvent, error)
}

// AssignWorkBatch applies declared ready assignments in one transaction, without
// inferring ownership or scheduling future dependency transitions.
func (s *Service) AssignWorkBatch(ctx context.Context, actor ActorContext, input AssignWorkBatchInput) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(ctx, actor, input.ExecutionID, input.SnapshotRevision, true, false)
	if err != nil || (rejected != nil && (snapshot == nil || rejected.ReasonCode != ErrorCodeStaleExecution)) {
		return resultOrZero(rejected), err
	}
	// Only an exact prior receipt may bypass a stale revision, never authority.
	if rejected != nil {
		if err := s.requireRuntimeCoordination(actor, snapshot); err != nil {
			return RejectedResult(snapshot, err, nil), nil
		}
		if err := requireCoordinator(actor, snapshot); err != nil {
			return RejectedResult(snapshot, err, nil), nil
		}
	}
	if input.CommandID == "" || len(input.Assignments) < 1 || len(input.Assignments) > protocol.ExecutionProjectionCollectionLimit {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "assignment batch requires a command id and 1-32 items"), nil), nil
	}
	repository, ok := s.repository.(assignmentBatchRepository)
	if !ok {
		return MutationResult{}, fmt.Errorf("assignment batch repository is unavailable")
	}
	// Digest business intent, excluding the host's refreshed snapshot revision.
	payload, err := json.Marshal(input.Assignments)
	if err != nil {
		return MutationResult{}, err
	}
	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	requestID := sha256.Sum256([]byte(input.CommandID))
	meta := s.commandMeta(actor, hex.EncodeToString(requestID[:]), "assign_batch")
	receipt, err := repository.FindAssignmentBatchReceipt(ctx, snapshot.Execution.ID, meta.CommandID)
	if err != nil {
		return MutationResult{}, err
	}
	if receipt != nil {
		if receipt.Type != protocol.ExecutionEventWorkAssigned || receipt.EntityType != protocol.ExecutionEntityExecution || receipt.ActorID != actor.AgentID || receipt.Payload["request_digest"] != digest {
			return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "assignment batch request_id was reused for a different intent"), nil), nil
		}
		result := NoOpResult(snapshot, "assignment batch was already committed")
		return withExactBatchSelfBinding(actor, result, receipt.Payload), nil
	}
	if rejected != nil {
		return *rejected, nil
	}
	// A bound work/review lane cannot create new coordinator responsibilities.
	if actor.WorkBinding != nil || actor.ReviewBinding != nil {
		return RejectedResult(snapshot, domainError(ErrorCodeWrongOwner, "assignment batch requires the coordinator lane"), nil), nil
	}
	batch := orchestrationstore.AssignBatchCommand{
		ExecutionID: snapshot.Execution.ID, ExpectedExecutionVersion: input.SnapshotRevision,
		RequestDigest: digest, Meta: meta,
	}
	batch.Meta.Payload = map[string]any{}
	seen := make(map[string]bool, len(input.Assignments))
	changed := make([]string, 0, 3*len(input.Assignments))
	for index, item := range input.Assignments {
		work, spec, resolveErr := resolvePlanWork(snapshot, item.WorkItemID, item.LogicalKey)
		if resolveErr != nil {
			return RejectedResult(snapshot, resolveErr, nextActions(snapshot, actor)), nil
		}
		if seen[work.ID] || activeAssignmentForWork(snapshot, work.ID) != nil {
			return RejectedResult(snapshot, newDomainError(ErrorCodeDuplicateAssignment, "batch repeats a Work Item or it already has a current Assignment", work.LogicalKey, work.ID), nil), nil
		}
		seen[work.ID] = true
		// Bound child command length independent of client request-id length.
		childID := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", meta.CommandID, index)))
		command, buildErr := s.prepareReadyAssignment(ctx, actor, snapshot, work, spec, AssignWorkInput{
			ExecutionID: input.ExecutionID, SnapshotRevision: input.SnapshotRevision,
			CommandID:  "assign_batch_" + hex.EncodeToString(childID[:]),
			WorkItemID: item.WorkItemID, LogicalKey: item.LogicalKey, TargetAgentID: item.TargetAgentID,
			ReturnToAgentID: item.ReturnToAgentID, Strategy: item.Strategy, Reason: item.Reason,
			Instruction: item.Instruction, DispatchKind: item.DispatchKind,
		})
		if buildErr != nil {
			return RejectedResult(snapshot, buildErr, nextActions(snapshot, actor)), nil
		}
		if command.Assignment.Strategy == protocol.AssignmentStrategySelf && index != len(input.Assignments)-1 {
			return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "at most one self assignment is permitted and it must be the last batch item"), nil), nil
		}
		batch.Assignments = append(batch.Assignments, command)
		changed = append(changed, "assignment:"+command.Assignment.ID, "attempt:"+command.RootAttempt.ID)
		if command.Dispatch != nil {
			changed = append(changed, "dispatch:"+command.Dispatch.ID)
		}
		if command.Assignment.Strategy == protocol.AssignmentStrategySelf {
			batch.Meta.Payload["self_work_item_id"] = work.ID
			batch.Meta.Payload["self_assignment_id"] = command.Assignment.ID
			batch.Meta.Payload["self_attempt_id"] = command.RootAttempt.ID
		}
	}
	updated, err := repository.AssignBatch(ctx, batch)
	if err != nil {
		return s.storageMutationResult(snapshot, err, nextActions(snapshot, actor))
	}
	// A concurrent identical command may win the commit. Only report the actual
	// receipt's binding, never the candidate random IDs from this invocation.
	receipt, err = repository.FindAssignmentBatchReceipt(ctx, snapshot.Execution.ID, meta.CommandID)
	if err != nil {
		return MutationResult{}, err
	}
	if receipt == nil {
		return MutationResult{}, fmt.Errorf("committed assignment batch receipt is missing")
	}
	if receipt.ID != meta.EventID {
		return withExactBatchSelfBinding(actor, NoOpResult(updated, "assignment batch was already committed"), receipt.Payload), nil
	}
	return withExactBatchSelfBinding(actor, AppliedResult(updated, changed, nextActions(updated, actor)), receipt.Payload), nil
}

func withExactBatchSelfBinding(actor ActorContext, result MutationResult, payload map[string]any) MutationResult {
	workID, _ := payload["self_work_item_id"].(string)
	assignmentID, _ := payload["self_assignment_id"].(string)
	attemptID, _ := payload["self_attempt_id"].(string)
	if result.Snapshot == nil || !result.Snapshot.Execution.Status.Current() || assignmentID == "" || attemptID == "" {
		return result
	}
	assignment := activeAssignmentForWork(result.Snapshot, workID)
	attempt := rootAttemptForAssignment(result.Snapshot, assignment)
	if assignment == nil || assignment.ID != assignmentID || attempt == nil || attempt.ID != attemptID {
		return result
	}
	return withRoomSelfWorkBindingReceipt(actor, result, workID)
}
