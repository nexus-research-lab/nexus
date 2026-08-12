// INPUT: Attempt physical cancellation outbox、typed runtime target 与 consumer receipt。
// OUTPUT: 独立 lease/retry/recovery drain，诚实区分 provider/local cancellation、no-op 与 unsupported，并广播 outbox 状态变更后的 session 失效事实。
// POS: SQL terminal state 与 Room/DM runtime interruption 之间的 durable application bridge。
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

// ExecutionCancellationDelivery 是交给 Room/runtime 数据面的精确中断目标。
type ExecutionCancellationDelivery struct {
	Binding      protocol.ExecutionCancellationBinding
	ExecutorKind protocol.AttemptExecutorKind
	Reason       string
}

// ExecutionCancellationReceipt 诚实记录实际中断或 exact target 已失效。
type ExecutionCancellationReceipt struct {
	Outcome        protocol.ExecutionCancellationOutcome
	LimitationCode string
	Detail         string
}

// ExecutionCancellationConsumer 由 App 按 Room slot 或 DM/runtime round 路由。
type ExecutionCancellationConsumer interface {
	DeliverExecutionCancellation(
		context.Context,
		ExecutionCancellationDelivery,
	) (ExecutionCancellationReceipt, error)
}

type cancellationDispatchOutboxRepository interface {
	ListAvailableCancellationDispatches(
		context.Context,
		int,
	) ([]protocol.ExecutionCancellationDispatch, error)
	ClaimCancellationDispatch(
		context.Context,
		string,
		int64,
		string,
		time.Duration,
	) (*protocol.ExecutionCancellationDispatch, error)
	ResolveCancellationDispatch(
		context.Context,
		string,
		int64,
		string,
		protocol.ExecutionCancellationDispatchStatus,
		protocol.ExecutionCancellationOutcome,
		string,
		string,
	) (*protocol.ExecutionCancellationDispatch, error)
	RetryCancellationDispatch(
		context.Context,
		string,
		int64,
		string,
		time.Time,
		string,
	) (*protocol.ExecutionCancellationDispatch, error)
	GetSnapshot(context.Context, string) (*protocol.ExecutionSnapshot, error)
}

// CancellationDispatchRunResult 汇总一次有界 cancellation outbox drain。
type CancellationDispatchRunResult struct {
	Claimed     int
	Delivered   int
	Retried     int
	NotRequired int
	Unsupported int
}

// SetExecutionCancellationConsumer 注入 physical runtime cancellation 数据面。
func (s *Service) SetExecutionCancellationConsumer(
	consumer ExecutionCancellationConsumer,
) {
	s.cancellationConsumer = consumer
}

// DispatchPendingCancellations claim 并投递一批 Attempt physical cancellations。
func (s *Service) DispatchPendingCancellations(
	ctx context.Context,
	workerID string,
	limit int,
) (CancellationDispatchRunResult, error) {
	var result CancellationDispatchRunResult
	repository, ok := s.repository.(cancellationDispatchOutboxRepository)
	if !ok {
		return result, errors.New(
			"orchestration repository does not support cancellation dispatch outbox",
		)
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return result, errors.New("cancellation dispatch worker id is required")
	}
	candidates, err := repository.ListAvailableCancellationDispatches(ctx, limit)
	if err != nil {
		return result, err
	}
	for _, candidate := range candidates {
		claimed, claimErr := repository.ClaimCancellationDispatch(
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
		// Claim is itself durable and visible in the WorkGraph cancellation state.
		s.invalidateExecutionID(ctx, candidate.ExecutionID)
		status, deliveryErr := s.deliverClaimedCancellation(
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
		switch status {
		case protocol.ExecutionCancellationDispatchDelivered:
			result.Delivered++
		case protocol.ExecutionCancellationDispatchNotRequired:
			result.NotRequired++
		case protocol.ExecutionCancellationDispatchUnsupported:
			result.Unsupported++
		}
		s.invalidateExecutionID(ctx, candidate.ExecutionID)
	}
	return result, nil
}

func (s *Service) deliverClaimedCancellation(
	ctx context.Context,
	repository cancellationDispatchOutboxRepository,
	workerID string,
	dispatch *protocol.ExecutionCancellationDispatch,
) (protocol.ExecutionCancellationDispatchStatus, error) {
	if dispatch == nil {
		return "", errors.New("claimed cancellation dispatch is nil")
	}
	switch dispatch.TargetKind {
	case protocol.ExecutionCancellationTargetNotStarted:
		_, err := repository.ResolveCancellationDispatch(
			ctx,
			dispatch.ID,
			dispatch.Version,
			workerID,
			protocol.ExecutionCancellationDispatchNotRequired,
			protocol.ExecutionCancellationOutcomeNotStarted,
			firstNonEmpty(
				dispatch.LimitationCode,
				"attempt_not_started",
			),
			"Attempt never acquired a physical runtime target",
		)
		return protocol.ExecutionCancellationDispatchNotRequired, err
	case protocol.ExecutionCancellationTargetUnavailable:
		_, err := repository.ResolveCancellationDispatch(
			ctx,
			dispatch.ID,
			dispatch.Version,
			workerID,
			protocol.ExecutionCancellationDispatchUnsupported,
			protocol.ExecutionCancellationOutcomeUnsupported,
			firstNonEmpty(
				dispatch.LimitationCode,
				"runtime_identity_unavailable",
			),
			"physical runtime identity was unavailable at terminalization",
		)
		return protocol.ExecutionCancellationDispatchUnsupported, err
	case protocol.ExecutionCancellationTargetRoomSlot,
		protocol.ExecutionCancellationTargetRuntimeRound:
	default:
		return "", s.retryClaimedCancellation(
			ctx,
			repository,
			workerID,
			dispatch,
			fmt.Errorf("unknown cancellation target kind %q", dispatch.TargetKind),
		)
	}
	if s.cancellationConsumer == nil {
		return "", s.retryClaimedCancellation(
			ctx,
			repository,
			workerID,
			dispatch,
			errors.New("execution cancellation consumer is unavailable"),
		)
	}
	deliveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	receipt, err := s.cancellationConsumer.DeliverExecutionCancellation(
		deliveryCtx,
		ExecutionCancellationDelivery{
			Binding:      cancellationBinding(*dispatch),
			ExecutorKind: dispatch.ExecutorKind,
			Reason:       dispatch.Reason,
		},
	)
	if err != nil {
		return "", s.retryClaimedCancellation(
			ctx,
			repository,
			workerID,
			dispatch,
			err,
		)
	}
	resolutionStatus := protocol.ExecutionCancellationDispatchDelivered
	switch receipt.Outcome {
	case protocol.ExecutionCancellationOutcomeProviderInterrupted,
		protocol.ExecutionCancellationOutcomeLocalRoundCancelled,
		protocol.ExecutionCancellationOutcomeAlreadyEnded,
		protocol.ExecutionCancellationOutcomeStaleTarget:
	case protocol.ExecutionCancellationOutcomeUnsupported:
		resolutionStatus = protocol.ExecutionCancellationDispatchUnsupported
	default:
		return "", s.retryClaimedCancellation(
			ctx,
			repository,
			workerID,
			dispatch,
			fmt.Errorf(
				"cancellation consumer returned invalid outcome %q",
				receipt.Outcome,
			),
		)
	}
	_, err = repository.ResolveCancellationDispatch(
		ctx,
		dispatch.ID,
		dispatch.Version,
		workerID,
		resolutionStatus,
		receipt.Outcome,
		strings.TrimSpace(receipt.LimitationCode),
		strings.TrimSpace(receipt.Detail),
	)
	return resolutionStatus, err
}

func (s *Service) retryClaimedCancellation(
	ctx context.Context,
	repository cancellationDispatchOutboxRepository,
	workerID string,
	dispatch *protocol.ExecutionCancellationDispatch,
	cause error,
) error {
	delay := time.Second << min(dispatch.DeliveryAttempts-1, 6)
	_, retryErr := repository.RetryCancellationDispatch(
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

func cancellationBinding(
	dispatch protocol.ExecutionCancellationDispatch,
) protocol.ExecutionCancellationBinding {
	return protocol.ExecutionCancellationBinding{
		ExecutionID:       dispatch.ExecutionID,
		PlanID:            dispatch.PlanID,
		WorkItemID:        dispatch.WorkItemID,
		SpecID:            dispatch.SpecID,
		AssignmentID:      dispatch.AssignmentID,
		AttemptID:         dispatch.AttemptID,
		RuntimeAttemptID:  dispatch.RuntimeAttemptID,
		DispatchID:        dispatch.DispatchID,
		TargetKind:        dispatch.TargetKind,
		TargetAgentID:     dispatch.TargetAgentID,
		ScopeSessionKey:   dispatch.ScopeSessionKey,
		RoomID:            dispatch.RoomID,
		ConversationID:    dispatch.ConversationID,
		RuntimeSessionKey: dispatch.RuntimeSessionKey,
		RuntimeRoundID:    dispatch.RuntimeRoundID,
		AgentRoundID:      dispatch.AgentRoundID,
	}
}
