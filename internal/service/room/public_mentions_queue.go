package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) queueBusyPublicMentionWakes(
	ctx context.Context,
	parentRound *activeRoomRound,
	sessionKey string,
	wakes []publicMentionWake,
) ([]publicMentionWake, error) {
	if parentRound == nil || len(wakes) == 0 {
		return wakes, nil
	}
	targetAgentIDs := make([]string, 0, len(wakes))
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID != "" {
			targetAgentIDs = append(targetAgentIDs, targetAgentID)
		}
	}
	busySlots := s.findActiveDeliverySlotsByAgent(sessionKey, parentRound.ConversationID, targetAgentIDs)
	if len(busySlots) == 0 {
		return wakes, nil
	}

	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, parentRound.Context)
	if err != nil {
		return nil, err
	}
	ready := make([]publicMentionWake, 0, len(wakes))
	queued := false
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID == "" {
			continue
		}
		if _, busy := busySlots[targetAgentID]; !busy {
			ready = append(ready, wake)
			continue
		}
		location, ok := locationsByAgentID[targetAgentID]
		if !ok {
			s.loggerFor(ctx).Warn("Room 公区 @ 目标正忙但缺少队列位置",
				"s", sessionKey,
				"r", parentRound.RoomID,
				"c", parentRound.ConversationID,
				"t", targetAgentID,
			)
			continue
		}
		if _, err := s.inputQueue.Enqueue(location.Location, protocol.InputQueueItem{
			Scope:           protocol.InputQueueScopeRoom,
			SessionKey:      location.Location.SessionKey,
			RoomID:          parentRound.RoomID,
			ConversationID:  parentRound.ConversationID,
			AgentID:         targetAgentID,
			SourceAgentID:   strings.TrimSpace(wake.SourceAgentID),
			SourceMessageID: strings.TrimSpace(wake.MessageID),
			TargetAgentIDs:  []string{targetAgentID},
			Source:          normalizeWakeQueueSource(wake),
			Content:         strings.TrimSpace(wake.Content),
			DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
			ReplyRoute:      wake.ReplyRoute,
			OwnerUserID:     parentRound.OwnerUserID,
			RootRoundID:     roomRootRoundID(parentRound),
			HopIndex:        parentRound.HopIndex,
		}); err != nil {
			return nil, err
		}
		queued = true
		s.loggerFor(ctx).Info(roomWakeQueuedLogMessage(wake),
			"s", sessionKey,
			"qs", location.Location.SessionKey,
			"r", parentRound.RoomID,
			"c", parentRound.ConversationID,
			"src", wake.SourceAgentID,
			"t", targetAgentID,
		)
		if normalizeWakeQueueSource(wake) == protocol.InputQueueSourceAgentRoomMessage {
			s.broadcastSharedEventWithTimeout(ctx, sessionKey, parentRound.RoomID, newRoomDirectedMessageWakeEvent(parentRound, wake, "wake_queued", map[string]any{
				"queue_session_key": location.Location.SessionKey,
			}))
		}
	}
	if queued {
		if err := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, parentRound.Context); err != nil {
			return nil, err
		}
	}
	return ready, nil
}
