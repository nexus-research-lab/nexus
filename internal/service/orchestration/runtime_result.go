// INPUT: Exact host-issued work/review binding and an explicit structured result.
// OUTPUT: Durable intake, domain-verified outcome and restart reconciliation.
// POS: Result hooks reuse SubmitWork/ReviewWork; runtime completion is never evidence of acceptance.
package orchestration

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type runtimeResultRepository interface {
	PutRuntimeResultReceipt(context.Context, store.RuntimeResultReceipt) (*store.RuntimeResultReceipt, error)
	GetRuntimeResultReceipt(context.Context, string) (*store.RuntimeResultReceipt, error)
	ListPendingRuntimeResults(context.Context, time.Time, int) ([]store.RuntimeResultReceipt, error)
	SetRuntimeResultState(context.Context, string, string, string, time.Time) error
	HasPendingDeliveryResult(context.Context, string, string) (bool, error)
}

type RuntimeResultOutcome struct{ ReceiptID, State, Reason string }

func runtimeResultIdentity(actor ActorContext, declaration protocol.ExecutionResultDeclaration) (string, string, string, error) {
	if actor.PlanMode || actor.ObservationOnly || actor.Role == ExecutionActorSubagent || actor.OwnerUserID == "" || actor.AgentID == "" || actor.AgentRoundID == "" || actor.RuntimeRoundID == "" || declaration.Version != 1 || declaration.RoundID != actor.AgentRoundID {
		return "", "", "", workBindingMismatch("result requires its exact authorized physical round")
	}
	if actor.WorkBinding != nil && actor.ReviewBinding == nil && declaration.Delivery != nil && declaration.Review == nil && actor.WorkBinding.Complete() {
		b := actor.WorkBinding
		return b.ExecutionID, "delivery", b.AttemptID, nil
	}
	if actor.ReviewBinding != nil && actor.WorkBinding == nil && declaration.Review != nil && declaration.Delivery == nil && actor.ReviewBinding.Complete() && actor.ReviewBinding.TargetAgentID == actor.AgentID {
		b := actor.ReviewBinding
		return b.ExecutionID, "review", b.SubmissionID, nil
	}
	return "", "", "", workBindingMismatch("result kind must match one complete host-issued binding")
}

// AcceptRuntimeResult acknowledges only a committed intake. Its state separately
// states whether the domain result has been proven, rejected, or awaits reconciliation.
func (s *Service) AcceptRuntimeResult(ctx context.Context, actor ActorContext, declaration protocol.ExecutionResultDeclaration) (RuntimeResultOutcome, error) {
	executionID, kind, target, err := runtimeResultIdentity(actor, declaration)
	if err != nil {
		return RuntimeResultOutcome{}, err
	}
	repository, ok := s.repository.(runtimeResultRepository)
	if !ok {
		return RuntimeResultOutcome{}, errors.New("runtime result intake is unavailable")
	}
	snapshot, err := s.repository.GetSnapshot(ctx, executionID)
	if err != nil {
		return RuntimeResultOutcome{}, err
	}
	if err = authorizeSnapshot(actor, snapshot); err != nil {
		return RuntimeResultOutcome{}, err
	}
	if err = authorizeRuntimeResultIdentity(actor, snapshot); err != nil {
		return RuntimeResultOutcome{}, err
	}
	// Remove observational, process-local fields before freezing the authority.
	actor.SubagentCommandAuthority = nil
	actor.ExecutionID = executionID
	actorJSON, err := json.Marshal(actor)
	if err != nil {
		return RuntimeResultOutcome{}, err
	}
	resultJSON, err := json.Marshal(declaration)
	if err != nil {
		return RuntimeResultOutcome{}, err
	}
	if len(resultJSON) > 64*1024 {
		return RuntimeResultOutcome{}, domainError(ErrorCodeInvalidInput, "result exceeds the intake limit")
	}
	id := fmt.Sprintf("runtime-result:%x", sha256.Sum256([]byte(actor.OwnerUserID+"\x00"+executionID+"\x00"+kind+"\x00"+target)))
	stored, err := repository.GetRuntimeResultReceipt(ctx, id)
	if err != nil {
		return RuntimeResultOutcome{}, err
	}
	if stored != nil {
		if stored.ActorJSON != string(actorJSON) || stored.ResultJSON != string(resultJSON) {
			return RuntimeResultOutcome{}, store.ErrRuntimeResultConflict
		}
		return s.acceptedRuntimeResult(ctx, *stored)
	}
	matched, conflict := runtimeResultProof(snapshot, actor, declaration)
	if conflict {
		return RuntimeResultOutcome{}, store.ErrRuntimeResultConflict
	}
	if !matched {
		if err := authorizePendingRuntimeResult(actor, snapshot); err != nil {
			return RuntimeResultOutcome{}, err
		}
		if _, err := scopeSnapshotToTrustedWorkBinding(actor, executionID, snapshot); err != nil {
			return RuntimeResultOutcome{}, err
		}
		if declaration.Delivery != nil && declaration.Delivery.ResultSummary == "" {
			return RuntimeResultOutcome{}, domainError(ErrorCodeInvalidInput, "delivery result_summary is required")
		}
		if delivery := declaration.Delivery; delivery != nil {
			if err := newProjectionLimitError("result_refs", len(delivery.ResultRefs), ""); err != nil {
				return RuntimeResultOutcome{}, err
			}
			if err := newProjectionLimitError("submission_evidence", len(delivery.Evidence), ""); err != nil {
				return RuntimeResultOutcome{}, err
			}
		}
		if declaration.Review != nil {
			if err := validateCriteriaResultsProjectionLimit(declaration.Review.CriteriaResults); err != nil {
				return RuntimeResultOutcome{}, err
			}
			submission, err := resolveSubmission(snapshot, actor.ReviewBinding.SubmissionID, actor.ReviewBinding.WorkItemID, "")
			if err != nil {
				return RuntimeResultOutcome{}, err
			}
			if err := validateReview(snapshot, *submission, ReviewWorkInput{Decision: declaration.Review.Decision, CriteriaResults: declaration.Review.CriteriaResults, Feedback: declaration.Review.Feedback}); err != nil {
				return RuntimeResultOutcome{}, err
			}
		}
	}
	stored, err = repository.PutRuntimeResultReceipt(ctx, store.RuntimeResultReceipt{ID: id, ExecutionID: executionID, OwnerUserID: actor.OwnerUserID, Kind: kind, TargetID: target, ActorJSON: string(actorJSON), ResultJSON: string(resultJSON)})
	if err != nil {
		return RuntimeResultOutcome{}, err
	}
	s.WakeOrchestrationRecovery()
	return s.acceptedRuntimeResult(ctx, *stored)
}

func (s *Service) acceptedRuntimeResult(ctx context.Context, receipt store.RuntimeResultReceipt) (RuntimeResultOutcome, error) {
	result, err := s.applyRuntimeResult(ctx, receipt)
	if err == nil {
		return result, nil
	}
	// The intake receipt is already committed. A domain/response failure cannot
	// erase that fact or invite another declaration; background recovery owns it.
	s.WakeOrchestrationRecovery()
	return RuntimeResultOutcome{ReceiptID: receipt.ID, State: receipt.State, Reason: "reconcile_required"}, nil
}

// authorizeRuntimeResultIdentity also works after an explicit tool completed the
// result: immutable ownership must match before looking at idempotency evidence.
func authorizeRuntimeResultIdentity(actor ActorContext, snapshot *protocol.ExecutionSnapshot) error {
	if snapshot == nil {
		return workBindingMismatch("result Execution disappeared")
	}
	if actor.ScopeKind != snapshot.Execution.ScopeKind || (actor.ExecutionID != "" && actor.ExecutionID != snapshot.Execution.ID) ||
		(snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom && (actor.RoomID != snapshot.Execution.RoomID || actor.ConversationID != snapshot.Execution.ConversationID)) {
		return workBindingMismatch("result must belong to its exact host execution scope")
	}
	if b := actor.WorkBinding; b != nil {
		a := findAssignmentByID(snapshot, b.AssignmentID)
		attempt := findAttemptByID(snapshot, b.AttemptID)
		if a == nil || attempt == nil || a.ExecutionID != b.ExecutionID || a.PlanID != b.PlanID || a.WorkItemID != b.WorkItemID || a.SpecID != b.SpecID || a.OwnerAgentID != actor.AgentID ||
			attempt.ExecutionID != b.ExecutionID || attempt.PlanID != b.PlanID || attempt.WorkItemID != b.WorkItemID || attempt.SpecID != b.SpecID || attempt.DispatchID != b.DispatchID ||
			attempt.AssignmentID != a.ID || attempt.ParentAttemptID != "" || attempt.ExecutorKind != protocol.AttemptExecutorAgent || attempt.ExecutorAgentID != actor.AgentID {
			return workBindingMismatch("delivery binding no longer identifies its exact owner and root Attempt")
		}
		if (attempt.AgentRoundID != "" && attempt.AgentRoundID != actor.AgentRoundID) || (attempt.RuntimeRoundID != "" && attempt.RuntimeRoundID != actor.RuntimeRoundID) {
			return workBindingMismatch("delivery belongs to a different physical Attempt round")
		}
		return nil
	}
	b := actor.ReviewBinding
	if b == nil {
		return workBindingMismatch("result binding missing")
	}
	for _, dispatch := range snapshot.ReviewDispatches {
		if dispatch.ID == b.ReviewDispatchID && dispatch.ExecutionID == b.ExecutionID && dispatch.PlanID == b.PlanID && dispatch.WorkItemID == b.WorkItemID && dispatch.SpecID == b.SpecID && dispatch.AssignmentID == b.AssignmentID && dispatch.SubmissionID == b.SubmissionID && dispatch.TargetAgentID == actor.AgentID {
			return nil
		}
	}
	return workBindingMismatch("review binding no longer identifies its exact selected reviewer")
}

// A receipt may outlive its round, but cannot move its declaration to a newer
// Attempt or revive work stopped/replaced after intake. Existing proof is read first.
func authorizePendingRuntimeResult(actor ActorContext, snapshot *protocol.ExecutionSnapshot) error {
	if !snapshot.Execution.Status.Current() {
		return terminalExecutionError()
	}
	if b := actor.WorkBinding; b != nil {
		if err := authorizeStructuredRoomWorkBinding(actor, b.ExecutionID, snapshot, *b); err != nil {
			return err
		}
		latest := latestRootAttemptForAssignment(snapshot, b.AssignmentID)
		if latest == nil || latest.ID != b.AttemptID {
			return workBindingMismatch("result cannot move to a newer root Attempt")
		}
		// SubmitWork would otherwise open a successor Attempt for an ended root,
		// recording the delivery under an identity this receipt can never prove.
		if latest.Status != protocol.WorkAttemptStatusPending && latest.Status != protocol.WorkAttemptStatusRunning {
			return workBindingMismatch("result's root Attempt already ended")
		}
	}
	return nil
}

func runtimeResultProof(snapshot *protocol.ExecutionSnapshot, actor ActorContext, result protocol.ExecutionResultDeclaration) (matched, conflict bool) {
	if snapshot == nil {
		return false, false
	}
	if result.Delivery != nil {
		for _, submission := range snapshot.Submissions {
			if submission.AttemptID != actor.WorkBinding.AttemptID {
				continue
			}
			expected := *result.Delivery
			actual := protocol.ExecutionDeliveryResult{ResultSummary: submission.ResultSummary, ResultRefs: submission.ResultRefs, Evidence: submission.Evidence}
			// JSON equates omitted and empty optional arrays without losing order.
			a, _ := json.Marshal(actual)
			b, _ := json.Marshal(expected)
			return string(a) == string(b), string(a) != string(b)
		}
	} else if result.Review != nil {
		if acceptance := acceptanceForSubmission(snapshot, actor.ReviewBinding.SubmissionID); acceptance != nil {
			actual := protocol.ExecutionReviewResult{Decision: acceptance.Decision, CriteriaResults: acceptance.CriteriaResults, Feedback: acceptance.Feedback}
			a, _ := json.Marshal(actual)
			b, _ := json.Marshal(result.Review)
			return string(a) == string(b), string(a) != string(b)
		}
	}
	return false, false
}

func (s *Service) applyRuntimeResult(ctx context.Context, receipt store.RuntimeResultReceipt) (RuntimeResultOutcome, error) {
	result := RuntimeResultOutcome{ReceiptID: receipt.ID, State: receipt.State, Reason: receipt.Reason}
	if receipt.State != "pending" {
		return result, nil
	}
	repository := s.repository.(runtimeResultRepository)
	var actor ActorContext
	var declaration protocol.ExecutionResultDeclaration
	if err := json.Unmarshal([]byte(receipt.ActorJSON), &actor); err != nil {
		return result, err
	}
	if err := json.Unmarshal([]byte(receipt.ResultJSON), &declaration); err != nil {
		return result, err
	}
	settle := func(state, reason string) (RuntimeResultOutcome, error) {
		if state == "rejected" && receipt.Kind == "delivery" {
			if err := s.finishRejectedRuntimeResult(ctx, actor, receipt); err != nil {
				return result, err
			}
		}
		err := repository.SetRuntimeResultState(ctx, receipt.ID, state, reason, s.currentTime().Add(5*time.Second))
		if err != nil {
			return result, err
		}
		current, err := repository.GetRuntimeResultReceipt(ctx, receipt.ID)
		if err != nil || current == nil {
			return result, err
		}
		return RuntimeResultOutcome{ReceiptID: receipt.ID, State: current.State, Reason: current.Reason}, nil
	}
	for range 3 {
		snapshot, err := s.repository.GetSnapshot(ctx, receipt.ExecutionID)
		if err != nil {
			return result, err
		}
		if err := authorizeSnapshot(actor, snapshot); err != nil {
			return settle("rejected", "stale_binding")
		}
		if err := authorizeRuntimeResultIdentity(actor, snapshot); err != nil {
			return settle("rejected", "stale_binding")
		}
		matched, conflict := runtimeResultProof(snapshot, actor, declaration)
		if matched {
			return settle("applied", "")
		}
		if conflict {
			return settle("rejected", "result_conflict")
		}
		if err := authorizePendingRuntimeResult(actor, snapshot); err != nil {
			return settle("rejected", "stale_binding")
		}
		var mutation MutationResult
		if declaration.Delivery != nil {
			delivery := declaration.Delivery
			mutation, err = s.SubmitWork(ctx, actor, SubmitWorkInput{ExecutionID: receipt.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: receipt.ID, ResultSummary: delivery.ResultSummary, ResultRefs: delivery.ResultRefs, Evidence: delivery.Evidence})
		} else {
			review := declaration.Review
			mutation, err = s.ReviewWork(ctx, actor, ReviewWorkInput{ExecutionID: receipt.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: receipt.ID, Decision: review.Decision, CriteriaResults: review.CriteriaResults, Feedback: review.Feedback})
		}
		if err != nil {
			return result, err
		}
		// Even no_op must be proven against exact stored content: a tool may
		// have won with a different delivery or review decision.
		if matched, conflict := runtimeResultProof(mutation.Snapshot, actor, declaration); matched {
			return settle("applied", "")
		} else if conflict {
			return settle("rejected", "result_conflict")
		}
		if mutation.Outcome == MutationRejected && mutation.ReasonCode == ErrorCodeStaleExecution {
			continue
		}
		// Any other outcome without proof (rejected, superseded or no_op) cannot
		// improve by replaying the same command; only a concurrent change can.
		latest, readErr := s.repository.GetSnapshot(ctx, receipt.ExecutionID)
		if readErr != nil {
			return result, readErr
		}
		if matched, conflict := runtimeResultProof(latest, actor, declaration); matched {
			return settle("applied", "")
		} else if conflict {
			return settle("rejected", "result_conflict")
		}
		if latest != nil && snapshot.Execution.Version == latest.Execution.Version {
			reason := string(mutation.ReasonCode)
			if reason == "" {
				reason = "not_applied"
			}
			return settle("rejected", reason)
		}
	}
	// Bounded: a receipt that keeps losing races cannot defer its root's
	// terminal settlement forever.
	if !receipt.CreatedAt.IsZero() && s.currentTime().Sub(receipt.CreatedAt) > runtimeResultReconcileWindow {
		return settle("rejected", "reconcile_expired")
	}
	return settle("pending", "reconcile_required")
}

const runtimeResultReconcileWindow = 30 * time.Minute

// A round may have ended while intake was pending. A rejected declaration must
// not leave its root running forever after terminal settlement was deferred.
func (s *Service) finishRejectedRuntimeResult(ctx context.Context, actor ActorContext, receipt store.RuntimeResultReceipt) error {
	snapshot, err := s.repository.GetSnapshot(ctx, receipt.ExecutionID)
	if err != nil || snapshot == nil {
		return err
	}
	if !snapshot.Execution.Status.Current() {
		return nil
	}
	attempt := findAttemptByID(snapshot, receipt.TargetID)
	if attempt == nil || (attempt.Status != protocol.WorkAttemptStatusPending && attempt.Status != protocol.WorkAttemptStatusRunning) {
		return nil
	}
	terminal := *attempt
	terminal.Status = protocol.WorkAttemptStatusFailed
	terminal.FailureReason = "structured_result_rejected: inspect the authoritative result before recovery"
	updated, err := s.repository.FinishAttempt(ctx, store.FinishAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAttemptVersion: attempt.Version, Attempt: terminal, Meta: s.commandMeta(actor, receipt.ID+":rejected", "result-rejected")})
	if err == nil {
		s.invalidateSnapshot(ctx, updated)
	}
	return err
}

// ReconcileRuntimeResults recovers intake even after the originating runtime exited.
func (s *Service) ReconcileRuntimeResults(ctx context.Context, limit int) (int, error) {
	repository, ok := s.repository.(runtimeResultRepository)
	if !ok {
		return 0, errors.New("runtime result repository unavailable")
	}
	receipts, err := repository.ListPendingRuntimeResults(ctx, s.currentTime(), limit)
	if err != nil {
		return 0, err
	}
	var failures []error
	for _, receipt := range receipts {
		if _, err := s.applyRuntimeResult(ctx, receipt); err != nil {
			failures = append(failures, err)
			if retryErr := repository.SetRuntimeResultState(ctx, receipt.ID, "pending", "reconcile_required", s.currentTime().Add(5*time.Second)); retryErr != nil {
				failures = append(failures, retryErr)
			}
		}
	}
	return len(receipts), errors.Join(failures...)
}

func (s *Service) pendingDeliveryResult(ctx context.Context, executionID, attemptID string) (bool, error) {
	if repository, ok := s.repository.(runtimeResultRepository); ok {
		return repository.HasPendingDeliveryResult(ctx, executionID, attemptID)
	}
	return false, nil
}
