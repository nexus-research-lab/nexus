// INPUT: Host-issued exact WorkGraph responsibility and stable durable queue identity.
// OUTPUT: Hidden DM work/review rounds honoring the existing queue pause gate.
// POS: DM counterpart of Room structured dispatch; model input cannot construct bindings.
package dm

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
)

func (s *Service) AutomaticDispatchAllowed(ctx context.Context, e protocol.Execution, target string) (bool, error) {
	if e.ScopeKind != protocol.ExecutionScopeDM || e.CoordinatorAgentID != target {
		return false, errors.New("scheduled DM target differs from coordinator")
	}
	ctx = runtimehost.ContextWithExactOwner(ctx, e.OwnerUserID)
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
func (s *Service) enqueueWorkGraphRound(ctx context.Context, owner, session, agent, key, content string, work *protocol.ExecutionWorkBinding, review *protocol.ExecutionReviewBinding) (string, error) {
	ctx = runtimehost.ContextWithExactOwner(ctx, owner)
	session, location, err := s.resolveInputQueueLocation(ctx, session, agent)
	if err != nil {
		return "", err
	}
	executionID := ""
	if work != nil {
		executionID = work.ExecutionID
	}
	if review != nil {
		executionID = review.ExecutionID
	}
	result, err := s.InputQueue.EnqueueIdempotent(location, protocol.InputQueueItem{Source: protocol.InputQueueSourceHostRecovery, RecoveryExecutionID: executionID, OwnerUserID: owner, AgentID: agent, TargetAgentIDs: []string{agent}, Content: content, DeliveryPolicy: protocol.ChatDeliveryPolicyQueue, WorkBinding: work, ReviewBinding: review}, key)
	if err != nil {
		return "", err
	}
	s.StartSessionBackgroundTask(session, owner, func(ctx context.Context) { s.dispatchNextInputQueueItemAtLocation(ctx, session, agent, location) })
	return result.Item.ID, nil
}
