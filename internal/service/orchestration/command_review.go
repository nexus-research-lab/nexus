// INPUT: Submission 审核与后续协调。
// OUTPUT: 原子命令结果与后续责任动作。
// POS: Orchestration 命令的业务阶段，复用同包授权、版本栅栏与结果投影。
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// ReviewWorkInput 是 Assignment 选定 reviewer 对 immutable Submission 的唯一 decision。
type ReviewWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	SubmissionID     string
	WorkItemID       string
	LogicalKey       string
	Decision         protocol.WorkAcceptanceDecision
	CriteriaResults  []protocol.WorkAcceptanceCriterionResult
	Feedback         string
}

// bindReviewWorkInputFromTrustedResponsibility defaults an exact ReviewBinding
// to its immutable Submission and an exact WorkBinding to its Work Item. This
// keeps the public optional schema honest while preserving strict mismatch
// rejection for every explicit reference.
func bindReviewWorkInputFromTrustedResponsibility(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	input ReviewWorkInput,
) (ReviewWorkInput, error) {
	if actor.ReviewBinding != nil {
		binding := actor.ReviewBinding.Normalized()
		if !binding.Complete() {
			return input, workBindingMismatch("trusted ReviewBinding is incomplete")
		}
		if explicitSubmissionID := strings.TrimSpace(input.SubmissionID); explicitSubmissionID != "" &&
			explicitSubmissionID != binding.SubmissionID {
			return input, workBindingMismatch(
				"explicit submission_id does not match the trusted ReviewBinding",
			)
		}
		if explicitWorkID := strings.TrimSpace(input.WorkItemID); explicitWorkID != "" &&
			explicitWorkID != binding.WorkItemID {
			return input, workBindingMismatch(
				"explicit work_item_id does not match the trusted ReviewBinding",
			)
		}
		if explicitLogicalKey := strings.TrimSpace(input.LogicalKey); explicitLogicalKey != "" {
			boundWork := findWorkItemByID(snapshot, binding.WorkItemID)
			if boundWork == nil {
				return input, workBindingMismatch(
					"trusted ReviewBinding Work Item is outside the active Plan",
				)
			}
			if explicitLogicalKey != strings.TrimSpace(boundWork.LogicalKey) {
				return input, workBindingMismatch(
					"explicit logical_key does not match the trusted ReviewBinding",
				)
			}
		}
		input.SubmissionID = binding.SubmissionID
		input.WorkItemID = binding.WorkItemID
		return input, nil
	}
	if actor.WorkBinding != nil {
		workItemID, logicalKey, err := bindWorkReferenceFromTrustedResponsibility(
			actor,
			snapshot,
			input.WorkItemID,
			input.LogicalKey,
		)
		if err != nil {
			return input, err
		}
		input.WorkItemID = workItemID
		input.LogicalKey = logicalKey
		return input, nil
	}
	if strings.TrimSpace(input.SubmissionID) == "" &&
		strings.TrimSpace(input.WorkItemID) == "" &&
		strings.TrimSpace(input.LogicalKey) == "" {
		return input, domainError(
			ErrorCodeInvalidInput,
			"submission_id, work_item_id or logical_key is required when no trusted binding is present",
		)
	}
	return input, nil
}

// ReviewWork 追加唯一 Acceptance，并由 accepted decision 解锁下游。
func (s *Service) ReviewWork(
	ctx context.Context,
	actor ActorContext,
	input ReviewWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		false,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	if limitErr := validateCriteriaResultsProjectionLimit(input.CriteriaResults); limitErr != nil {
		return RejectedResult(snapshot, limitErr, nil), nil
	}
	input, bindErr := bindReviewWorkInputFromTrustedResponsibility(actor, snapshot, input)
	if bindErr != nil {
		return RejectedResult(snapshot, bindErr, nextActions(snapshot, actor)), nil
	}
	submission, resolveErr := resolveSubmission(snapshot, input.SubmissionID, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nextActions(snapshot, actor)), nil
	}
	if acceptance := acceptanceForSubmission(snapshot, submission.ID); acceptance != nil {
		if acceptance.Decision == protocol.WorkAcceptanceAccepted &&
			len(snapshot.CompletionBlockers) == 0 &&
			snapshot.Execution.Status != protocol.ExecutionStatusCompleted {
			result, completeErr := s.completeAfterReview(
				ctx,
				actor,
				snapshot,
				input.CommandID,
				nil,
			)
			return s.activateReviewContinuationResult(actor, result), completeErr
		}
		return s.activateReviewContinuationResult(
			actor,
			NoOpResult(snapshot, "submission already has an acceptance decision"),
		), nil
	}
	assignment := findAssignmentByID(snapshot, submission.AssignmentID)
	if assignment == nil || assignment.Status != protocol.WorkAssignmentStatusActive {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"submission Assignment is not active",
		), nextActions(snapshot, actor)), nil
	}
	if reviewAuthErr := s.authorizeRoomReviewActor(
		actor,
		snapshot,
		assignment,
		submission,
	); reviewAuthErr != nil {
		return RejectedResult(snapshot, reviewAuthErr, nextActions(snapshot, actor)), nil
	}
	if validationErr := validateReview(snapshot, *submission, input); validationErr != nil {
		return RejectedResult(snapshot, validationErr, nil), nil
	}
	acceptanceID := s.id("acceptance")
	updated, reviewErr := s.repository.Review(ctx, orchestrationstore.ReviewCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Acceptance: protocol.WorkAcceptance{
			ID:              acceptanceID,
			ExecutionID:     submission.ExecutionID,
			PlanID:          submission.PlanID,
			WorkItemID:      submission.WorkItemID,
			SpecID:          submission.SpecID,
			AssignmentID:    submission.AssignmentID,
			SubmissionID:    submission.ID,
			Decision:        input.Decision,
			ReviewerKind:    reviewerKind(actor),
			ReviewerID:      strings.TrimSpace(actor.AgentID),
			CriteriaResults: cloneCriteriaResults(input.CriteriaResults),
			Feedback:        strings.TrimSpace(input.Feedback),
			DecisionRoundID: strings.TrimSpace(actor.RuntimeRoundID),
		},
		Meta: s.commandMeta(actor, input.CommandID, "review"),
	})
	if reviewErr != nil {
		return s.storageMutationResult(snapshot, reviewErr, nextActions(snapshot, actor))
	}
	changed := []string{"acceptance:" + acceptanceID}
	if input.Decision == protocol.WorkAcceptanceAccepted &&
		len(updated.CompletionBlockers) == 0 {
		result, completeErr := s.completeAfterReview(
			ctx,
			actor,
			updated,
			input.CommandID,
			changed,
		)
		return s.activateReviewContinuationResult(actor, result), completeErr
	}
	return s.activateReviewContinuationResult(
		actor,
		AppliedResult(updated, changed, nextActions(updated, actor)),
	), nil
}

func (s *Service) authorizeRoomReviewActor(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
	submission *protocol.WorkSubmission,
) error {
	if snapshot == nil || assignment == nil || submission == nil {
		return domainError(ErrorCodeInvalidInput, "review target is incomplete")
	}
	if snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		normalizeActorKind(actor.ActorKind) != protocol.ExecutionActorAgent {
		return nil
	}
	actorID := strings.TrimSpace(actor.AgentID)
	if actorID == "" || actorID != strings.TrimSpace(assignment.ReturnToAgentID) {
		return domainError(
			ErrorCodeWrongReviewer,
			"Room review is reserved for the reviewer selected by this Assignment",
		)
	}
	if actor.ReviewBinding != nil {
		return nil
	}
	if actor.WorkBinding != nil {
		binding := actor.WorkBinding.Normalized()
		if binding.AssignmentID == assignment.ID &&
			binding.WorkItemID == assignment.WorkItemID &&
			binding.SpecID == assignment.SpecID {
			return nil
		}
		return domainError(
			ErrorCodeReviewBindingRequired,
			"Room self-review must stay inside the trusted Assignment binding",
		)
	}
	if actorID == strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) {
		if err := s.requireRuntimeCoordination(actor, snapshot); err != nil {
			return err
		}
		return nil
	}
	return domainError(
		ErrorCodeReviewBindingRequired,
		"Room review requires the trusted binding for the selected reviewer",
	)
}

func (s *Service) completeAfterReview(
	ctx context.Context,
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	commandID string,
	changed []string,
) (MutationResult, error) {
	if snapshot.Execution.Status == protocol.ExecutionStatusCompleted {
		if len(changed) == 0 {
			return NoOpResult(snapshot, "submission is accepted and execution is completed"), nil
		}
		return AppliedResult(snapshot, changed, nextActions(snapshot, actor)), nil
	}
	completed, err := s.repository.Complete(ctx, orchestrationstore.CompleteCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Meta:                     s.commandMeta(actor, commandID, "complete-after-review"),
	})
	if err != nil {
		if errors.Is(err, orchestrationstore.ErrVersionConflict) ||
			errors.Is(err, orchestrationstore.ErrCompletionBlocked) {
			result := AppliedResult(snapshot, changed, nextActions(snapshot, actor))
			result.Message = "Acceptance committed; backend completion audit will retry from the latest snapshot"
			return result, nil
		}
		return MutationResult{}, err
	}
	changed = append(changed, "execution:"+completed.Execution.ID)
	return AppliedResult(completed, changed, nextActions(completed, actor)), nil
}

func validateReview(
	snapshot *protocol.ExecutionSnapshot,
	submission protocol.WorkSubmission,
	input ReviewWorkInput,
) error {
	if strings.TrimSpace(string(input.Decision)) == "" {
		return domainError(
			ErrorCodeInvalidInput,
			"acceptance decision is required; use accepted, rejected, or changes_requested",
		)
	}
	switch input.Decision {
	case protocol.WorkAcceptanceAccepted,
		protocol.WorkAcceptanceRejected,
		protocol.WorkAcceptanceChangesRequested:
	default:
		return domainError(
			ErrorCodeInvalidInput,
			fmt.Sprintf(
				"invalid acceptance decision %q; use accepted, rejected, or changes_requested",
				input.Decision,
			),
		)
	}
	if input.Decision != protocol.WorkAcceptanceAccepted {
		return nil
	}
	var spec *protocol.WorkItemSpec
	for index := range snapshot.WorkItemSpecs {
		if snapshot.WorkItemSpecs[index].ID == submission.SpecID {
			spec = &snapshot.WorkItemSpecs[index]
			break
		}
	}
	if spec == nil {
		return domainError(ErrorCodeInvalidInput, "Submission spec is missing")
	}
	resultByCriterion := make(map[string]protocol.WorkAcceptanceCriterionResult, len(input.CriteriaResults))
	for _, result := range input.CriteriaResults {
		criterion := strings.TrimSpace(result.Criterion)
		if criterion == "" {
			return domainError(ErrorCodeAcceptanceCriteriaEmpty, "criterion result is empty")
		}
		if _, duplicate := resultByCriterion[criterion]; duplicate {
			return domainError(ErrorCodeInvalidInput, "criterion result appears more than once")
		}
		resultByCriterion[criterion] = result
	}
	for _, criterion := range spec.AcceptanceCriteria {
		result, exists := resultByCriterion[criterion]
		if !exists || !result.Passed {
			return domainError(
				ErrorCodeAcceptanceCriteriaEmpty,
				"accepted decision requires a passing result for every acceptance criterion",
			)
		}
	}
	return nil
}

func acceptanceForSubmission(
	snapshot *protocol.ExecutionSnapshot,
	submissionID string,
) *protocol.WorkAcceptance {
	for index := range snapshot.Acceptances {
		if snapshot.Acceptances[index].SubmissionID == submissionID {
			return &snapshot.Acceptances[index]
		}
	}
	return nil
}

func cloneCriteriaResults(
	values []protocol.WorkAcceptanceCriterionResult,
) []protocol.WorkAcceptanceCriterionResult {
	result := make([]protocol.WorkAcceptanceCriterionResult, len(values))
	for index, value := range values {
		value.Criterion = strings.TrimSpace(value.Criterion)
		value.Evidence = normalizeNonEmptyValues(value.Evidence)
		value.Note = strings.TrimSpace(value.Note)
		result[index] = value
	}
	return result
}

func validateCriteriaResultsProjectionLimit(
	values []protocol.WorkAcceptanceCriterionResult,
) error {
	if err := newProjectionLimitError("criteria_results", len(values), ""); err != nil {
		return err
	}
	for index, value := range values {
		if err := newProjectionLimitError(
			fmt.Sprintf("criteria_results[%d].evidence", index),
			len(value.Evidence),
			"",
		); err != nil {
			return err
		}
	}
	return nil
}

func reviewerKind(actor ActorContext) protocol.WorkReviewerKind {
	if normalizeActorKind(actor.ActorKind) == protocol.ExecutionActorUser {
		return protocol.WorkReviewerUser
	}
	if normalizeActorKind(actor.ActorKind) == protocol.ExecutionActorSystem {
		return protocol.WorkReviewerSystem
	}
	return protocol.WorkReviewerAgent
}

func reviewDispatchInstruction(
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
	submission protocol.WorkSubmission,
	work protocol.WorkItem,
) string {
	base := fmt.Sprintf(
		"Review Submission %s for Work Item %s. Result: %s",
		submission.ID,
		work.LogicalKey,
		submission.ResultSummary,
	)
	if snapshot == nil || assignment == nil {
		return base
	}
	reviewerID := strings.TrimSpace(assignment.ReturnToAgentID)
	coordinatorID := strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
	ownerID := strings.TrimSpace(assignment.OwnerAgentID)
	switch {
	case reviewerID != "" && reviewerID == ownerID:
		return base + " The Assignment selected self-review, so keep the review inside the current Agent responsibility and continue from the recorded decision when possible."
	case reviewerID != "" && coordinatorID != "" && reviewerID != coordinatorID:
		return fmt.Sprintf(
			"%s After recording the decision, send the substantive findings to coordinator %s through Room communication so the collaboration can continue; do not send a status-only handoff and do not wait for a user continuation message.",
			base,
			coordinatorID,
		)
	default:
		return base + " After recording the decision, continue coordination from the resulting state when possible; do not wait for a user continuation message."
	}
}

func needsReviewDispatch(
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
) bool {
	return snapshot != nil &&
		assignment != nil &&
		snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom &&
		strings.TrimSpace(assignment.ReturnToAgentID) !=
			strings.TrimSpace(assignment.OwnerAgentID)
}
