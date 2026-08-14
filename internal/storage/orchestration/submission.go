// INPUT: succeeded Attempt 的 immutable Submission 与 reviewer Acceptance command。
// OUTPUT: append-only Submission/Acceptance、accepted Review completion audit receipt、Assignment completion/release 与事件。
// POS: worker 交付和 reviewer 决策不可混写的事务边界。
package orchestration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (r *Repository) rejectUnreviewedSubmission(
	ctx context.Context,
	queryer sqlQueryer,
	executionID string,
	workItemID string,
	specID string,
) error {
	var pending int
	if err := queryer.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_submissions submission
LEFT JOIN execution_acceptances acceptance
  ON acceptance.submission_id = submission.submission_id
WHERE submission.execution_id = `+r.bind(1)+`
  AND submission.work_item_id = `+r.bind(2)+`
  AND submission.spec_id = `+r.bind(3)+`
  AND acceptance.acceptance_id IS NULL`,
		executionID, workItemID, specID,
	).Scan(&pending); err != nil {
		return err
	}
	if pending != 0 {
		return fmt.Errorf("%w: Work Item has an unreviewed Submission", ErrInvariant)
	}
	return nil
}

// Submit 追加 immutable Submission；不会隐式 Acceptance。
func (r *Repository) Submit(ctx context.Context, command SubmitCommand) (*protocol.ExecutionSnapshot, error) {
	item := command.Submission
	item.ID = strings.TrimSpace(item.ID)
	item.ExecutionID = strings.TrimSpace(item.ExecutionID)
	item.AssignmentID = strings.TrimSpace(item.AssignmentID)
	item.AttemptID = strings.TrimSpace(item.AttemptID)
	item.SubmitterAgentID = strings.TrimSpace(item.SubmitterAgentID)
	if item.ID == "" || item.ExecutionID == "" || item.AssignmentID == "" ||
		item.AttemptID == "" || item.SubmitterAgentID == "" ||
		strings.TrimSpace(item.ResultSummary) == "" {
		return nil, fmt.Errorf("%w: Submission identity, submitter and summary are required", ErrInvariant)
	}
	for _, collection := range []struct {
		field string
		count int
	}{
		{field: "result_refs", count: len(item.ResultRefs)},
		{field: "submission_evidence", count: len(item.Evidence)},
	} {
		if err := protocol.ValidateExecutionProjectionLimit(
			collection.field,
			collection.count,
		); err != nil {
			return nil, err
		}
	}
	mutation, err := r.beginMutation(
		ctx, item.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventWorkSubmitted,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	assignment, err := r.getAssignment(ctx, mutation.tx, item.AssignmentID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if assignment == nil || assignment.Status != protocol.WorkAssignmentStatusActive ||
		assignment.ExecutionID != item.ExecutionID || assignment.PlanID != item.PlanID ||
		assignment.WorkItemID != item.WorkItemID || assignment.SpecID != item.SpecID ||
		assignment.OwnerAgentID != item.SubmitterAgentID {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Submission is outside active Assignment or wrong owner", ErrInvariant)
	}
	attempt, err := r.getAttempt(ctx, mutation.tx, item.AttemptID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if attempt == nil || !attemptMatchesAssignment(*attempt, *assignment) ||
		attempt.Status != protocol.WorkAttemptStatusSucceeded {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Submission requires succeeded Attempt in the same chain", ErrInvariant)
	}
	if err = validateExpectedVersion(command.ExpectedAssignmentVersion, "expected Assignment version"); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	var pending int
	if err = mutation.tx.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM execution_submissions submission
LEFT JOIN execution_acceptances acceptance
  ON acceptance.submission_id = submission.submission_id
WHERE submission.work_item_id = `+r.bind(1)+`
  AND submission.spec_id = `+r.bind(2)+`
  AND acceptance.acceptance_id IS NULL`,
		item.WorkItemID, item.SpecID,
	).Scan(&pending); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if pending != 0 {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Work Item already has an unreviewed Submission", ErrInvariant)
	}
	var nextSequence int64
	if err = mutation.tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(submission_sequence), 0) + 1
FROM execution_submissions
WHERE work_item_id = `+r.bind(1),
		item.WorkItemID,
	).Scan(&nextSequence); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if item.Sequence != 0 && item.Sequence != nextSequence {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Submission sequence is stale", ErrVersionConflict)
	}
	item.Sequence = nextSequence
	item.CreatedAt = timeOr(item.CreatedAt, r.currentTime())
	reviewDispatch, err := r.normalizeReviewDispatch(
		command.ReviewDispatch,
		item,
		*assignment,
		command.Meta,
		item.CreatedAt,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	result, err := mutation.tx.ExecContext(ctx, `
UPDATE execution_work_assignments
SET version = version + 1
WHERE assignment_id = `+r.bind(1)+`
  AND version = `+r.bind(2)+`
  AND status = 'active'`,
		assignment.ID, command.ExpectedAssignmentVersion,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = r.insertSubmission(ctx, mutation.tx, item); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if reviewDispatch != nil {
		if err = r.insertReviewDispatch(ctx, mutation.tx, *reviewDispatch); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	event := protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntitySubmission,
		EntityID:      item.ID,
		EntityVersion: item.Sequence,
		PlanID:        item.PlanID,
		WorkItemID:    item.WorkItemID,
		SpecID:        item.SpecID,
		AssignmentID:  item.AssignmentID,
		AttemptID:     item.AttemptID,
		SubmissionID:  item.ID,
	}
	if reviewDispatch != nil {
		event.ReviewDispatchID = reviewDispatch.ID
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:       event.EntityType,
		EntityID:         event.EntityID,
		EntityVersion:    event.EntityVersion,
		PlanID:           event.PlanID,
		WorkItemID:       event.WorkItemID,
		SpecID:           event.SpecID,
		AssignmentID:     event.AssignmentID,
		AttemptID:        event.AttemptID,
		SubmissionID:     event.SubmissionID,
		ReviewDispatchID: event.ReviewDispatchID,
	})
}

// Review 追加 append-only Acceptance，并按 decision 完成或释放 Assignment。
func (r *Repository) Review(ctx context.Context, command ReviewCommand) (*protocol.ExecutionSnapshot, error) {
	item := command.Acceptance
	item.ID = strings.TrimSpace(item.ID)
	item.ExecutionID = strings.TrimSpace(item.ExecutionID)
	item.SubmissionID = strings.TrimSpace(item.SubmissionID)
	item.ReviewerID = strings.TrimSpace(item.ReviewerID)
	if item.ID == "" || item.ExecutionID == "" || item.SubmissionID == "" || item.ReviewerID == "" {
		return nil, fmt.Errorf("%w: Acceptance identity and reviewer are required", ErrInvariant)
	}
	if err := protocol.ValidateExecutionProjectionLimit(
		"criteria_results",
		len(item.CriteriaResults),
	); err != nil {
		return nil, err
	}
	for index, criterion := range item.CriteriaResults {
		if err := protocol.ValidateExecutionProjectionLimit(
			fmt.Sprintf("criteria_results[%d].evidence", index),
			len(criterion.Evidence),
		); err != nil {
			return nil, err
		}
	}
	switch item.Decision {
	case protocol.WorkAcceptanceAccepted,
		protocol.WorkAcceptanceRejected,
		protocol.WorkAcceptanceChangesRequested:
	default:
		return nil, fmt.Errorf("%w: Acceptance decision %q is invalid", ErrInvariant, item.Decision)
	}
	if item.Decision == protocol.WorkAcceptanceAccepted {
		for _, criterion := range item.CriteriaResults {
			if !criterion.Passed {
				return nil, fmt.Errorf("%w: accepted decision contains failed criterion %q", ErrInvariant, criterion.Criterion)
			}
		}
	}
	mutation, err := r.beginMutation(
		ctx, item.ExecutionID, command.ExpectedExecutionVersion,
		command.Meta, protocol.ExecutionEventAcceptanceRecorded,
	)
	if err != nil {
		return nil, err
	}
	if mutation.replayed {
		return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{})
	}
	submission, err := r.getSubmission(ctx, mutation.tx, item.SubmissionID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if submission == nil || submission.ExecutionID != item.ExecutionID ||
		submission.PlanID != item.PlanID || submission.WorkItemID != item.WorkItemID ||
		submission.SpecID != item.SpecID || submission.AssignmentID != item.AssignmentID {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Acceptance is outside Submission chain", ErrInvariant)
	}
	assignment, err := r.getAssignment(ctx, mutation.tx, submission.AssignmentID)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if assignment == nil || assignment.Status != protocol.WorkAssignmentStatusActive {
		r.abortMutation(mutation)
		return nil, fmt.Errorf("%w: Submission Assignment is not active", ErrInvariant)
	}
	if err = validateExpectedVersion(command.ExpectedAssignmentVersion, "expected Assignment version"); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	item.CreatedAt = timeOr(item.CreatedAt, r.currentTime())
	if err = r.insertAcceptance(ctx, mutation.tx, item); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if _, err = mutation.tx.ExecContext(ctx, `
UPDATE execution_review_dispatches
SET status = 'cancelled',
    last_error = 'Submission already reviewed',
    version = version + 1,
    lease_owner = NULL,
    lease_expires_at = NULL,
    updated_at = `+r.bind(1)+`
WHERE submission_id = `+r.bind(2)+`
  AND status IN ('pending', 'claimed', 'failed')`,
		r.timestamp(item.CreatedAt), item.SubmissionID,
	); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	nextStatus := protocol.WorkAssignmentStatusReleased
	timestampColumn := "released_at"
	if item.Decision == protocol.WorkAcceptanceAccepted {
		nextStatus = protocol.WorkAssignmentStatusCompleted
		timestampColumn = "completed_at"
	}
	query := `
UPDATE execution_work_assignments
SET status = ` + r.bind(1) + `,
    version = version + 1,
    ` + timestampColumn + ` = ` + r.bind(2) + `
WHERE assignment_id = ` + r.bind(3) + `
  AND version = ` + r.bind(4) + `
  AND status = 'active'`
	result, err := mutation.tx.ExecContext(ctx, query,
		nextStatus, r.timestamp(item.CreatedAt), assignment.ID, command.ExpectedAssignmentVersion,
	)
	if err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if err = requireOne(result); err != nil {
		r.abortMutation(mutation)
		return nil, err
	}
	if item.Decision == protocol.WorkAcceptanceAccepted {
		if err = r.ensureCompletionAuditTx(
			ctx,
			mutation.tx,
			item.ExecutionID,
			item.ID,
			item.CreatedAt,
		); err != nil {
			r.abortMutation(mutation)
			return nil, err
		}
	}
	return r.finishMutation(ctx, mutation, command.Meta, protocol.ExecutionEvent{
		EntityType:    protocol.ExecutionEntityAcceptance,
		EntityID:      item.ID,
		EntityVersion: 1,
		PlanID:        item.PlanID,
		WorkItemID:    item.WorkItemID,
		SpecID:        item.SpecID,
		AssignmentID:  item.AssignmentID,
		SubmissionID:  item.SubmissionID,
		AcceptanceID:  item.ID,
	})
}

func (r *Repository) insertSubmission(ctx context.Context, tx *sql.Tx, item protocol.WorkSubmission) error {
	resultRefsJSON, err := marshalSlice(item.ResultRefs)
	if err != nil {
		return err
	}
	evidenceJSON, err := marshalSlice(item.Evidence)
	if err != nil {
		return err
	}
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_submissions (
    submission_id, execution_id, plan_id, work_item_id, spec_id,
    assignment_id, attempt_id, submission_sequence, submitter_agent_id,
    result_summary, result_refs_json, evidence_json, created_at, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.jsonBind(11)+`,`+r.jsonBind(12)+`,`+r.bind(13)+`,`+r.jsonBind(14)+`)`,
		item.ID, item.ExecutionID, item.PlanID, item.WorkItemID, item.SpecID,
		item.AssignmentID, item.AttemptID, item.Sequence, item.SubmitterAgentID,
		item.ResultSummary, resultRefsJSON, evidenceJSON, r.timestamp(item.CreatedAt), metadataJSON,
	)
	return err
}

func (r *Repository) insertAcceptance(ctx context.Context, tx *sql.Tx, item protocol.WorkAcceptance) error {
	criteriaJSON, err := marshalSlice(item.CriteriaResults)
	if err != nil {
		return err
	}
	metadataJSON, err := marshalMap(item.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO execution_acceptances (
    acceptance_id, execution_id, plan_id, work_item_id, spec_id,
    assignment_id, submission_id, decision, reviewer_kind, reviewer_id,
    criteria_results_json, feedback, decision_round_id, created_at, metadata_json
) VALUES (`+
		r.bind(1)+`,`+r.bind(2)+`,`+r.bind(3)+`,`+r.bind(4)+`,`+r.bind(5)+`,`+
		r.bind(6)+`,`+r.bind(7)+`,`+r.bind(8)+`,`+r.bind(9)+`,`+r.bind(10)+`,`+
		r.jsonBind(11)+`,`+r.bind(12)+`,`+r.bind(13)+`,`+r.bind(14)+`,`+r.jsonBind(15)+`)`,
		item.ID, item.ExecutionID, item.PlanID, item.WorkItemID, item.SpecID,
		item.AssignmentID, item.SubmissionID, item.Decision, item.ReviewerKind,
		item.ReviewerID, criteriaJSON, nullString(item.Feedback),
		nullString(item.DecisionRoundID), r.timestamp(item.CreatedAt), metadataJSON,
	)
	return err
}
