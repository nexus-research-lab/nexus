// INPUT: Current root owner's explicit progress and external-effect receipts.
// OUTPUT: Immutable checkpoint receipt and resume context without a completion transition.
// POS: Semantic checkpoint command; evidence must be reconciled again before resuming effects.
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type RecordCheckpointInput struct {
	ExecutionID, CommandID, WorkItemID, LogicalKey string
	SnapshotRevision                               int64
	Data                                           protocol.WorkCheckpointData
}
type checkpointRepository interface {
	RecordCheckpoint(context.Context, store.RecordCheckpointCommand) (*protocol.ExecutionSnapshot, error)
}

func (s *Service) RecordCheckpoint(ctx context.Context, actor ActorContext, input RecordCheckpointInput) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(ctx, actor, input.ExecutionID, input.SnapshotRevision, false, false)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if actor.ObservationOnly || actor.Role == ExecutionActorSubagent || actor.ReviewBinding != nil || input.CommandID == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeWrongOwner, "only the current root owner can record progress"), nil), nil
	}
	raw, err := json.Marshal(input.Data)
	if err != nil || len(raw) > 16384 || input.Data.Summary == "" || input.Data.NextStep == "" || len(input.Data.Summary) > 2048 || len(input.Data.NextStep) > 2048 || len(input.Data.ArtifactRefs) > 32 || len(input.Data.Evidence) > 32 || len(input.Data.Effects) > 32 {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "checkpoint requires concise summary, next_step and bounded evidence (16 KiB total)"), nil), nil
	}
	for _, effect := range input.Data.Effects {
		if effect.Operation == "" || (effect.Outcome != "unknown" && effect.Outcome != "confirmed") || (effect.Outcome == "confirmed" && effect.ReceiptRef == "") {
			return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "each effect requires operation, confirmed/unknown outcome and a receipt for confirmed results"), nil), nil
		}
	}
	bound, err := bindSubmitWorkInputFromTrustedResponsibility(actor, snapshot, SubmitWorkInput{WorkItemID: input.WorkItemID, LogicalKey: input.LogicalKey})
	if err != nil {
		return RejectedResult(snapshot, err, nil), nil
	}
	work, spec, err := resolvePlanWork(snapshot, bound.WorkItemID, bound.LogicalKey)
	if err != nil {
		return RejectedResult(snapshot, err, nil), nil
	}
	assignment := activeAssignmentForWork(snapshot, work.ID)
	attempt := rootAttemptForAssignment(snapshot, assignment)
	if assignment == nil || assignment.OwnerAgentID != actor.AgentID || attempt == nil || attempt.ParentAttemptID != "" || attempt.Status != protocol.WorkAttemptStatusPending && attempt.Status != protocol.WorkAttemptStatusRunning || latestUnreviewedSubmission(snapshot, work.ID) != nil {
		return RejectedResult(snapshot, domainError(ErrorCodeWrongOwner, "checkpoint requires an active, unsubmitted root responsibility"), nil), nil
	}
	if actor.WorkBinding != nil && actor.WorkBinding.AttemptID != attempt.ID {
		return RejectedResult(snapshot, workBindingMismatch("checkpoint cannot move to a successor Attempt"), nil), nil
	}
	repository, ok := s.repository.(checkpointRepository)
	if !ok {
		return MutationResult{}, errors.New("checkpoint repository unavailable")
	}
	p := protocol.WorkCheckpoint{ExecutionID: snapshot.Execution.ID, PlanID: snapshot.Plan.ID, WorkItemID: work.ID, SpecID: spec.ID, AssignmentID: assignment.ID, AttemptID: attempt.ID, AgentID: actor.AgentID, WorkCheckpointData: input.Data}
	digestRaw, _ := json.Marshal(p)
	digest := sha256.Sum256(digestRaw)
	id := sha256.Sum256([]byte(input.CommandID))
	p.ID = s.id("checkpoint")
	updated, err := repository.RecordCheckpoint(ctx, store.RecordCheckpointCommand{Checkpoint: p, ExpectedExecutionVersion: snapshot.Execution.Version, Digest: hex.EncodeToString(digest[:]), Meta: s.commandMeta(actor, fmt.Sprintf("%x", id), "checkpoint")})
	if err != nil {
		return s.storageMutationResult(snapshot, err, nil)
	}
	return AppliedResult(updated, []string{"checkpoint:" + work.ID}, nil), nil
}
