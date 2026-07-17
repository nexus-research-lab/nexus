// INPUT: 已持久化的 Room directed message 与其唤醒目标。
// OUTPUT: 去重、有界、可过期且能短窗口合并的 Agent 唤醒队列。
// POS: directed message 从“可见记录”进入“执行轮次”的唯一入口。
package room

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const (
	roomDirectedWakeQueueCapacity = 64
	roomDirectedWakeQueueTTL      = 24 * time.Hour
	roomDirectedWakeBatchWindow   = 200 * time.Millisecond
)

func (s *RealtimeService) enqueueRoomDirectedMessageWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	message protocol.RoomDirectedMessageRecord,
) error {
	wakeContent, ok := roomDirectedMessageWakeContent(message)
	if !ok || contextValue == nil {
		return nil
	}
	targetAgentIDs := roomDirectedMessageWakeTargetAgentIDs(message)
	if len(targetAgentIDs) == 0 {
		return nil
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return err
	}
	now := time.Now()
	accepted := false
	for _, targetAgentID := range targetAgentIDs {
		location, exists := locations[targetAgentID]
		if !exists {
			continue
		}
		_, inserted, enqueueErr := s.inputQueue.EnqueueBounded(location.Location, protocol.InputQueueItem{
			Scope:           protocol.InputQueueScopeRoom,
			SessionKey:      location.Location.SessionKey,
			RoomID:          message.RoomID,
			ConversationID:  message.ConversationID,
			AgentID:         targetAgentID,
			SourceAgentID:   strings.TrimSpace(message.SourceAgentID),
			SourceMessageID: strings.TrimSpace(message.MessageID),
			TargetAgentIDs:  []string{targetAgentID},
			Source:          protocol.InputQueueSourceAgentRoomMessage,
			Content:         wakeContent,
			DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
			ReplyRoute:      message.ReplyRoute,
			OwnerUserID:     authctx.OwnerUserID(ctx),
			RootRoundID:     firstNonEmptyString(message.RootRoundID, message.MessageID),
			HopIndex:        message.HopIndex,
			ExpiresAt:       now.Add(roomDirectedWakeQueueTTL).UnixMilli(),
		}, roomDirectedWakeQueueCapacity)
		if enqueueErr != nil {
			return enqueueErr
		}
		accepted = accepted || inserted
	}
	if !accepted {
		return nil
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(message.ConversationID)
	if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
		return err
	}
	s.scheduleRoomDirectedQueueDispatch(
		contextWithQueueOwner(context.Background(), authctx.OwnerUserID(ctx)),
		sessionKey,
		message.RoomID,
		message.ConversationID,
	)
	return nil
}

func (s *RealtimeService) scheduleRoomDirectedQueueDispatch(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return
	}
	s.wakeTimers.ScheduleDispatch(key, roomDirectedWakeBatchWindow, func() {
		s.dispatchNextInputQueueItem(ctx, sessionKey, roomID, conversationID)
	})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}
