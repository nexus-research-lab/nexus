package room

import (
	"context"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomActionTriggerType = "room_action"

func (s *RealtimeService) startRoomActionWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	action protocol.RoomActionRecord,
) error {
	if contextValue == nil {
		return nil
	}
	if action.WakePolicy == protocol.RoomWakePolicyNone {
		return nil
	}
	if action.WakePolicy == protocol.RoomWakePolicyDelayed {
		s.scheduleRoomActionWake(ctx, action)
		return nil
	}
	return s.runRoomActionWake(ctx, contextValue, action)
}

func (s *RealtimeService) runRoomActionWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	action protocol.RoomActionRecord,
) error {
	wakeContent, ok := roomActionWakeContent(action)
	if !ok {
		return nil
	}
	targetAgentIDs := roomActionWakeTargetAgentIDs(action)
	if len(targetAgentIDs) == 0 {
		return nil
	}
	parentRound := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(action.ConversationID),
		RoomID:         action.RoomID,
		ConversationID: action.ConversationID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        action.ActionID,
		RootRoundID:    action.ActionID,
		OwnerUserID:    authctx.OwnerUserID(ctx),
	}
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		wakes = append(wakes, publicMentionWake{
			TriggerType:   roomActionTriggerType,
			QueueSource:   protocol.InputQueueSourceAgentRoomAction,
			SourceAgentID: strings.TrimSpace(action.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       wakeContent,
			MessageID:     strings.TrimSpace(action.ActionID),
			RequestID:     strings.TrimSpace(action.RequestID),
			ReplyTarget:   action.ReplyTarget,
			ReplyAudience: append([]string(nil), action.AudienceAgentIDs...),
		})
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}

func (s *RealtimeService) scheduleRoomActionWake(ctx context.Context, action protocol.RoomActionRecord) {
	delay := time.Duration(action.DelaySeconds) * time.Second
	if delay <= 0 {
		return
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(action.ConversationID)
	s.broadcastSharedEventWithTimeout(ctx, sessionKey, action.RoomID, newRoomActionScheduledWakeEvent(action))
	ownerUserID := authctx.OwnerUserID(ctx)
	s.loggerFor(ctx).Info("Room action 延迟唤醒已计划",
		"room_id", action.RoomID,
		"conversation_id", action.ConversationID,
		"action_id", action.ActionID,
		"action_type", action.ActionType,
		"target_agent_id", action.TargetAgentID,
		"audience_agent_ids", action.AudienceAgentIDs,
		"delay_seconds", action.DelaySeconds,
	)
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C

		wakeCtx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
			UserID:     strings.TrimSpace(ownerUserID),
			Username:   strings.TrimSpace(ownerUserID),
			Role:       authctx.RoleOwner,
			AuthMethod: "room_action_delayed",
		})
		contextValue, err := s.resolveActionContext(wakeCtx, action.RoomID, action.ConversationID)
		if err != nil {
			s.loggerFor(wakeCtx).Error("Room action 延迟唤醒解析上下文失败",
				"room_id", action.RoomID,
				"conversation_id", action.ConversationID,
				"action_id", action.ActionID,
				"err", err,
			)
			return
		}
		if err = s.runRoomActionWake(wakeCtx, contextValue, action); err != nil {
			s.loggerFor(wakeCtx).Error("执行 Room action 延迟唤醒失败",
				"room_id", action.RoomID,
				"conversation_id", action.ConversationID,
				"action_id", action.ActionID,
				"action_type", action.ActionType,
				"target_agent_id", action.TargetAgentID,
				"audience_agent_ids", action.AudienceAgentIDs,
				"err", err,
			)
		}
	}()
}

func roomActionWakeContent(action protocol.RoomActionRecord) (string, bool) {
	switch action.ActionType {
	case protocol.RoomActionTypePrivateMessage:
		if action.WakePolicy == protocol.RoomWakePolicyNone {
			return "", false
		}
		return "收到一条 Room private_message；请读取 <room_actions> 中投影给你的内容。", true
	case protocol.RoomActionTypeRequestReply:
		if action.WakePolicy != protocol.RoomWakePolicyImmediate && action.WakePolicy != protocol.RoomWakePolicyDelayed {
			return "", false
		}
		return "收到一条 Room request_reply；请读取 <room_actions> 中投影给你的请求并按 reply_target 回复。", true
	default:
		return "", false
	}
}

func roomActionWakeTargetAgentIDs(action protocol.RoomActionRecord) []string {
	targetAgentID := strings.TrimSpace(action.TargetAgentID)
	if targetAgentID != "" {
		return []string{targetAgentID}
	}
	if action.ActionType != protocol.RoomActionTypePrivateMessage {
		return nil
	}
	result := make([]string, 0, len(action.AudienceAgentIDs))
	for _, agentID := range action.AudienceAgentIDs {
		normalized := strings.TrimSpace(agentID)
		if normalized == "" || containsRoomActionWakeTarget(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func containsRoomActionWakeTarget(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}
