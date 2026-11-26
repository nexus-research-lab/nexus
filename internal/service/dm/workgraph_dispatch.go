// INPUT: Host-issued exact WorkGraph responsibility and stable durable queue identity.
// OUTPUT: Hidden DM work/review rounds with exact continuation admission through the pause gate (main window inbox or JSONL queue).
// POS: DM counterpart of Room structured dispatch; model input cannot construct bindings.
package dm

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
)

func (s *Service) AutomaticDispatchAllowed(ctx context.Context, e protocol.Execution, target string) (bool, error) {
	if e.ScopeKind != protocol.ExecutionScopeDM || e.CoordinatorAgentID != target {
		return false, fmt.Errorf("%w: scheduled DM target differs from coordinator", orchestration.ErrScheduledTargetUnavailable)
	}
	ctx = runtimehost.ContextWithExactOwner(ctx, e.OwnerUserID)
	inbox, mainWindow, err := s.mainInboxForExecution(ctx, e.SessionKey, target)
	if err != nil {
		return false, err
	}
	if mainWindow {
		gate, gateErr := s.inbox.Gate(ctx, inbox.sessionKey)
		return !gate.Paused, gateErr
	}
	_, location, err := s.resolveInputQueueLocation(ctx, e.SessionKey, target)
	if err != nil {
		return false, err
	}
	paused, err := s.InputQueue.DispatchPaused(location)
	return !paused, err
}
func (s *Service) QueueScheduledWork(ctx context.Context, e protocol.Execution, binding protocol.ExecutionWorkBinding, instruction, key string) error {
	if e.ScopeKind != protocol.ExecutionScopeDM {
		return errors.New("scheduled work requires a DM Execution")
	}
	content := fmt.Sprintf("Execute only the bound Work Item %s in Execution %s. Read its current contract and accepted dependencies. Deliver an explicit result for review; do not assign other work. %s", binding.WorkItemID, e.ID, instruction)
	_, err := s.enqueueWorkGraphRound(ctx, e.OwnerUserID, e.SessionKey, e.CoordinatorAgentID, key, content, &binding, nil)
	return err
}
func (s *Service) DeliverExecutionReviewDispatch(ctx context.Context, d orchestration.ExecutionReviewDispatchDelivery) (orchestration.ExecutionReviewDispatchReceipt, error) {
	if d.RoomID != "" || d.ConversationID != "" || d.TargetAgentID != d.Binding.TargetAgentID {
		return orchestration.ExecutionReviewDispatchReceipt{}, errors.New("invalid DM review delivery")
	}
	id, err := s.enqueueWorkGraphRound(ctx, d.OwnerUserID, d.SessionKey, d.TargetAgentID, d.DispatchDedupeKey, d.Instruction, nil, &d.Binding)
	return orchestration.ExecutionReviewDispatchReceipt{QueueItemID: id}, err
}
func (s *Service) enqueueWorkGraphRound(ctx context.Context, owner, session, agent, key, content string, work *protocol.ExecutionWorkBinding, review *protocol.ExecutionReviewBinding, continuationRevision ...int64) (string, error) {
	ctx = runtimehost.ContextWithExactOwner(ctx, owner)
	source := workGraphRecoverySource(work, review, continuationRevision...)
	inbox, mainWindow, err := s.mainInboxForExecution(ctx, session, agent)
	if err != nil {
		return "", err
	}
	if mainWindow {
		event, enqueueErr := s.enqueueMainInboxRecovery(ctx, inbox, key, content, source)
		return event.EventID, enqueueErr
	}
	session, location, err := s.resolveInputQueueLocation(ctx, session, agent)
	if err != nil {
		return "", err
	}
	result, err := s.InputQueue.EnqueueIdempotent(location, protocol.InputQueueItem{ContinuationPauseRevision: source.PauseRevision, RecoveryTarget: source.Target, Source: protocol.InputQueueSourceHostRecovery, RecoveryExecutionID: source.ExecutionID, OwnerUserID: owner, AgentID: agent, TargetAgentIDs: []string{agent}, Content: content, DeliveryPolicy: protocol.ChatDeliveryPolicyQueue, WorkBinding: work, ReviewBinding: review}, key)
	if err != nil {
		return "", err
	}
	s.StartSessionBackgroundTask(session, owner, func(ctx context.Context) { s.dispatchNextInputQueueItemAtLocation(ctx, session, agent, location) })
	return result.Item.ID, nil
}

// workGraphRecoverySource 从宿主签发的绑定推出恢复身份；只有续跑带闸门代数，并以自身绑定为显式目标。
func workGraphRecoverySource(work *protocol.ExecutionWorkBinding, review *protocol.ExecutionReviewBinding, continuationRevision ...int64) inboxstore.RecoverySource {
	source := inboxstore.RecoverySource{Work: work, Review: review}
	if work != nil {
		source.ExecutionID = work.ExecutionID
	}
	if review != nil {
		source.ExecutionID = review.ExecutionID
	}
	if len(continuationRevision) > 0 && work != nil {
		source.PauseRevision = &continuationRevision[0]
		source.Target = &protocol.ExecutionRecoveryTarget{ExecutionID: work.ExecutionID, WorkItemID: work.WorkItemID, AttemptID: work.AttemptID}
	}
	return source
}

// QueueContinuedWork admits this explicit task without unpausing unrelated inputs.
func (s *Service) QueueContinuedWork(ctx context.Context, e protocol.Execution, b protocol.ExecutionWorkBinding, instruction, key string, revision int64) error {
	if e.ScopeKind != protocol.ExecutionScopeDM {
		return errors.New("continuation requires DM scope")
	}
	_, err := s.enqueueWorkGraphRound(ctx, e.OwnerUserID, e.SessionKey, e.CoordinatorAgentID, key, instruction, &b, nil, revision)
	return err
}

func (s *Service) ContinuationPauseRevision(ctx context.Context, e protocol.Execution, target string) (int64, error) {
	ctx = runtimehost.ContextWithExactOwner(ctx, e.OwnerUserID)
	inbox, mainWindow, err := s.mainInboxForExecution(ctx, e.SessionKey, target)
	if err != nil {
		return 0, err
	}
	if mainWindow {
		gate, gateErr := s.inbox.Gate(ctx, inbox.sessionKey)
		return gate.Revision, gateErr
	}
	_, location, err := s.resolveInputQueueLocation(ctx, e.SessionKey, target)
	if err != nil {
		return 0, err
	}
	return s.InputQueue.ContinuationPauseRevision(location)
}
