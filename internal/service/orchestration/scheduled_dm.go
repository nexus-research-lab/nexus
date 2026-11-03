// INPUT: A private DM queue binding and the new physical round identity.
// OUTPUT: Exact responsibility admission and persisted root activation before runtime work.
// POS: Host admission, not model mutation or a fresh coordination capability.
package orchestration

import (
	"context"
	"errors"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func (s *Service) PrepareScheduledDMRound(ctx context.Context, actor ActorContext) (*protocol.Execution, error) {
	for range 3 {
		snapshot, err := s.GetSnapshot(ctx, actor, actor.ExecutionID)
		if err != nil {
			return nil, err
		}
		if snapshot == nil || snapshot.Execution.ScopeKind != protocol.ExecutionScopeDM || snapshot.Execution.CoordinatorAgentID != actor.AgentID || actor.WorkBinding == nil && actor.ReviewBinding == nil {
			return nil, workBindingMismatch("invalid queued DM responsibility")
		}
		if s.scheduledWork == nil {
			return nil, errors.New("scheduled admission unavailable")
		}
		continuation := actor.WorkBinding != nil && findAssignmentByID(snapshot, actor.WorkBinding.AssignmentID) != nil && findAssignmentByID(snapshot, actor.WorkBinding.AssignmentID).Metadata["user_continuation"] == true
		allowed, err := s.scheduledWork.AutomaticDispatchAllowed(ctx, snapshot.Execution, actor.AgentID)
		if err != nil {
			return nil, err
		}
		if !allowed && !continuation {
			return nil, workBindingMismatch("queued DM responsibility is paused")
		}
		if actor.ReviewBinding != nil {
			return &snapshot.Execution, nil
		}
		binding := actor.WorkBinding
		assignment := findAssignmentByID(snapshot, binding.AssignmentID)
		attempt := findAttemptByID(snapshot, binding.AttemptID)
		if assignment == nil || assignment.Metadata["dispatch_policy"] != true || attempt == nil {
			return nil, workBindingMismatch("DM responsibility is not policy-authorized")
		}
		if attempt.Status == protocol.WorkAttemptStatusRunning {
			if attempt.AgentRoundID != actor.AgentRoundID {
				return nil, workBindingMismatch("DM responsibility already started in another round")
			}
			return &snapshot.Execution, nil
		}
		if attempt.Status != protocol.WorkAttemptStatusPending {
			return nil, workBindingMismatch("queued DM responsibility already ended")
		}
		running := mergeSubmissionRuntime(*attempt, actor, SubmitWorkInput{RuntimeSessionKey: actor.SessionKey})
		running.Status = protocol.WorkAttemptStatusRunning
		updated, err := s.repository.StartAttempt(ctx, store.StartAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAssignmentVersion: assignment.Version, ExpectedAttemptVersion: attempt.Version, Attempt: running, Meta: s.commandMeta(actor, "policy-start:"+attempt.ID, "activate")})
		if errors.Is(err, store.ErrVersionConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		s.invalidateSnapshot(ctx, updated)
		return &updated.Execution, nil
	}
	return nil, store.ErrVersionConflict
}
func (s *Service) RecoveryReviewInputCurrent(ctx context.Context, owner, session string, b protocol.ExecutionReviewBinding) (bool, error) {
	e, err := s.repository.FindCurrent(ctx, owner, session)
	if err != nil || e == nil {
		return false, err
	}
	if e.ID != b.ExecutionID {
		return false, nil
	}
	snapshot, err := s.repository.GetSnapshot(ctx, e.ID)
	if err != nil {
		return false, err
	}
	_, err = authorizeReviewDispatchSnapshot(snapshot, protocol.ExecutionReviewDispatch{ID: b.ReviewDispatchID, ExecutionID: b.ExecutionID, PlanID: b.PlanID, WorkItemID: b.WorkItemID, SpecID: b.SpecID, AssignmentID: b.AssignmentID, SubmissionID: b.SubmissionID, TargetAgentID: b.TargetAgentID}, b.TargetAgentID)
	return err == nil, nil
}
