// INPUT: Authenticated owner and exact current Execution/Work Item/root Attempt.
// OUTPUT: Atomic same-owner continuation, durable dispatch and exact request receipts.
// POS: Explicit user continuation replaces failed responsibility without replaying tools or resuming other queues.
package orchestration

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type workRecoveryConsumer interface {
	QueueWorkRecovery(context.Context, protocol.Execution, protocol.ExecutionRecoveryTarget, string) error
	FindWorkRecovery(context.Context, protocol.Execution, string) (*protocol.ExecutionRecoveryTarget, error)
}

func workRecoveryKey(executionID, requestID string) string {
	return fmt.Sprintf("work-recovery:%x", sha256.Sum256([]byte(executionID+"\x00"+requestID)))
}

// GetWorkRecoveryReceipt proves the committed continuation from its transaction event.
func (s *Service) GetWorkRecoveryReceipt(ctx context.Context, owner, executionID, session, requestID string) (*protocol.ExecutionRecoveryReceipt, error) {
	if owner == "" || executionID == "" || session == "" || requestID == "" || len(requestID) > protocol.MaxClientMessageIDBytes {
		return nil, domainError(ErrorCodeInvalidInput, "recovery receipt identity is required")
	}
	execution, err := s.repository.Get(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil || execution.OwnerUserID != owner || execution.SessionKey != session {
		return nil, domainError(ErrorCodeWrongOwner, "recovery receipt is outside the requested owner/session")
	}
	repository, ok := s.repository.(interface {
		FindAssignmentBatchReceipt(context.Context, string, string) (*protocol.ExecutionEvent, error)
	})
	if !ok {
		return nil, fmt.Errorf("continuation receipts unavailable")
	}
	event, err := repository.FindAssignmentBatchReceipt(ctx, executionID, workRecoveryKey(executionID, requestID))
	if err != nil {
		return nil, err
	}
	receipt := &protocol.ExecutionRecoveryReceipt{RequestID: requestID, ExecutionRecoveryTarget: protocol.ExecutionRecoveryTarget{ExecutionID: executionID}}
	if event != nil {
		source, _ := event.Payload["continued_attempt_id"].(string)
		if event.Type != protocol.ExecutionEventWorkTakenOver || source == "" || event.ActorKind != protocol.ExecutionActorUser || event.ActorID != owner {
			return nil, domainError(ErrorCodeInvalidInput, "continuation receipt conflict")
		}
		receipt.WorkItemID, receipt.AttemptID, receipt.Queued = event.WorkItemID, source, true
	} else if consumer, ok := s.rootRecoveryConsumer.(workRecoveryConsumer); ok {
		// Requests admitted before this version keep their original receipt;
		// reconciling a lost response must not create a second, different action.
		target, readErr := consumer.FindWorkRecovery(ctx, *execution, workRecoveryKey(executionID, requestID))
		if readErr != nil {
			return nil, readErr
		}
		if target != nil {
			receipt.ExecutionRecoveryTarget, receipt.Queued = *target, true
		}
	}
	return receipt, nil
}

// RequestWorkRecovery accepts an explicit request without impersonating a runtime actor.
func (s *Service) RequestWorkRecovery(ctx context.Context, owner, executionID string, input protocol.RequestExecutionRecovery) (*protocol.ExecutionRecoveryReceipt, error) {
	if owner == "" || executionID == "" || input.SessionKey == "" || input.RequestID == "" || len(input.RequestID) > protocol.MaxClientMessageIDBytes || input.WorkItemID == "" || input.AttemptID == "" {
		return nil, domainError(ErrorCodeInvalidInput, "exact recovery identity and request_id are required")
	}
	target := protocol.ExecutionRecoveryTarget{ExecutionID: executionID, WorkItemID: input.WorkItemID, AttemptID: input.AttemptID}
	if receipt, err := s.GetWorkRecoveryReceipt(ctx, owner, executionID, input.SessionKey, input.RequestID); err != nil {
		return nil, err
	} else if receipt.Queued {
		if receipt.ExecutionRecoveryTarget != target {
			return nil, domainError(ErrorCodeInvalidInput, "request_id already identifies a different recovery target")
		}
		return receipt, nil
	}
	snapshot, err := s.userRecoverySnapshot(ctx, owner, input.SessionKey, target)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || !canRecoverWorkAttempt(snapshot, target.WorkItemID, target.AttemptID) {
		return nil, domainError(ErrorCodeInvalidInput, "recovery target is no longer a current failed root Attempt; refresh the graph")
	}
	if pending, err := s.pendingDeliveryResult(ctx, executionID, target.AttemptID); err != nil {
		return nil, err
	} else if pending {
		return nil, domainError(ErrorCodeCompletionBlocked, "delivery result is pending reconciliation")
	}
	current := findAssignmentByID(snapshot, findAttemptByID(snapshot, target.AttemptID).AssignmentID)
	work, spec, err := resolvePlanWork(snapshot, target.WorkItemID, "")
	if err != nil {
		return nil, err
	}
	actor := ActorContext{OwnerUserID: owner, SessionKey: input.SessionKey, AgentID: snapshot.Execution.CoordinatorAgentID, Role: ExecutionActorCoordinator, ActorKind: protocol.ExecutionActorSystem, ScopeKind: snapshot.Execution.ScopeKind, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID}
	key := workRecoveryKey(executionID, input.RequestID)
	instruction := fmt.Sprintf("The user requested continuation of this exact task after Attempt %s stopped. First inspect existing files, checkpoints, accepted dependencies and external receipts. Preserve completed work; reconcile unknown effects before retrying. Continue the remaining work and submit it for the original reviewer. Do not create a replacement Execution or restart other tasks.", target.AttemptID)
	replacement, dispatch, attempt, err := s.authorizedPolicyChain(ctx, actor, snapshot, work, spec, DispatchPolicyInput{TargetAgentID: current.OwnerAgentID, ReturnToAgentID: current.ReturnToAgentID, Instruction: instruction}, key)
	if err != nil {
		return nil, err
	}
	admission, ok := s.scheduledWork.(interface {
		ContinuationPauseRevision(context.Context, protocol.Execution, string) (int64, error)
	})
	if !ok {
		return nil, errors.New("continuation admission unavailable")
	}
	pauseRevision, err := admission.ContinuationPauseRevision(ctx, snapshot.Execution, current.OwnerAgentID)
	if err != nil {
		return nil, err
	}
	replacement.Metadata["continuation_pause_revision"] = pauseRevision
	replacement.TakeoverReason = "user requested same-owner continuation"
	replacement.Metadata["user_continuation"] = true
	meta := s.commandMeta(actor, key, "")
	meta.CommandID, meta.ActorKind, meta.ActorID = key, protocol.ExecutionActorUser, owner
	meta.Payload = map[string]any{"continued_attempt_id": target.AttemptID}
	updated, err := s.repository.Takeover(ctx, store.TakeoverCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedCurrentAssignmentVersion: current.Version,
		CurrentAssignmentID: current.ID, Replacement: replacement, Dispatch: dispatch, RootAttempt: &attempt,
		ContinuationInstruction: instruction, Meta: meta,
	})
	if err != nil {
		// A competing identical request may have committed; prove it before reporting success.
		if receipt, readErr := s.GetWorkRecoveryReceipt(ctx, owner, executionID, input.SessionKey, input.RequestID); readErr == nil && receipt.Queued && receipt.ExecutionRecoveryTarget == target {
			return receipt, nil
		}
		return nil, err
	}
	receipt, err := s.GetWorkRecoveryReceipt(ctx, owner, executionID, input.SessionKey, input.RequestID)
	if err != nil {
		return nil, err
	}
	if !receipt.Queued || receipt.ExecutionRecoveryTarget != target {
		return nil, domainError(ErrorCodeInvalidInput, "continuation request identity conflicts")
	}
	s.invalidateSnapshot(ctx, updated)
	s.WakeExecutionDispatch()
	s.WakeOrchestrationRecovery()
	return &protocol.ExecutionRecoveryReceipt{ExecutionRecoveryTarget: target, RequestID: input.RequestID, Queued: true}, nil
}

func (s *Service) userRecoverySnapshot(ctx context.Context, owner, session string, target protocol.ExecutionRecoveryTarget) (*protocol.ExecutionSnapshot, error) {
	execution, err := s.repository.FindCurrent(ctx, owner, session)
	if err != nil || execution == nil {
		return nil, err
	}
	if execution.ID != target.ExecutionID || execution.OwnerUserID != owner || execution.SessionKey != session {
		return nil, domainError(ErrorCodeWrongOwner, "recovery target is outside the current owner/session Execution")
	}
	snapshot, err := s.repository.GetSnapshot(ctx, execution.ID)
	if err != nil || snapshot == nil {
		return nil, err
	}
	if snapshot.Execution.OwnerUserID != owner || snapshot.Execution.SessionKey != session {
		return nil, domainError(ErrorCodeWrongOwner, "recovery snapshot is outside the requested owner/session")
	}
	return snapshot, nil
}

// RecoveryTargetCurrent drops queued requests after takeover, replan or completion.
func (s *Service) RecoveryTargetCurrent(ctx context.Context, owner, session string, target protocol.ExecutionRecoveryTarget) (bool, error) {
	snapshot, err := s.userRecoverySnapshot(ctx, owner, session, target)
	var domainErr *DomainError
	if errors.As(err, &domainErr) && domainErr.Code == ErrorCodeWrongOwner {
		return false, nil
	}
	return snapshot != nil && canRecoverWorkAttempt(snapshot, target.WorkItemID, target.AttemptID), err
}

func canRecoverWorkAttempt(snapshot *protocol.ExecutionSnapshot, workID, attemptID string) bool {
	if snapshot == nil || snapshot.Plan == nil || snapshot.Execution.Status != protocol.ExecutionStatusActive {
		return false
	}
	attempt := findAttemptByID(snapshot, attemptID)
	if attempt == nil || attempt.ParentAttemptID != "" || attempt.WorkItemID != workID || attempt.PlanID != snapshot.Plan.ID {
		return false
	}
	_, spec, err := resolvePlanWork(snapshot, workID, "")
	if err != nil || spec.ID != attempt.SpecID {
		return false
	}
	switch attempt.Status {
	case protocol.WorkAttemptStatusFailed, protocol.WorkAttemptStatusInterrupted, protocol.WorkAttemptStatusTimedOut:
	default:
		return false
	}
	if latestUnreviewedSubmission(snapshot, workID) != nil {
		return false
	}
	for _, child := range snapshot.Attempts {
		if child.AssignmentID == attempt.AssignmentID && (child.Status == protocol.WorkAttemptStatusRunning || child.Status == protocol.WorkAttemptStatusPending) {
			return false
		}
	}
	assignment := findAssignmentByID(snapshot, attempt.AssignmentID)
	if assignment == nil || !currentAssignment(*assignment) {
		return false
	}
	latest := latestRootAttemptForAssignment(snapshot, assignment.ID)
	if latest == nil || latest.ID != attempt.ID {
		return false
	}
	for _, submission := range snapshot.Submissions {
		if submission.AttemptID == attempt.ID {
			return false
		}
	}
	return true
}
