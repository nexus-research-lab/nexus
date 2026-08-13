// INPUT: trusted runtime/scheduler boundary observation and current actor fence.
// OUTPUT: idempotent, auditable persistence evidence on the current Execution and its session invalidation fact.
// POS: runtime lifecycle facts enter adaptive Goal policy here, never through model booleans.
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// PersistenceEvidenceKind is a backend-observed reason that required work must
// survive the current execution boundary.
type PersistenceEvidenceKind string

const (
	PersistenceEvidenceContextBoundary PersistenceEvidenceKind = "context_boundary"
	PersistenceEvidenceScheduledRetry  PersistenceEvidenceKind = "scheduled_retry"
)

type persistenceEvidenceRepository interface {
	RecordEvidence(
		context.Context,
		orchestrationstore.RecordEvidenceCommand,
	) (*protocol.ExecutionSnapshot, error)
}

// RecordPersistenceEvidence is an internal runtime/scheduler entrypoint. It is
// intentionally absent from the model MCP surface.
func (s *Service) RecordPersistenceEvidence(
	ctx context.Context,
	actor ActorContext,
	kind PersistenceEvidenceKind,
	commandID string,
) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	key, err := persistenceEvidenceMetadataKey(kind)
	if err != nil {
		return err
	}
	commandID = strings.TrimSpace(commandID)
	if commandID == "" {
		return domainError(ErrorCodeInvalidInput, "persistence evidence command_id is required")
	}
	recorder, ok := s.repository.(persistenceEvidenceRepository)
	if !ok {
		return fmt.Errorf("orchestration repository cannot record persistence evidence")
	}
	runtimeActor := actor
	runtimeActor.ActorKind = protocol.ExecutionActorRuntime
	executionID := persistenceEvidenceExecutionID(actor)
	for attempt := 0; attempt < 3; attempt++ {
		snapshot, admitted, snapshotErr := s.persistenceEvidenceSnapshot(
			ctx,
			actor,
			executionID,
		)
		if snapshotErr != nil {
			return snapshotErr
		}
		if !admitted || snapshot == nil {
			// An unbound round may coexist with a background Execution, and an
			// old bound round may finish after replacement. Neither observation
			// is evidence about the current Execution.
			return nil
		}
		if snapshot.Execution.GoalID != "" ||
			metadataBool(snapshot.Execution.Metadata, key) {
			return nil
		}
		updated, recordErr := recorder.RecordEvidence(ctx, orchestrationstore.RecordEvidenceCommand{
			ExecutionID:              snapshot.Execution.ID,
			ExpectedExecutionVersion: snapshot.Execution.Version,
			MetadataKey:              key,
			Meta: s.commandMeta(
				runtimeActor,
				commandID,
				"record-"+string(kind),
			),
		})
		if recordErr == nil {
			s.invalidateSnapshot(ctx, updated)
			return nil
		}
		if !errors.Is(recordErr, orchestrationstore.ErrVersionConflict) {
			return recordErr
		}
	}
	return orchestrationstore.ErrVersionConflict
}

func persistenceEvidenceExecutionID(actor ActorContext) string {
	if executionID := strings.TrimSpace(actor.ExecutionID); executionID != "" {
		return executionID
	}
	if actor.WorkBinding != nil {
		return strings.TrimSpace(actor.WorkBinding.ExecutionID)
	}
	if actor.ReviewBinding != nil {
		return strings.TrimSpace(actor.ReviewBinding.ExecutionID)
	}
	return ""
}

func (s *Service) persistenceEvidenceSnapshot(
	ctx context.Context,
	actor ActorContext,
	executionID string,
) (*protocol.ExecutionSnapshot, bool, error) {
	current, err := s.repository.FindCurrent(
		ctx,
		strings.TrimSpace(actor.OwnerUserID),
		strings.TrimSpace(actor.SessionKey),
	)
	if err != nil || current == nil {
		return nil, false, err
	}
	if executionID == "" {
		executionID = strings.TrimSpace(current.ID)
	} else if strings.TrimSpace(current.ID) != executionID {
		// The observed round belongs to a predecessor. Never retarget its late
		// runtime event to the successor selected by FindCurrent.
		return nil, false, nil
	}
	snapshot, err := s.repository.GetSnapshot(ctx, executionID)
	if err != nil || snapshot == nil {
		return snapshot, false, err
	}
	if err = authorizeSnapshot(actor, snapshot); err != nil {
		return nil, false, err
	}
	if snapshot.Execution.ID != executionID ||
		(snapshot.Execution.Status != protocol.ExecutionStatusActive &&
			snapshot.Execution.Status != protocol.ExecutionStatusWaiting) {
		return nil, false, nil
	}
	if persistenceEvidenceExecutionID(actor) == "" {
		// Coordinator identity grants control affordances, but an unrelated
		// conversation round is not automatic evidence about a background
		// Execution. Only the root round that created this Execution may add
		// unbound runtime lifecycle evidence.
		if strings.TrimSpace(actor.AgentID) == "" ||
			strings.TrimSpace(actor.AgentID) !=
				strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) ||
			(actor.Role != "" && actor.Role != ExecutionActorCoordinator) ||
			strings.TrimSpace(actor.RootRoundID) == "" ||
			strings.TrimSpace(snapshot.Execution.RootRoundID) !=
				strings.TrimSpace(actor.RootRoundID) {
			return nil, false, nil
		}
	}
	scoped, err := scopeSnapshotToTrustedWorkBinding(actor, executionID, snapshot)
	if err != nil {
		return nil, false, err
	}
	return scoped, true, nil
}

func persistenceEvidenceMetadataKey(kind PersistenceEvidenceKind) (string, error) {
	switch kind {
	case PersistenceEvidenceContextBoundary:
		return ExecutionMetadataContextBoundaryEvidence, nil
	case PersistenceEvidenceScheduledRetry:
		return ExecutionMetadataScheduledRetryEvidence, nil
	default:
		return "", domainError(ErrorCodeInvalidInput, "unknown persistence evidence kind")
	}
}
