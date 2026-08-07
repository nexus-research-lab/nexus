package orchestration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type executionReviewDispatchConsumerFunc func(
	context.Context,
	ExecutionReviewDispatchDelivery,
) (ExecutionReviewDispatchReceipt, error)

func (f executionReviewDispatchConsumerFunc) DeliverExecutionReviewDispatch(
	ctx context.Context,
	delivery ExecutionReviewDispatchDelivery,
) (ExecutionReviewDispatchReceipt, error) {
	return f(ctx, delivery)
}

type reviewDispatchRepositoryFake struct {
	*fakeRepository
	candidates []protocol.ExecutionReviewDispatch
	claim      func(
		string,
		int64,
		string,
		time.Duration,
	) (*protocol.ExecutionReviewDispatch, error)
	mark func(
		string,
		int64,
		string,
		string,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
	retry func(
		string,
		int64,
		string,
		time.Time,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
	cancel func(
		string,
		int64,
		string,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
}

func (f *reviewDispatchRepositoryFake) ListAvailableReviewDispatches(
	context.Context,
	int,
) ([]protocol.ExecutionReviewDispatch, error) {
	return append([]protocol.ExecutionReviewDispatch(nil), f.candidates...), nil
}

func (f *reviewDispatchRepositoryFake) ClaimReviewDispatch(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	lease time.Duration,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.claim == nil {
		return nil, errors.New("unexpected ClaimReviewDispatch")
	}
	return f.claim(id, version, workerID, lease)
}

func (f *reviewDispatchRepositoryFake) MarkReviewDispatchDelivered(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	handoffID string,
	queueItemID string,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.mark == nil {
		return nil, errors.New("unexpected MarkReviewDispatchDelivered")
	}
	return f.mark(id, version, workerID, handoffID, queueItemID)
}

func (f *reviewDispatchRepositoryFake) RetryReviewDispatch(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	retryAt time.Time,
	cause string,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.retry == nil {
		return nil, errors.New("unexpected RetryReviewDispatch")
	}
	return f.retry(id, version, workerID, retryAt, cause)
}

func (f *reviewDispatchRepositoryFake) CancelReviewDispatch(
	_ context.Context,
	id string,
	version int64,
	workerID string,
	reason string,
) (*protocol.ExecutionReviewDispatch, error) {
	if f.cancel == nil {
		return nil, errors.New("unexpected CancelReviewDispatch")
	}
	return f.cancel(id, version, workerID, reason)
}

func TestDispatchPendingReviewsDeliversTrustedCoordinatorBinding(t *testing.T) {
	snapshot, dispatch := reviewReturnSnapshot()
	claimed := dispatch
	claimed.Status = protocol.ExecutionReviewDispatchStatusClaimed
	claimed.Version = 2
	claimed.DeliveryAttempts = 1
	repository := &reviewDispatchRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		candidates:     []protocol.ExecutionReviewDispatch{dispatch},
		claim: func(
			id string,
			version int64,
			workerID string,
			_ time.Duration,
		) (*protocol.ExecutionReviewDispatch, error) {
			if id != dispatch.ID || version != dispatch.Version ||
				workerID != "review-worker" {
				t.Fatalf(
					"claim identity = id=%q version=%d worker=%q",
					id,
					version,
					workerID,
				)
			}
			return &claimed, nil
		},
		mark: func(
			id string,
			version int64,
			workerID string,
			handoffID string,
			queueItemID string,
		) (*protocol.ExecutionReviewDispatch, error) {
			if id != dispatch.ID || version != claimed.Version ||
				workerID != "review-worker" ||
				handoffID != "review-handoff-1" ||
				queueItemID != "review-queue-1" {
				t.Fatalf(
					"mark identity = id=%q version=%d worker=%q handoff=%q queue=%q",
					id,
					version,
					workerID,
					handoffID,
					queueItemID,
				)
			}
			delivered := claimed
			delivered.Status = protocol.ExecutionReviewDispatchStatusDelivered
			return &delivered, nil
		},
	}
	service := testService(repository)
	var delivered ExecutionReviewDispatchDelivery
	service.SetExecutionReviewDispatchConsumer(
		executionReviewDispatchConsumerFunc(func(
			_ context.Context,
			value ExecutionReviewDispatchDelivery,
		) (ExecutionReviewDispatchReceipt, error) {
			delivered = value
			return ExecutionReviewDispatchReceipt{
				HandoffID:   "review-handoff-1",
				QueueItemID: "review-queue-1",
			}, nil
		}),
	)
	result, err := service.DispatchPendingReviews(
		context.Background(),
		"review-worker",
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 1 || result.Delivered != 1 ||
		result.Retried != 0 || result.Cancelled != 0 {
		t.Fatalf("Dispatch result = %+v", result)
	}
	if delivered.SourceAgentID != "agent-worker" ||
		delivered.TargetAgentID != "agent-lead" ||
		delivered.Binding.ExecutionID != snapshot.Execution.ID ||
		delivered.Binding.PlanID != snapshot.Plan.ID ||
		delivered.Binding.WorkItemID != "work-1" ||
		delivered.Binding.SpecID != "spec-1" ||
		delivered.Binding.AssignmentID != "assignment-1" ||
		delivered.Binding.SubmissionID != "submission-1" ||
		delivered.Binding.ReviewDispatchID != dispatch.ID ||
		delivered.Binding.TargetAgentID != "agent-lead" {
		t.Fatalf("review delivery = %+v", delivered)
	}
	contextValue := RenderExecutionContext(
		snapshot,
		ExecutionContextOptions{
			ActorAgentID: "agent-lead",
			ScopeKind:    protocol.ExecutionScopeRoom,
		},
	)
	if !strings.Contains(contextValue, "<pending_reviews>") ||
		!strings.Contains(contextValue, `submission_id="submission-1"`) ||
		!strings.Contains(contextValue, "<action>review_work</action>") {
		t.Fatalf("coordinator context lacks actionable pending review:\n%s", contextValue)
	}
}

func TestDispatchPendingReviewsCancelsTerminalReturnWithoutDelivery(t *testing.T) {
	snapshot, dispatch := reviewReturnSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	claimed := dispatch
	claimed.Status = protocol.ExecutionReviewDispatchStatusClaimed
	claimed.Version = 2
	claimed.DeliveryAttempts = 1
	cancelled := false
	repository := &reviewDispatchRepositoryFake{
		fakeRepository: &fakeRepository{snapshot: snapshot},
		candidates:     []protocol.ExecutionReviewDispatch{dispatch},
		claim: func(
			string,
			int64,
			string,
			time.Duration,
		) (*protocol.ExecutionReviewDispatch, error) {
			return &claimed, nil
		},
		cancel: func(
			id string,
			version int64,
			workerID string,
			reason string,
		) (*protocol.ExecutionReviewDispatch, error) {
			cancelled = true
			if id != dispatch.ID || version != claimed.Version ||
				workerID != "review-worker" ||
				!strings.Contains(reason, "terminal") {
				t.Fatalf(
					"cancel identity = id=%q version=%d worker=%q reason=%q",
					id,
					version,
					workerID,
					reason,
				)
			}
			value := claimed
			value.Status = protocol.ExecutionReviewDispatchStatusCancelled
			return &value, nil
		},
	}
	service := testService(repository)
	service.SetExecutionReviewDispatchConsumer(
		executionReviewDispatchConsumerFunc(func(
			context.Context,
			ExecutionReviewDispatchDelivery,
		) (ExecutionReviewDispatchReceipt, error) {
			t.Fatal("terminal review return reached Room delivery")
			return ExecutionReviewDispatchReceipt{}, nil
		}),
	)
	result, err := service.DispatchPendingReviews(
		context.Background(),
		"review-worker",
		10,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled || result.Cancelled != 1 || result.Delivered != 0 {
		t.Fatalf("terminal Dispatch result = %+v, cancelled=%t", result, cancelled)
	}
}

func reviewReturnSnapshot() (*protocol.ExecutionSnapshot, protocol.ExecutionReviewDispatch) {
	snapshot := roomAssignmentSnapshot()
	snapshot.Execution.CoordinatorAgentID = "agent-lead"
	snapshot.Execution.Status = protocol.ExecutionStatusActive
	snapshot.Assignments = []protocol.WorkAssignment{{
		ID:                "assignment-1",
		ExecutionID:       snapshot.Execution.ID,
		PlanID:            snapshot.Plan.ID,
		WorkItemID:        "work-1",
		SpecID:            "spec-1",
		OwnerAgentID:      "agent-worker",
		AssignedByAgentID: "agent-lead",
		ReturnToAgentID:   "agent-lead",
		Strategy:          protocol.AssignmentStrategyRoomMember,
		Status:            protocol.WorkAssignmentStatusActive,
		Version:           3,
	}}
	snapshot.Submissions = []protocol.WorkSubmission{{
		ID:               "submission-1",
		ExecutionID:      snapshot.Execution.ID,
		PlanID:           snapshot.Plan.ID,
		WorkItemID:       "work-1",
		SpecID:           "spec-1",
		AssignmentID:     "assignment-1",
		AttemptID:        "attempt-1",
		SubmitterAgentID: "agent-worker",
		ResultSummary:    "completed evidence",
	}}
	dispatch := protocol.ExecutionReviewDispatch{
		ID:            "review-dispatch-1",
		ExecutionID:   snapshot.Execution.ID,
		PlanID:        snapshot.Plan.ID,
		WorkItemID:    "work-1",
		SpecID:        "spec-1",
		AssignmentID:  "assignment-1",
		SubmissionID:  "submission-1",
		DedupeKey:     "review-return:submission-1",
		TargetAgentID: "agent-lead",
		Status:        protocol.ExecutionReviewDispatchStatusPending,
		Instruction:   "review evidence",
		Version:       1,
	}
	snapshot.ReviewDispatches = []protocol.ExecutionReviewDispatch{dispatch}
	return snapshot, dispatch
}
