// INPUT: Room Submission review-return outbox、Assignment 选择的 reviewer target 与 Room delivery receipt。
// OUTPUT: 独立于 worker Dispatch 的 claim/retry/deliver loop、迟到回交 admission 与 outbox 状态变更后的 session 失效事实。
// POS: Submission commit 后可靠唤醒 reviewer，不依赖模型手写 @Coordinator。
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// ExecutionReviewDispatchDelivery 是交给 Room 数据面的结构化 review 回交。
type ExecutionReviewDispatchDelivery struct {
	OwnerUserID       string
	SessionKey        string
	RoomID            string
	ConversationID    string
	SourceAgentID     string
	TargetAgentID     string
	Instruction       string
	Binding           protocol.ExecutionReviewBinding
	DispatchDedupeKey string
}

// ExecutionReviewDispatchReceipt 表示 Room 已同步接受 review handoff/queue。
type ExecutionReviewDispatchReceipt struct {
	HandoffID   string
	QueueItemID string
}

// ExecutionReviewDispatchConsumer 是 Room realtime 实现的 review 回交端口。
type ExecutionReviewDispatchConsumer interface {
	DeliverExecutionReviewDispatch(
		context.Context,
		ExecutionReviewDispatchDelivery,
	) (ExecutionReviewDispatchReceipt, error)
}

type reviewDispatchOutboxRepository interface {
	ListAvailableReviewDispatches(context.Context, int) ([]protocol.ExecutionReviewDispatch, error)
	ClaimReviewDispatch(
		context.Context,
		string,
		int64,
		string,
		time.Duration,
	) (*protocol.ExecutionReviewDispatch, error)
	MarkReviewDispatchDelivered(
		context.Context,
		string,
		int64,
		string,
		string,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
	RetryReviewDispatch(
		context.Context,
		string,
		int64,
		string,
		time.Time,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
	CancelReviewDispatch(
		context.Context,
		string,
		int64,
		string,
		string,
	) (*protocol.ExecutionReviewDispatch, error)
	GetSnapshot(context.Context, string) (*protocol.ExecutionSnapshot, error)
}

// SetExecutionReviewDispatchConsumer 注入 Room structured review-return 数据面。
func (s *Service) SetExecutionReviewDispatchConsumer(
	consumer ExecutionReviewDispatchConsumer,
) {
	s.reviewDispatchConsumer = consumer
}

// DispatchPendingReviews claim 并投递一批 Submission review 回交。
func (s *Service) DispatchPendingReviews(
	ctx context.Context,
	workerID string,
	limit int,
) (DispatchRunResult, error) {
	var result DispatchRunResult
	repository, ok := s.repository.(reviewDispatchOutboxRepository)
	if !ok {
		return result, errors.New(
			"orchestration repository does not support review dispatch outbox",
		)
	}
	if s.reviewDispatchConsumer == nil {
		return result, errors.New("execution review dispatch consumer is unavailable")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return result, errors.New("review dispatch worker id is required")
	}
	candidates, err := repository.ListAvailableReviewDispatches(ctx, limit)
	if err != nil {
		return result, err
	}
	for _, candidate := range candidates {
		claimed, claimErr := repository.ClaimReviewDispatch(
			ctx,
			candidate.ID,
			candidate.Version,
			workerID,
			30*time.Second,
		)
		if errors.Is(claimErr, orchestrationstore.ErrDispatchLease) {
			continue
		}
		if claimErr != nil {
			return result, claimErr
		}
		if claimed == nil {
			continue
		}
		result.Claimed++
		// Claim is itself durable and visible in the WorkGraph review state.
		s.invalidateExecutionID(ctx, candidate.ExecutionID)
		delivered, deliveryErr := s.deliverClaimedReviewDispatch(
			ctx,
			repository,
			workerID,
			claimed,
		)
		if deliveryErr != nil {
			result.Retried++
			s.invalidateExecutionID(ctx, candidate.ExecutionID)
			continue
		}
		if delivered {
			result.Delivered++
		} else {
			result.Cancelled++
		}
		s.invalidateExecutionID(ctx, candidate.ExecutionID)
	}
	return result, nil
}

func (s *Service) deliverClaimedReviewDispatch(
	ctx context.Context,
	repository reviewDispatchOutboxRepository,
	workerID string,
	dispatch *protocol.ExecutionReviewDispatch,
) (bool, error) {
	if dispatch == nil {
		return false, errors.New("claimed review dispatch is nil")
	}
	snapshot, err := repository.GetSnapshot(ctx, dispatch.ExecutionID)
	if err != nil {
		return false, s.retryClaimedReviewDispatch(
			ctx,
			repository,
			workerID,
			dispatch,
			err,
		)
	}
	submission, admissionErr := authorizeReviewDispatchSnapshot(
		snapshot,
		*dispatch,
		dispatch.TargetAgentID,
	)
	if admissionErr != nil {
		_, cancelErr := repository.CancelReviewDispatch(
			ctx,
			dispatch.ID,
			dispatch.Version,
			workerID,
			admissionErr.Error(),
		)
		return false, cancelErr
	}
	receipt, err := s.reviewDispatchConsumer.DeliverExecutionReviewDispatch(
		ctx,
		ExecutionReviewDispatchDelivery{
			OwnerUserID:       snapshot.Execution.OwnerUserID,
			SessionKey:        snapshot.Execution.SessionKey,
			RoomID:            snapshot.Execution.RoomID,
			ConversationID:    snapshot.Execution.ConversationID,
			SourceAgentID:     submission.SubmitterAgentID,
			TargetAgentID:     dispatch.TargetAgentID,
			Instruction:       dispatch.Instruction,
			DispatchDedupeKey: dispatch.DedupeKey,
			Binding: protocol.ExecutionReviewBinding{
				ExecutionID:      dispatch.ExecutionID,
				PlanID:           dispatch.PlanID,
				WorkItemID:       dispatch.WorkItemID,
				SpecID:           dispatch.SpecID,
				AssignmentID:     dispatch.AssignmentID,
				SubmissionID:     dispatch.SubmissionID,
				ReviewDispatchID: dispatch.ID,
				TargetAgentID:    dispatch.TargetAgentID,
			},
		},
	)
	if err != nil {
		return false, s.retryClaimedReviewDispatch(
			ctx,
			repository,
			workerID,
			dispatch,
			err,
		)
	}
	_, err = repository.MarkReviewDispatchDelivered(
		ctx,
		dispatch.ID,
		dispatch.Version,
		workerID,
		receipt.HandoffID,
		receipt.QueueItemID,
	)
	return err == nil, err
}

func (s *Service) retryClaimedReviewDispatch(
	ctx context.Context,
	repository reviewDispatchOutboxRepository,
	workerID string,
	dispatch *protocol.ExecutionReviewDispatch,
	cause error,
) error {
	delay := time.Second << min(dispatch.DeliveryAttempts-1, 6)
	_, retryErr := repository.RetryReviewDispatch(
		ctx,
		dispatch.ID,
		dispatch.Version,
		workerID,
		s.now().UTC().Add(delay),
		cause.Error(),
	)
	if retryErr != nil {
		return errors.Join(cause, retryErr)
	}
	return cause
}

func authorizeReviewDispatchSnapshot(
	snapshot *protocol.ExecutionSnapshot,
	dispatch protocol.ExecutionReviewDispatch,
	targetAgentID string,
) (*protocol.WorkSubmission, error) {
	if snapshot == nil {
		return nil, errors.New("review return Execution no longer exists")
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusActive &&
		snapshot.Execution.Status != protocol.ExecutionStatusWaiting {
		return nil, errors.New("review return targets a terminal or paused Execution")
	}
	if snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		snapshot.Plan == nil ||
		snapshot.Plan.ID != dispatch.PlanID ||
		snapshot.Plan.Status != protocol.PlanRevisionStatusActive {
		return nil, errors.New("review return targets a stale Room Plan")
	}
	targetAgentID = strings.TrimSpace(targetAgentID)
	if targetAgentID == "" ||
		targetAgentID != strings.TrimSpace(dispatch.TargetAgentID) {
		return nil, errors.New("review return target does not match the selected reviewer")
	}
	var assignment *protocol.WorkAssignment
	for index := range snapshot.Assignments {
		candidate := &snapshot.Assignments[index]
		if candidate.ID == dispatch.AssignmentID {
			assignment = candidate
			break
		}
	}
	if assignment == nil ||
		assignment.ExecutionID != dispatch.ExecutionID ||
		assignment.PlanID != dispatch.PlanID ||
		assignment.WorkItemID != dispatch.WorkItemID ||
		assignment.SpecID != dispatch.SpecID ||
		strings.TrimSpace(assignment.ReturnToAgentID) != targetAgentID ||
		assignment.Status != protocol.WorkAssignmentStatusActive {
		return nil, errors.New("review return Assignment is stale")
	}
	var submission *protocol.WorkSubmission
	for index := range snapshot.Submissions {
		candidate := &snapshot.Submissions[index]
		if candidate.ID == dispatch.SubmissionID {
			submission = candidate
			break
		}
	}
	if submission == nil ||
		submission.ExecutionID != dispatch.ExecutionID ||
		submission.PlanID != dispatch.PlanID ||
		submission.WorkItemID != dispatch.WorkItemID ||
		submission.SpecID != dispatch.SpecID ||
		submission.AssignmentID != dispatch.AssignmentID {
		return nil, errors.New("review return Submission is stale")
	}
	for _, acceptance := range snapshot.Acceptances {
		if acceptance.SubmissionID == submission.ID {
			return nil, errors.New("review return Submission was already reviewed")
		}
	}
	dispatchMatched := false
	for _, candidate := range snapshot.ReviewDispatches {
		if candidate.ID == dispatch.ID &&
			candidate.ExecutionID == dispatch.ExecutionID &&
			candidate.SubmissionID == dispatch.SubmissionID &&
			candidate.TargetAgentID == targetAgentID &&
			candidate.Status != protocol.ExecutionReviewDispatchStatusCancelled &&
			candidate.Status != protocol.ExecutionReviewDispatchStatusFailed {
			dispatchMatched = true
			break
		}
	}
	if !dispatchMatched {
		return nil, errors.New("review return Dispatch is stale")
	}
	return submission, nil
}

// AuthorizeRoomReviewReturn 在 Room 写 workspace queue 前重新核验 trusted review binding。
func (s *Service) AuthorizeRoomReviewReturn(
	ctx context.Context,
	actor ActorContext,
	binding *protocol.ExecutionReviewBinding,
) error {
	if err := validateActor(actor); err != nil {
		return err
	}
	normalized := binding.Normalized()
	if !normalized.Complete() {
		return domainError(
			ErrorCodeAssignmentTargetInvalid,
			"structured Room review binding is incomplete",
		)
	}
	snapshot, err := s.repository.GetSnapshot(ctx, normalized.ExecutionID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return domainError(
			ErrorCodeAssignmentTargetInvalid,
			"structured Room review Execution no longer exists",
		)
	}
	if err = authorizeSnapshot(actor, snapshot); err != nil {
		return err
	}
	dispatch := protocol.ExecutionReviewDispatch{
		ID:            normalized.ReviewDispatchID,
		ExecutionID:   normalized.ExecutionID,
		PlanID:        normalized.PlanID,
		WorkItemID:    normalized.WorkItemID,
		SpecID:        normalized.SpecID,
		AssignmentID:  normalized.AssignmentID,
		SubmissionID:  normalized.SubmissionID,
		TargetAgentID: normalized.TargetAgentID,
	}
	if _, err = authorizeReviewDispatchSnapshot(
		snapshot,
		dispatch,
		strings.TrimSpace(actor.AgentID),
	); err != nil {
		return domainError(
			ErrorCodeAssignmentTargetInvalid,
			fmt.Sprintf("structured Room review return rejected: %v", err),
		)
	}
	return nil
}
