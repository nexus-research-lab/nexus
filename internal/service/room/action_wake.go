package room

import (
	"context"
	"strings"

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

func roomActionWakeContent(action protocol.RoomActionRecord) (string, bool) {
	switch action.ActionType {
	case protocol.RoomActionTypePrivateMessage:
		if action.WakePolicy == protocol.RoomWakePolicyNone {
			return "", false
		}
		return "收到一条 Room private_message；请读取 <room_actions> 中投影给你的内容。", true
	case protocol.RoomActionTypeRequestReply:
		if action.WakePolicy != protocol.RoomWakePolicyImmediate {
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
