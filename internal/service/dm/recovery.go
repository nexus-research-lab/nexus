// INPUT: Authoritative Execution recovery receipts or exact DM terminal outcomes.
// OUTPUT: Durable hidden recovery input and root Attempt settlement; a deleted coordinator Agent reports an unavailable target.
// POS: DM lifecycle adapter; pause gates and dispatch admission remain authoritative.
package dm

import (
	"context"
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) QueueExecutionRecovery(ctx context.Context, e protocol.Execution, key string) error {
	return s.queueExecutionRecovery(ctx, e, key, nil)
}

func (s *Service) QueueWorkRecovery(ctx context.Context, e protocol.Execution, target protocol.ExecutionRecoveryTarget, key string) error {
	return s.queueExecutionRecovery(ctx, e, key, &target)
}

func (s *Service) FindWorkRecovery(ctx context.Context, e protocol.Execution, key string) (*protocol.ExecutionRecoveryTarget, error) {
	ctx = runtimehost.ContextWithExactOwner(ctx, e.OwnerUserID)
	_, location, err := s.resolveInputQueueLocation(ctx, e.SessionKey, e.CoordinatorAgentID)
	if err != nil {
		return nil, err
	}
	item, found, err := s.InputQueue.FindAcceptedEnqueue(location, key)
	if err != nil || !found {
		return nil, err
	}
	return item.RecoveryTarget, nil
}

func (s *Service) queueExecutionRecovery(ctx context.Context, e protocol.Execution, key string, target *protocol.ExecutionRecoveryTarget) error {
	ctx = runtimehost.ContextWithExactOwner(ctx, e.OwnerUserID)
	sessionKey, location, err := s.resolveInputQueueLocation(ctx, e.SessionKey, e.CoordinatorAgentID)
	if errors.Is(err, agentsvc.ErrAgentNotFound) {
		return fmt.Errorf("%w: coordinator Agent %s no longer exists", orchestrationsvc.ErrExecutionRecoveryTargetUnavailable, e.CoordinatorAgentID)
	}
	if err != nil {
		return err
	}
	content := runtimehost.ExecutionRecoveryInstruction(e)
	if target != nil {
		content = runtimehost.WorkRecoveryInstruction(e, *target)
	}
	_, err = s.InputQueue.EnqueueIdempotent(location, protocol.InputQueueItem{
		Source: protocol.InputQueueSourceHostRecovery, RecoveryExecutionID: e.ID,
		RecoveryTarget: target,
		OwnerUserID:    e.OwnerUserID, AgentID: e.CoordinatorAgentID, Content: content,
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}, key)
	if err != nil {
		return err
	}
	if runtimehost.RecoveryDispatchDeferred(ctx) {
		return nil
	}
	s.StartSessionBackgroundTask(sessionKey, e.OwnerUserID, func(ctx context.Context) {
		s.dispatchNextInputQueueItemAtLocation(ctx, sessionKey, e.CoordinatorAgentID, location)
	})
	return nil
}

type dmAttemptTerminalizer interface {
	FinishDMRoundAttempts(context.Context, orchestrationsvc.ActorContext, protocol.WorkAttemptStatus, string) error
}

func (r *roundRunner) finishWorkAttempts(status protocol.WorkAttemptStatus, reason string) {
	if r.userStoppedRound() {
		status = protocol.WorkAttemptStatusInterrupted
		reason = message.InterruptWithoutMessage
	} else if r.explicitlyStoppedRound() {
		status = protocol.WorkAttemptStatusInterrupted
		reason = protocol.WorkAttemptReasonHostControlStop
	}
	provider, ok := r.service.ExecutionContext.(dmAttemptTerminalizer)
	if !ok {
		return
	}
	if err := provider.FinishDMRoundAttempts(context.Background(), r.orchestrationActor(), status, reason); err != nil {
		r.service.LoggerFor(context.Background()).Error("DM root Attempt settlement failed", "round_id", r.roundID, "err", err)
	}
}

func (s *Service) RecoverInputQueues(ctx context.Context) error {
	return s.VisitInputQueues(ctx, protocol.InputQueueScopeDM, func(ctx context.Context, location workspacestore.InputQueueLocation) {
		s.StartSessionBackgroundTask(location.SessionKey, location.OwnerUserID, func(ctx context.Context) {
			s.dispatchNextInputQueueItemAtLocation(ctx, location.SessionKey, inputQueueLocationAgentID(location), location)
		})
	})
}
