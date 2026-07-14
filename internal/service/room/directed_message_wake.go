package room

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const roomDirectedMessageTriggerType = "room_directed_message"
const roomDirectedMessageWakeRetryDelay = 30 * time.Second

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
		return s.scheduleRoomDirectedMessageWake(ctx, message)
	}
	return s.runRoomDirectedMessageWake(ctx, contextValue, message)
}

func (s *RealtimeService) runRoomDirectedMessageWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	message protocol.RoomDirectedMessageRecord,
) error {
	return s.enqueueRoomDirectedMessageWake(ctx, contextValue, message)
}

func (s *RealtimeService) scheduleRoomDirectedMessageWake(ctx context.Context, message protocol.RoomDirectedMessageRecord) error {
	delay := time.Duration(message.DelaySeconds) * time.Second
	if delay <= 0 {
		return errors.New("delay_seconds must be positive")
	}
	if s.directedWakes == nil {
		return errors.New("room directed wake store is not configured")
	}
	wake := workspacestore.RoomDirectedMessageWake{
		WakeID:      strings.TrimSpace(message.MessageID),
		OwnerUserID: authctx.OwnerUserID(ctx),
		Message:     message,
		DueAt:       time.Now().Add(delay).UnixMilli(),
		CreatedAt:   time.Now().UnixMilli(),
	}
	if err := s.directedWakes.Schedule(wake); err != nil {
		return err
	}
	s.schedulePersistedRoomDirectedWake(wake, delay)
	sessionKey := protocol.BuildRoomSharedSessionKey(message.ConversationID)
	s.broadcastSharedEventWithTimeout(ctx, sessionKey, message.RoomID, newRoomDirectedMessageScheduledWakeEvent(message))
	s.loggerFor(ctx).Info("Room directed message 延迟唤醒已计划",
		"room_id", message.RoomID,
		"conversation_id", message.ConversationID,
		"message_id", message.MessageID,
		"recipient_agent_ids", message.Recipients,
		"delay_seconds", message.DelaySeconds,
	)
	return nil
}

// StartDelayedWakeScheduler 恢复宕机前未完成的 Room 延迟唤醒。
func (s *RealtimeService) StartDelayedWakeScheduler(context.Context) (func(), error) {
	if s.directedWakes == nil {
		return nil, nil
	}
	pending, err := s.directedWakes.Pending()
	if err != nil {
		return nil, err
	}
	s.wakeTimers.Start()
	for _, wake := range pending {
		delay := time.Until(time.UnixMilli(wake.DueAt))
		if delay < 0 {
			delay = 0
		}
		s.schedulePersistedRoomDirectedWake(wake, delay)
	}
	return s.stopRoomWakeSchedulers, nil
}

func (s *RealtimeService) schedulePersistedRoomDirectedWake(
	wake workspacestore.RoomDirectedMessageWake,
	delay time.Duration,
) {
	wakeID := strings.TrimSpace(wake.WakeID)
	if wakeID == "" {
		return
	}
	s.wakeTimers.ScheduleDelayed(wakeID, delay, func() {
		s.executePersistedRoomDirectedWake(wake)
	})
}

func (s *RealtimeService) executePersistedRoomDirectedWake(wake workspacestore.RoomDirectedMessageWake) {
	wakeCtx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     strings.TrimSpace(wake.OwnerUserID),
		Username:   strings.TrimSpace(wake.OwnerUserID),
		Role:       authctx.RoleOwner,
		AuthMethod: "room_directed_message_delayed",
	})
	message := wake.Message
	contextValue, err := s.resolveDirectedMessageContext(wakeCtx, message.RoomID, message.ConversationID)
	if err == nil {
		err = s.runRoomDirectedMessageWake(wakeCtx, contextValue, message)
	}
	if err != nil {
		s.loggerFor(wakeCtx).Error("执行 Room directed message 延迟唤醒失败，稍后重试",
			"room_id", message.RoomID,
			"conversation_id", message.ConversationID,
			"message_id", message.MessageID,
			"err", err,
		)
		s.schedulePersistedRoomDirectedWake(wake, roomDirectedMessageWakeRetryDelay)
		return
	}
	if err = s.directedWakes.Complete(wake.WakeID); err != nil {
		s.loggerFor(wakeCtx).Error("记录 Room directed message 延迟唤醒完成失败", "wake_id", wake.WakeID, "err", err)
	}
}

func (s *RealtimeService) stopRoomWakeSchedulers() {
	s.wakeTimers.Stop()
}

func roomDirectedMessageWakeContent(message protocol.RoomDirectedMessageRecord) (string, bool) {
	if message.WakePolicy != protocol.RoomWakePolicyImmediate &&
		message.WakePolicy != protocol.RoomWakePolicyDelayed {
		return "", false
	}
	return "A Room directed message was delivered to you. Read the content projected in <room_directed_messages> and answer according to reply_route.", true
}

func roomDirectedMessageWakeTargetAgentIDs(message protocol.RoomDirectedMessageRecord) []string {
	targets := message.WakeTargets
	if len(targets) == 0 && message.WakePolicy != protocol.RoomWakePolicyNone {
		targets = message.Recipients
	}
	result := make([]string, 0, len(targets))
	for _, agentID := range targets {
		normalized := strings.TrimSpace(agentID)
		if normalized == "" || slices.Contains(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}
