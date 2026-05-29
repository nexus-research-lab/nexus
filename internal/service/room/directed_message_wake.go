package room

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomDirectedMessageTriggerType = "room_directed_message"

func (s *RealtimeService) startRoomDirectedMessageWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	message protocol.RoomDirectedMessageRecord,
) error {
	if contextValue == nil {
		return nil
	}
	if message.WakePolicy == protocol.RoomWakePolicyNone {
		return nil
	}
	if message.WakePolicy == protocol.RoomWakePolicyDelayed {
		s.scheduleRoomDirectedMessageWake(ctx, message)
		return nil
	}
	return s.runRoomDirectedMessageWake(ctx, contextValue, message)
}

func (s *RealtimeService) runRoomDirectedMessageWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	message protocol.RoomDirectedMessageRecord,
) error {
	wakeContent, ok := roomDirectedMessageWakeContent(message)
	if !ok {
		return nil
	}
	targetAgentIDs := roomDirectedMessageWakeTargetAgentIDs(message)
	if len(targetAgentIDs) == 0 {
		return nil
	}
	parentRound := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(message.ConversationID),
		RoomID:         message.RoomID,
		ConversationID: message.ConversationID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        message.MessageID,
		RootRoundID:    message.MessageID,
		OwnerUserID:    authctx.OwnerUserID(ctx),
	}
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		wakes = append(wakes, publicMentionWake{
			TriggerType:   roomDirectedMessageTriggerType,
			QueueSource:   protocol.InputQueueSourceAgentRoomMessage,
			SourceAgentID: strings.TrimSpace(message.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       wakeContent,
			MessageID:     strings.TrimSpace(message.MessageID),
			ReplyRoute:    message.ReplyRoute,
		})
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}

func (s *RealtimeService) scheduleRoomDirectedMessageWake(ctx context.Context, message protocol.RoomDirectedMessageRecord) {
	delay := time.Duration(message.DelaySeconds) * time.Second
	if delay <= 0 {
		return
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(message.ConversationID)
	s.broadcastSharedEventWithTimeout(ctx, sessionKey, message.RoomID, newRoomDirectedMessageScheduledWakeEvent(message))
	ownerUserID := authctx.OwnerUserID(ctx)
	s.loggerFor(ctx).Info("Room directed message 延迟唤醒已计划",
		"room_id", message.RoomID,
		"conversation_id", message.ConversationID,
		"message_id", message.MessageID,
		"recipient_agent_ids", message.Recipients,
		"delay_seconds", message.DelaySeconds,
	)
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C

		wakeCtx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
			UserID:     strings.TrimSpace(ownerUserID),
			Username:   strings.TrimSpace(ownerUserID),
			Role:       authctx.RoleOwner,
			AuthMethod: "room_directed_message_delayed",
		})
		contextValue, err := s.resolveDirectedMessageContext(wakeCtx, message.RoomID, message.ConversationID)
		if err != nil {
			s.loggerFor(wakeCtx).Error("Room directed message 延迟唤醒解析上下文失败",
				"room_id", message.RoomID,
				"conversation_id", message.ConversationID,
				"message_id", message.MessageID,
				"err", err,
			)
			return
		}
		if err = s.runRoomDirectedMessageWake(wakeCtx, contextValue, message); err != nil {
			s.loggerFor(wakeCtx).Error("执行 Room directed message 延迟唤醒失败",
				"room_id", message.RoomID,
				"conversation_id", message.ConversationID,
				"message_id", message.MessageID,
				"recipient_agent_ids", message.Recipients,
				"err", err,
			)
		}
	}()
}

func roomDirectedMessageWakeContent(message protocol.RoomDirectedMessageRecord) (string, bool) {
	if message.WakePolicy != protocol.RoomWakePolicyImmediate &&
		message.WakePolicy != protocol.RoomWakePolicyDelayed {
		return "", false
	}
	return "A Room directed message was delivered to you. Read the content projected in <room_directed_messages> and answer according to reply_route.", true
}

func roomDirectedMessageWakeTargetAgentIDs(message protocol.RoomDirectedMessageRecord) []string {
	result := make([]string, 0, len(message.Recipients))
	for _, agentID := range message.Recipients {
		normalized := strings.TrimSpace(agentID)
		if normalized == "" || containsRoomDirectedMessageWakeTarget(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func containsRoomDirectedMessageWakeTarget(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
