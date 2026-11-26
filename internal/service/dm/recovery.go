// INPUT: Authoritative Execution recovery receipts or exact DM terminal outcomes.
// OUTPUT: Durable hidden recovery input (main window inbox or JSONL queue) and root Attempt settlement; a deleted coordinator Agent reports an unavailable target.
// POS: DM lifecycle adapter; pause gates and dispatch admission remain authoritative. 主窗口的恢复输入只进 inbox，启动扫描跳过主窗口的 JSONL 队列。
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
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
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
	inbox, mainWindow, err := s.mainInboxForExecution(ctx, e.SessionKey, e.CoordinatorAgentID)
	if err != nil {
		return nil, err
	}
	if mainWindow {
		return s.findMainInboxRecovery(ctx, inbox, key)
	}
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
	inbox, mainWindow, err := s.mainInboxForExecution(ctx, e.SessionKey, e.CoordinatorAgentID)
	var sessionKey string
	var location workspacestore.InputQueueLocation
	if err == nil && !mainWindow {
		sessionKey, location, err = s.resolveInputQueueLocation(ctx, e.SessionKey, e.CoordinatorAgentID)
	}
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
	if mainWindow {
		_, err = s.enqueueMainInboxRecovery(ctx, inbox, key, content, inboxstore.RecoverySource{ExecutionID: e.ID, Target: target})
		return err
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

// RecoverInputQueues 在启动结算之后接续积压：主窗口交给 inbox 定时器重扫（结算期间入队的恢复被推迟唤醒），
// 其余 DM 派发各自的 JSONL 队列。
func (s *Service) RecoverInputQueues(ctx context.Context) error {
	s.notifyMainInboxTimer()
	return s.VisitInputQueues(ctx, protocol.InputQueueScopeDM, func(ctx context.Context, location workspacestore.InputQueueLocation) {
		if _, mainWindow, err := s.mainInboxForExecution(ctx, location.SessionKey, ""); err == nil && mainWindow {
			return
		}
		s.StartSessionBackgroundTask(location.SessionKey, location.OwnerUserID, func(ctx context.Context) {
			s.dispatchNextInputQueueItemAtLocation(ctx, location.SessionKey, inputQueueLocationAgentID(location), location)
		})
	})
}
