// INPUT: trusted Goal objective transition with exact old/new revision and reserved successor identity.
// OUTPUT: terminal old Goal-bound WorkGraph or replayed terminal snapshot, plus post-commit session invalidation.
// POS: internal application-service bridge used only by the Goal retarget coordinator.
package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// GoalRevisionSupersedeInput identifies the predecessor graph of a Goal
// objective revision. Actor fields are trusted application context used only
// for audit metadata.
type GoalRevisionSupersedeInput struct {
	ExecutionID              string
	ExpectedOwnerUserID      string
	GoalID                   string
	OldGoalObjectiveRevision int64
	NewGoalObjectiveRevision int64
	SuccessorExecutionID     string
	CommandID                string
	Reason                   string
	Source                   protocol.GoalUpdateSource
	ActorID                  string
	RootRoundID              string
}

// ValidateGoalRevisionOwner performs the read-only owner fence required before
// an HTTP user mutation prepares durable Goal transition state. A false result
// means the reserved Execution identity has not materialized.
func (s *Service) ValidateGoalRevisionOwner(
	ctx context.Context,
	executionID string,
	goalID string,
	goalObjectiveRevision int64,
	expectedOwnerUserID string,
) (bool, error) {
	if s == nil || s.repository == nil {
		return false, fmt.Errorf("orchestration repository is nil")
	}
	executionID = strings.TrimSpace(executionID)
	goalID = strings.TrimSpace(goalID)
	expectedOwnerUserID = strings.TrimSpace(expectedOwnerUserID)
	if executionID == "" || goalID == "" || goalObjectiveRevision <= 0 ||
		expectedOwnerUserID == "" {
		return false, domainError(
			ErrorCodeInvalidInput,
			"complete Goal revision owner identity is required",
		)
	}
	snapshot, err := s.repository.GetSnapshot(ctx, executionID)
	if err != nil {
		return false, err
	}
	if snapshot == nil {
		return false, nil
	}
	if strings.TrimSpace(snapshot.Execution.OwnerUserID) != expectedOwnerUserID ||
		strings.TrimSpace(snapshot.Execution.GoalID) != goalID ||
		snapshot.Execution.GoalObjectiveRevision != goalObjectiveRevision {
		return false, domainError(
			ErrorCodeGoalBindingConflict,
			"Goal objective transition owner or revision does not match its Execution",
		)
	}
	return true, nil
}

// SupersedeGoalRevision terminalizes the old graph before the Goal service
// commits the new canonical objective.
func (s *Service) SupersedeGoalRevision(
	ctx context.Context,
	input GoalRevisionSupersedeInput,
) (*protocol.ExecutionSnapshot, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	executionID := strings.TrimSpace(input.ExecutionID)
	if executionID == "" {
		return nil, nil
	}
	actorKind := protocol.ExecutionActorSystem
	switch input.Source {
	case protocol.GoalUpdateSourceModel:
		actorKind = protocol.ExecutionActorAgent
	case protocol.GoalUpdateSourceUser:
		actorKind = protocol.ExecutionActorUser
	}
	actor := ActorContext{
		AgentID:     strings.TrimSpace(input.ActorID),
		ActorKind:   actorKind,
		RootRoundID: strings.TrimSpace(input.RootRoundID),
	}
	snapshot, err := s.repository.GetSnapshot(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		fenced, fenceErr := s.repository.FenceGoalExecutionIdentity(
			ctx,
			orchestrationstore.FenceGoalExecutionIdentityCommand{
				ExecutionID:           executionID,
				ExpectedOwnerUserID:   strings.TrimSpace(input.ExpectedOwnerUserID),
				GoalID:                strings.TrimSpace(input.GoalID),
				GoalObjectiveRevision: input.OldGoalObjectiveRevision,
				SuccessorExecutionID:  strings.TrimSpace(input.SuccessorExecutionID),
				Meta:                  s.commandMeta(actor, input.CommandID, "goal-retarget-fence"),
			},
		)
		if fenceErr != nil {
			return nil, fenceErr
		}
		if fenced {
			return nil, nil
		}
		snapshot, err = s.repository.GetSnapshot(ctx, executionID)
		if err != nil {
			return nil, err
		}
		if snapshot == nil {
			return nil, domainError(
				ErrorCodeGoalBindingConflict,
				"Goal Execution materialization won its identity claim but no Execution is readable",
			)
		}
	}
	if strings.TrimSpace(snapshot.Execution.GoalID) != strings.TrimSpace(input.GoalID) ||
		snapshot.Execution.GoalObjectiveRevision != input.OldGoalObjectiveRevision {
		return nil, domainError(
			ErrorCodeGoalBindingConflict,
			"old Execution does not match the Goal objective revision being retargeted",
		)
	}
	if expectedOwner := strings.TrimSpace(input.ExpectedOwnerUserID); expectedOwner != "" &&
		strings.TrimSpace(snapshot.Execution.OwnerUserID) != expectedOwner {
		return nil, domainError(
			ErrorCodeGoalBindingConflict,
			"old Execution belongs to another owner",
		)
	}
	updated, supersedeErr := s.repository.SupersedeGoalRevision(ctx, orchestrationstore.SupersedeGoalRevisionCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedOwnerUserID:      strings.TrimSpace(input.ExpectedOwnerUserID),
		GoalID:                   strings.TrimSpace(input.GoalID),
		OldGoalObjectiveRevision: input.OldGoalObjectiveRevision,
		NewGoalObjectiveRevision: input.NewGoalObjectiveRevision,
		SuccessorExecutionID:     strings.TrimSpace(input.SuccessorExecutionID),
		Reason:                   strings.TrimSpace(input.Reason),
		Meta:                     s.commandMeta(actor, input.CommandID, "goal-retarget-supersede"),
	})
	if supersedeErr == nil {
		s.invalidateSnapshot(ctx, updated)
	}
	return updated, supersedeErr
}
