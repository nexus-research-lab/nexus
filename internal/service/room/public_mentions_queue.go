// INPUT: Agent 公区 @ / Room 定向消息唤醒与目标 Agent 当前活跃 slot。
// OUTPUT: 公区 @ 对 busy 目标优先绑定当前轮 guide，其他唤醒排队；idle 目标继续立即开新轮。
// POS: Agent 唤醒进入 Room runtime 前的 busy/idle 分流与 durable queue 登记点。
package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
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
	dispatchQueued := false
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID == "" {
			continue
		}
		busySlot := busySlots[targetAgentID]
		if busySlot == nil {
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
		queueSource := normalizeWakeQueueSource(wake)
		deliveryPolicy := protocol.ChatDeliveryPolicyQueue
		rootRoundID := roomRootRoundID(parentRound)
		if queueSource == protocol.InputQueueSourceAgentPublicMention {
			// 公区 @ 已经是目标 Agent 可见的新上下文。目标忙碌时先绑定它
			// 当前 slot 的 PostToolUse hook；只有 hook 没有消费，slot 收尾才会
			// 把它降级为普通 queue 并续开下一轮，避免同 Agent 并发第二个 slot。
			deliveryPolicy = protocol.ChatDeliveryPolicyGuide
			rootRoundID = strings.TrimSpace(busySlot.AgentRoundID)
		}
		queuedItemID := workspacestore.NewInputQueueID()
		queuedItem := protocol.InputQueueItem{
			ID:              queuedItemID,
			Scope:           protocol.InputQueueScopeRoom,
			SessionKey:      location.Location.SessionKey,
			RoomID:          parentRound.RoomID,
			ConversationID:  parentRound.ConversationID,
			AgentID:         targetAgentID,
			SourceAgentID:   strings.TrimSpace(wake.SourceAgentID),
			SourceMessageID: strings.TrimSpace(wake.MessageID),
			HandoffID:       strings.TrimSpace(wake.HandoffID),
			TargetAgentIDs:  []string{targetAgentID},
			Source:          queueSource,
			Content:         strings.TrimSpace(wake.Content),
			DeliveryPolicy:  deliveryPolicy,
			ReplyRoute:      wake.ReplyRoute,
			OwnerUserID:     parentRound.OwnerUserID,
			RootRoundID:     rootRoundID,
			HopIndex:        parentRound.HopIndex,
		}
		queueItems, inserted, err := s.inputQueue.EnqueueBounded(location.Location, queuedItem, 0)
		if err != nil {
			return nil, err
		}
		if !inserted {
			for _, existing := range queueItems {
				if strings.TrimSpace(existing.HandoffID) == strings.TrimSpace(wake.HandoffID) ||
					(existing.Source == queuedItem.Source &&
						strings.TrimSpace(existing.SourceMessageID) == strings.TrimSpace(queuedItem.SourceMessageID) &&
						strings.TrimSpace(existing.AgentID) == strings.TrimSpace(queuedItem.AgentID)) {
					queuedItem = existing
					queuedItemID = existing.ID
					break
				}
			}
		}
		if s.publicHandoffs != nil && strings.TrimSpace(wake.HandoffID) != "" {
			if err := s.publicHandoffs.MarkQueued(parentRound.ConversationID, wake.HandoffID, queuedItemID); err != nil {
				return nil, err
			}
		}
		if deliveryPolicy == protocol.ChatDeliveryPolicyGuide && !isActiveDeliverySlot(busySlot) {
			if _, err := s.inputQueue.UpdateDeliveryPolicy(
				location.Location,
				queuedItemID,
				protocol.ChatDeliveryPolicyQueue,
			); err != nil {
				return nil, err
			}
			deliveryPolicy = protocol.ChatDeliveryPolicyQueue
			dispatchQueued = true
		}
		queued = true
		s.loggerFor(ctx).Info(roomWakeQueuedLogMessage(wake),
			"s", sessionKey,
			"qs", location.Location.SessionKey,
			"r", parentRound.RoomID,
			"c", parentRound.ConversationID,
			"src", wake.SourceAgentID,
			"t", targetAgentID,
			"active_round_id", busySlot.AgentRoundID,
			"delivery_policy", deliveryPolicy,
		)
		if queueSource == protocol.InputQueueSourceAgentRoomMessage {
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
	if dispatchQueued {
		go s.dispatchNextInputQueueItem(
			contextWithQueueOwner(context.Background(), parentRound.OwnerUserID),
			sessionKey,
			parentRound.RoomID,
			parentRound.ConversationID,
		)
	}
	return ready, nil
}
