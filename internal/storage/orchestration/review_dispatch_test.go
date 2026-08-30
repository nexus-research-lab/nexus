package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryRoomSubmitAtomicallyCreatesReviewDispatch(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("review-return"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand(
			"review-return",
			snapshot.Execution.Version,
			"review-return",
			"",
			1,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(
		snapshot,
		"work-review-return-1",
		"spec-review-return-1",
		"review-return",
		"agent-worker",
	)
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Assignment.ReturnToAgentID = "agent-lead"
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-review-return",
		DedupeKey:     "assignment:review-return",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver result",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.Assignment.ID,
		assign.RootAttempt.ID,
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.RootAttempt.ID,
		protocol.WorkAttemptStatusSucceeded,
	)
	assignment := findAssignment(t, snapshot, assign.Assignment.ID)
	submission := protocol.WorkSubmission{
		ID:               "submission-review-return",
		ExecutionID:      assignment.ExecutionID,
		PlanID:           assignment.PlanID,
		WorkItemID:       assignment.WorkItemID,
		SpecID:           assignment.SpecID,
		AssignmentID:     assignment.ID,
		AttemptID:        assign.RootAttempt.ID,
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "completed review return",
		Evidence:         []string{"verified"},
	}
	versionBeforeFailedSubmit := snapshot.Execution.Version
	if _, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submission,
		Meta:                      testMeta("submit-without-review-return"),
	}); !errors.Is(err, ErrInvariant) {
		t.Fatalf("Room Submit without review Dispatch error = %v, want ErrInvariant", err)
	}
	unchanged, err := repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Execution.Version != versionBeforeFailedSubmit ||
		len(unchanged.Submissions) != 0 ||
		len(unchanged.ReviewDispatches) != 0 {
		t.Fatalf("failed Room Submit was not atomic: %+v", unchanged)
	}

	assignment = findAssignment(t, unchanged, assign.Assignment.ID)
	snapshot, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  unchanged.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submission,
		ReviewDispatch: &protocol.ExecutionReviewDispatch{
			ID:            "review-dispatch-1",
			DedupeKey:     "review-return:" + submission.ID,
			TargetAgentID: "agent-lead",
			Status:        protocol.ExecutionReviewDispatchStatusPending,
			Instruction:   "review the submitted result",
		},
		Meta: testMeta("submit-with-review-return"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Submissions) != 1 ||
		len(snapshot.ReviewDispatches) != 1 {
		t.Fatalf(
			"Submission/review Dispatch snapshot = submissions=%+v dispatches=%+v",
			snapshot.Submissions,
			snapshot.ReviewDispatches,
		)
	}
	reviewDispatch := snapshot.ReviewDispatches[0]
	if reviewDispatch.SubmissionID != submission.ID ||
		reviewDispatch.AssignmentID != assignment.ID ||
		reviewDispatch.TargetAgentID != assignment.ReturnToAgentID ||
		reviewDispatch.Status != protocol.ExecutionReviewDispatchStatusPending {
		t.Fatalf("review Dispatch = %+v", reviewDispatch)
	}
	candidates, err := repository.ListAvailableReviewDispatches(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != reviewDispatch.ID {
		t.Fatalf("available review Dispatches = %+v", candidates)
	}
	claimed, err := repository.ClaimReviewDispatch(
		ctx,
		reviewDispatch.ID,
		reviewDispatch.Version,
		"review-worker",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != protocol.ExecutionReviewDispatchStatusClaimed ||
		claimed.DeliveryAttempts != 1 {
		t.Fatalf("claimed review Dispatch = %+v", claimed)
	}

	snapshot, err = repository.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = reviewTestWork(
		t,
		ctx,
		repository,
		snapshot,
		assignment.ID,
		submission.ID,
		"acceptance-review-return",
		protocol.WorkAcceptanceAccepted,
	)
	if len(snapshot.ReviewDispatches) != 1 ||
		snapshot.ReviewDispatches[0].Status !=
			protocol.ExecutionReviewDispatchStatusCancelled {
		t.Fatalf(
			"review did not cancel undelivered return: %+v",
			snapshot.ReviewDispatches,
		)
	}
}

func TestNormalizeReviewDispatchAllowsSelfAssignmentWithIndependentReviewer(t *testing.T) {
	repository := &Repository{}
	submission := protocol.WorkSubmission{
		ID:           "submission-independent-review",
		ExecutionID:  "execution-independent-review",
		PlanID:       "plan-independent-review",
		WorkItemID:   "work-independent-review",
		SpecID:       "spec-independent-review",
		AssignmentID: "assignment-independent-review",
	}
	assignment := protocol.WorkAssignment{
		ID:              submission.AssignmentID,
		OwnerAgentID:    "agent-coordinator",
		ReturnToAgentID: "agent-reviewer",
		Strategy:        protocol.AssignmentStrategySelf,
	}
	dispatch, err := repository.normalizeReviewDispatch(
		&protocol.ExecutionReviewDispatch{
			ID:            "review-dispatch-independent-review",
			TargetAgentID: assignment.ReturnToAgentID,
			Status:        protocol.ExecutionReviewDispatchStatusPending,
			Instruction:   "review the coordinator deliverable",
		},
		submission,
		assignment,
		testMeta("submit-independent-review"),
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if dispatch == nil || dispatch.AssignmentID != assignment.ID ||
		dispatch.TargetAgentID != assignment.ReturnToAgentID {
		t.Fatalf("independent review Dispatch = %+v", dispatch)
	}
}

func TestRepositoryRoomSelfReviewSubmitNeedsNoReviewDispatch(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand("self-review"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand(
			"self-review",
			snapshot.Execution.Version,
			"self-review",
			"",
			1,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(
		snapshot,
		"work-self-review-1",
		"spec-self-review-1",
		"self-review",
		"agent-worker",
	)
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Assignment.ReturnToAgentID = "agent-worker"
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-self-review",
		DedupeKey:     "assignment:self-review",
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver and self-review result",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.Assignment.ID,
		assign.RootAttempt.ID,
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.RootAttempt.ID,
		protocol.WorkAttemptStatusSucceeded,
	)
	assignment := findAssignment(t, snapshot, assign.Assignment.ID)
	snapshot, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission: protocol.WorkSubmission{
			ID:               "submission-self-review",
			ExecutionID:      assignment.ExecutionID,
			PlanID:           assignment.PlanID,
			WorkItemID:       assignment.WorkItemID,
			SpecID:           assignment.SpecID,
			AssignmentID:     assignment.ID,
			AttemptID:        assign.RootAttempt.ID,
			SubmitterAgentID: "agent-worker",
			ResultSummary:    "completed self-review result",
		},
		Meta: testMeta("submit-self-review"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Submissions) != 1 || len(snapshot.ReviewDispatches) != 0 {
		t.Fatalf(
			"self-review Submission created a redundant return: submissions=%+v dispatches=%+v",
			snapshot.Submissions,
			snapshot.ReviewDispatches,
		)
	}
}

func TestRepositoryTerminalizerCancelsClaimedReviewDispatch(t *testing.T) {
	repository, snapshot, reviewDispatch := setupClaimableReviewDispatch(t, "terminal-review")
	ctx := context.Background()
	claimed, err := repository.ClaimReviewDispatch(
		ctx,
		reviewDispatch.ID,
		reviewDispatch.Version,
		"review-worker",
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.Abandon(ctx, AbandonCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Reason:                   "user abandoned objective",
		Meta:                     testMeta("abandon-terminal-review"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusCancelled {
		t.Fatalf("Execution status = %s", snapshot.Execution.Status)
	}
	current, err := repository.getReviewDispatch(
		ctx,
		repository.db,
		claimed.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if current == nil ||
		current.Status != protocol.ExecutionReviewDispatchStatusCancelled ||
		current.LeaseOwner != "" {
		t.Fatalf("terminal review Dispatch = %+v", current)
	}
}

func setupClaimableReviewDispatch(
	t *testing.T,
	suffix string,
) (*Repository, *protocol.ExecutionSnapshot, protocol.ExecutionReviewDispatch) {
	t.Helper()
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	snapshot, err := repository.Create(ctx, createTestCommand(suffix))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(
		ctx,
		testPlanCommand(suffix, snapshot.Execution.Version, suffix, "", 1),
	)
	if err != nil {
		t.Fatal(err)
	}
	assign := assignTestCommand(
		snapshot,
		"work-"+suffix+"-1",
		"spec-"+suffix+"-1",
		suffix,
		"agent-worker",
	)
	assign.Assignment.Strategy = protocol.AssignmentStrategyRoomMember
	assign.Assignment.ReturnToAgentID = "agent-lead"
	assign.Dispatch = &protocol.ExecutionDispatch{
		ID:            "dispatch-" + suffix,
		DedupeKey:     "assignment:" + suffix,
		TargetAgentID: "agent-worker",
		Kind:          protocol.ExecutionDispatchRoomDirected,
		Status:        protocol.ExecutionDispatchStatusPending,
		Instruction:   "deliver result",
	}
	snapshot, err = repository.Assign(ctx, assign)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = startTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.Assignment.ID,
		assign.RootAttempt.ID,
	)
	snapshot = finishTestAttempt(
		t,
		ctx,
		repository,
		snapshot,
		assign.RootAttempt.ID,
		protocol.WorkAttemptStatusSucceeded,
	)
	assignment := findAssignment(t, snapshot, assign.Assignment.ID)
	submission := protocol.WorkSubmission{
		ID:               "submission-" + suffix,
		ExecutionID:      assignment.ExecutionID,
		PlanID:           assignment.PlanID,
		WorkItemID:       assignment.WorkItemID,
		SpecID:           assignment.SpecID,
		AssignmentID:     assignment.ID,
		AttemptID:        assign.RootAttempt.ID,
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "completed " + suffix,
	}
	reviewDispatch := protocol.ExecutionReviewDispatch{
		ID:            "review-dispatch-" + suffix,
		DedupeKey:     "review-return:" + submission.ID,
		TargetAgentID: "agent-lead",
		Status:        protocol.ExecutionReviewDispatchStatusPending,
		Instruction:   "review result",
	}
	snapshot, err = repository.Submit(ctx, SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission:                submission,
		ReviewDispatch:            &reviewDispatch,
		Meta:                      testMeta("submit-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository, snapshot, snapshot.ReviewDispatches[0]
}
