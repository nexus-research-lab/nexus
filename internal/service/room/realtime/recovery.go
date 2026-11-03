// INPUT: Authoritative Room Execution and durable recovery identity.
// OUTPUT: Coordinator recovery queue entry without user impersonation; a deleted conversation or departed coordinator reports an unavailable target.
// POS: Room WorkGraph recovery routing through ordinary member dispatch gates.
package realtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) QueueExecutionRecovery(ctx context.Context, e protocol.Execution, key string) error {
	ctx, value, err := s.internalConversationContext(ctx, e.ConversationID, true)
	if errors.Is(err, roomsvc.ErrRoomNotFound) || errors.Is(err, roomsvc.ErrConversationNotFound) || err == nil && value == nil {
		return fmt.Errorf("%w: Room conversation %s no longer exists", orchestrationsvc.ErrExecutionRecoveryTargetUnavailable, e.ConversationID)
	}
	if err != nil {
		return err
	}
	if value.Room.OwnerUserID != e.OwnerUserID {
		return errors.New("recovery owner mismatch")
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, value)
	if err != nil {
		return err
	}
	target, ok := locations[e.CoordinatorAgentID]
	if !ok {
		return fmt.Errorf("%w: coordinator %s is no longer a Room member", orchestrationsvc.ErrExecutionRecoveryTargetUnavailable, e.CoordinatorAgentID)
	}
	_, err = s.InputQueue.EnqueueIdempotent(target.Location, protocol.InputQueueItem{
		SessionKey: target.Location.SessionKey, Source: protocol.InputQueueSourceHostRecovery, RecoveryExecutionID: e.ID,
		OwnerUserID: e.OwnerUserID, AgentID: e.CoordinatorAgentID, TargetAgentIDs: []string{e.CoordinatorAgentID},
		Content: runtimehost.ExecutionRecoveryInstruction(e), DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}, key)
	if err != nil {
		return err
	}
	if runtimehost.RecoveryDispatchDeferred(ctx) {
		return nil
	}
	s.StartSessionBackgroundTask(e.SessionKey, e.OwnerUserID, func(ctx context.Context) { s.dispatchNextInputQueueItem(ctx, e.SessionKey, e.RoomID, e.ConversationID) })
	return nil
}

func (s *Service) RecoverInputQueues(ctx context.Context) error {
	seen := map[string]bool{}
	return s.VisitInputQueues(ctx, protocol.InputQueueScopeRoom, func(ctx context.Context, location workspacestore.InputQueueLocation) {
		if location.ConversationID == "" || seen[location.ConversationID] {
			return
		}
		seen[location.ConversationID] = true
		session := protocol.BuildRoomSharedSessionKey(location.ConversationID)
		s.StartSessionBackgroundTask(session, location.OwnerUserID, func(ctx context.Context) {
			s.dispatchNextInputQueueItem(ctx, session, location.RoomID, location.ConversationID)
		})
	})
}
