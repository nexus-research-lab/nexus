// INPUT: 可信责任绑定与工作提交。
// OUTPUT: 原子命令结果与后续责任动作。
// POS: Orchestration 命令的业务阶段，复用同包授权、版本栅栏与结果投影。
package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// SubmitWorkInput 是 Assignment owner 对当前 immutable spec 的完成声明。
type SubmitWorkInput struct {
	ExecutionID       string
	SnapshotRevision  int64
	CommandID         string
	WorkItemID        string
	LogicalKey        string
	AssignmentID      string
	ResultSummary     string
	ResultRefs        []string
	Evidence          []string
	RuntimeSessionKey string
	RoomSessionID     string
	SDKSessionID      string
	ToolUseID         string
}

// SubmitWork 透明启动并成功结束当前 root Attempt，然后记录 Submission。
func (s *Service) SubmitWork(
	ctx context.Context,
	actor ActorContext,
	input SubmitWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	input.CommandID = strings.TrimSpace(input.CommandID)
	input.ResultSummary = strings.TrimSpace(input.ResultSummary)
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
	if input.CommandID == "" || input.ResultSummary == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id and result_summary are required",
		), nil), nil
	}
	input, bindingErr := bindSubmitWorkInputFromTrustedResponsibility(actor, snapshot, input)
	if bindingErr != nil {
		return RejectedResult(snapshot, bindingErr, nil), nil
	}
	actorAgentID := strings.TrimSpace(actor.AgentID)
	for _, collection := range []struct {
		field string
		count int
	}{
		{field: "result_refs", count: len(input.ResultRefs)},
		{field: "submission_evidence", count: len(input.Evidence)},
	} {
		if limitErr := newProjectionLimitError(collection.field, collection.count, ""); limitErr != nil {
			return RejectedResult(snapshot, limitErr, nil), nil
		}
	}
	work, _, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	assignment := selectAssignment(snapshot, work.ID, input.AssignmentID)
	if assignment == nil || !currentAssignment(*assignment) {
		if submission := latestUnreviewedSubmission(snapshot, work.ID); submission != nil &&
			submission.SubmitterAgentID == actorAgentID {
			return NoOpResult(snapshot, "work is already submitted and awaiting review"), nil
		}
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"no current Assignment exists for this actor and Work Item",
			work.LogicalKey,
			"",
		), nil), nil
	}
	if assignment.OwnerAgentID != actorAgentID {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeWrongOwner,
			"only the current Assignment owner may submit work",
			work.LogicalKey,
			assignment.OwnerAgentID,
		), nil), nil
	}
	if submission := latestUnreviewedSubmission(snapshot, work.ID); submission != nil {
		return NoOpResult(snapshot, "work is already submitted and awaiting review"), nil
	}
	state := findStateByWorkID(snapshot, work.ID)
	if state == nil || state.CurrentSpecID != assignment.SpecID {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"current Work Item state/spec fence is missing",
		), nil), nil
	}
	if state.Status != protocol.WorkItemStatusOpen {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"Work Item must be resumed before it can create a new Attempt or Submission",
			work.LogicalKey,
			string(state.Status),
		), nextActions(snapshot, actor)), nil
	}

	attempt := currentOrSucceededAttempt(snapshot, assignment.ID)
	if attempt == nil {
		created := protocol.WorkAttempt{
			ID:              s.id("attempt"),
			ExecutionID:     assignment.ExecutionID,
			PlanID:          assignment.PlanID,
			WorkItemID:      assignment.WorkItemID,
			SpecID:          assignment.SpecID,
			AssignmentID:    assignment.ID,
			ExecutorKind:    protocol.AttemptExecutorAgent,
			ExecutorAgentID: assignment.OwnerAgentID,
			Status:          protocol.WorkAttemptStatusRunning,
		}
		updated, startErr := s.repository.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    0,
			Attempt:                   mergeSubmissionRuntime(created, actor, input),
			Meta:                      s.commandMeta(actor, input.CommandID, "submit-start"),
		})
		if startErr != nil {
			return s.storageMutationResult(snapshot, startErr, nextActions(snapshot, actor))
		}
		s.invalidateSnapshot(ctx, updated)
		snapshot = updated
		assignment = findAssignmentByID(snapshot, assignment.ID)
		attempt = currentOrSucceededAttempt(snapshot, assignment.ID)
	} else if attempt.Status == protocol.WorkAttemptStatusPending {
		updated, startErr := s.repository.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    attempt.Version,
			Attempt:                   mergeSubmissionRuntime(*attempt, actor, input),
			Meta:                      s.commandMeta(actor, input.CommandID, "submit-start"),
		})
		if startErr != nil {
			return s.storageMutationResult(snapshot, startErr, nextActions(snapshot, actor))
		}
		s.invalidateSnapshot(ctx, updated)
		snapshot = updated
		assignment = findAssignmentByID(snapshot, assignment.ID)
		attempt = findAttemptByID(snapshot, attempt.ID)
	}
	if assignment == nil || attempt == nil {
		return MutationResult{}, fmt.Errorf("repository returned an incomplete Assignment/Attempt snapshot")
	}
	if attempt.Status == protocol.WorkAttemptStatusRunning {
		terminal := mergeSubmissionRuntime(*attempt, actor, input)
		terminal.Status = protocol.WorkAttemptStatusSucceeded
		updated, finishErr := s.repository.FinishAttempt(ctx, orchestrationstore.FinishAttemptCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			ExpectedAttemptVersion:   attempt.Version,
			Attempt:                  terminal,
			Meta:                     s.commandMeta(actor, input.CommandID, "submit-finish"),
		})
		if finishErr != nil {
			return s.storageMutationResult(snapshot, finishErr, nextActions(snapshot, actor))
		}
		s.invalidateSnapshot(ctx, updated)
		snapshot = updated
		assignment = findAssignmentByID(snapshot, assignment.ID)
		attempt = findAttemptByID(snapshot, attempt.ID)
	}
	if assignment == nil || attempt == nil || attempt.Status != protocol.WorkAttemptStatusSucceeded {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeDuplicateAttempt,
			"current Attempt is terminal without success; refresh Execution context and use an allowed coordinator recovery action to create a fresh Attempt before submitting again",
			work.LogicalKey,
			"",
		), nextActions(snapshot, actor)), nil
	}
	submissionRecord := protocol.WorkSubmission{
		ID:               s.id("submission"),
		ExecutionID:      assignment.ExecutionID,
		PlanID:           assignment.PlanID,
		WorkItemID:       assignment.WorkItemID,
		SpecID:           assignment.SpecID,
		AssignmentID:     assignment.ID,
		AttemptID:        attempt.ID,
		SubmitterAgentID: actorAgentID,
		ResultSummary:    input.ResultSummary,
		ResultRefs:       normalizeNonEmptyValues(input.ResultRefs),
		Evidence:         normalizeNonEmptyValues(input.Evidence),
	}
	var reviewDispatch *protocol.ExecutionReviewDispatch
	if needsReviewDispatch(snapshot, assignment) {
		reviewDispatch = &protocol.ExecutionReviewDispatch{
			ID:            s.id("review_dispatch"),
			ExecutionID:   assignment.ExecutionID,
			PlanID:        assignment.PlanID,
			WorkItemID:    assignment.WorkItemID,
			SpecID:        assignment.SpecID,
			AssignmentID:  assignment.ID,
			SubmissionID:  submissionRecord.ID,
			DedupeKey:     "review-return:" + submissionRecord.ID,
			TargetAgentID: strings.TrimSpace(assignment.ReturnToAgentID),
			Status:        protocol.ExecutionReviewDispatchStatusPending,
			Instruction: reviewDispatchInstruction(
				snapshot,
				assignment,
				submissionRecord,
				work,
			),
		}
	}
	updated, submitErr := s.repository.Submit(ctx, orchestrationstore.SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submissionRecord,
		ReviewDispatch:            reviewDispatch,
		Meta:                      s.commandMeta(actor, input.CommandID, "submit-record"),
	})
	if submitErr != nil {
		return s.storageMutationResult(snapshot, submitErr, nextActions(snapshot, actor))
	}
	submission := latestUnreviewedSubmission(updated, work.ID)
	changed := []string{"attempt:" + attempt.ID}
	if submission != nil {
		changed = append(changed, "submission:"+submission.ID)
	}
	if reviewDispatch != nil {
		changed = append(changed, "review_dispatch:"+reviewDispatch.ID)
	}
	return AppliedResult(updated, changed, nextActions(updated, actor)), nil
}

// bindSubmitWorkInputFromTrustedResponsibility resolves omitted locator fields
// only from the host-issued exact WorkBinding. Explicit model values remain
// strict consistency checks and can never select a sibling Assignment.
func bindSubmitWorkInputFromTrustedResponsibility(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	input SubmitWorkInput,
) (SubmitWorkInput, error) {
	if actor.WorkBinding == nil {
		if strings.TrimSpace(input.WorkItemID) == "" && strings.TrimSpace(input.LogicalKey) == "" {
			return input, domainError(
				ErrorCodeInvalidInput,
				"work_item_id or logical_key is required when no trusted WorkBinding is present",
			)
		}
		return input, nil
	}
	binding := actor.WorkBinding.Normalized()
	if !binding.Complete() {
		return input, workBindingMismatch("trusted WorkBinding is incomplete")
	}
	if explicitWorkID := strings.TrimSpace(input.WorkItemID); explicitWorkID != "" &&
		explicitWorkID != binding.WorkItemID {
		return input, workBindingMismatch("explicit work_item_id does not match the trusted WorkBinding")
	}
	if explicitAssignmentID := strings.TrimSpace(input.AssignmentID); explicitAssignmentID != "" &&
		explicitAssignmentID != binding.AssignmentID {
		return input, workBindingMismatch("explicit assignment_id does not match the trusted WorkBinding")
	}
	if err := validateWorkBindingLogicalKey(snapshot, binding, input.LogicalKey); err != nil {
		return input, err
	}
	input.WorkItemID = binding.WorkItemID
	input.AssignmentID = binding.AssignmentID
	return input, nil
}

func resolveSubmission(
	snapshot *protocol.ExecutionSnapshot,
	submissionID string,
	workItemID string,
	logicalKey string,
) (*protocol.WorkSubmission, error) {
	submissionID = strings.TrimSpace(submissionID)
	workItemID = strings.TrimSpace(workItemID)
	logicalKey = strings.TrimSpace(logicalKey)
	if submissionID != "" {
		for index := range snapshot.Submissions {
			submission := &snapshot.Submissions[index]
			if submission.ID != submissionID {
				continue
			}
			if workItemID != "" && submission.WorkItemID != workItemID {
				return nil, domainError(
					ErrorCodeInvalidInput,
					"submission_id and work_item_id identify different Work Items",
				)
			}
			work := findWorkItemByID(snapshot, submission.WorkItemID)
			if work == nil {
				return nil, domainError(
					ErrorCodeInvalidInput,
					"submission Work Item is not part of the active Plan",
				)
			}
			if logicalKey != "" && strings.TrimSpace(work.LogicalKey) != logicalKey {
				return nil, domainError(
					ErrorCodeInvalidInput,
					"submission_id and logical_key identify different Work Items",
				)
			}
			return submission, nil
		}
		return nil, domainError(ErrorCodeInvalidInput, "submission is not part of the active Plan")
	}
	work, _, err := resolvePlanWork(snapshot, workItemID, logicalKey)
	if err != nil {
		return nil, err
	}
	submission := latestUnreviewedSubmission(snapshot, work.ID)
	if submission == nil {
		return nil, newDomainError(
			ErrorCodeInvalidInput,
			"Work Item has no unreviewed Submission",
			work.LogicalKey,
			"",
		)
	}
	return submission, nil
}

func mergeSubmissionRuntime(
	attempt protocol.WorkAttempt,
	actor ActorContext,
	input SubmitWorkInput,
) protocol.WorkAttempt {
	attempt.ExecutorKind = protocol.AttemptExecutorAgent
	attempt.ExecutorAgentID = strings.TrimSpace(actor.AgentID)
	attempt.RuntimeSessionKey = firstNonEmpty(input.RuntimeSessionKey, actor.SessionKey)
	attempt.RoomSessionID = strings.TrimSpace(input.RoomSessionID)
	attempt.SDKSessionID = strings.TrimSpace(input.SDKSessionID)
	attempt.RuntimeRoundID = strings.TrimSpace(actor.RuntimeRoundID)
	attempt.RootRoundID = strings.TrimSpace(actor.RootRoundID)
	attempt.AgentRoundID = strings.TrimSpace(actor.AgentRoundID)
	attempt.ToolUseID = strings.TrimSpace(input.ToolUseID)
	return attempt
}

func latestUnreviewedSubmission(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
) *protocol.WorkSubmission {
	return latestUnreviewedSubmissionForSpec(snapshot, workItemID, "")
}

func latestUnreviewedSubmissionForSpec(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	specID string,
) *protocol.WorkSubmission {
	specID = strings.TrimSpace(specID)
	reviewed := make(map[string]bool, len(snapshot.Acceptances))
	for _, acceptance := range snapshot.Acceptances {
		reviewed[acceptance.SubmissionID] = true
	}
	var selected *protocol.WorkSubmission
	for index := range snapshot.Submissions {
		submission := &snapshot.Submissions[index]
		if submission.WorkItemID != workItemID ||
			(specID != "" && submission.SpecID != specID) ||
			reviewed[submission.ID] {
			continue
		}
		if selected == nil || submission.Sequence > selected.Sequence {
			selected = submission
		}
	}
	return selected
}

func hasUnreviewedSubmission(snapshot *protocol.ExecutionSnapshot) bool {
	for _, work := range snapshot.WorkItems {
		if latestUnreviewedSubmission(snapshot, work.ID) != nil {
			return true
		}
	}
	return false
}
