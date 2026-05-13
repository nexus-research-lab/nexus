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
	targetAgentID := strings.TrimSpace(action.TargetAgentID)
	if targetAgentID == "" {
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
	return s.startPublicMentionRound(ctx, parentRound, []publicMentionWake{
		{
			TriggerType:   roomActionTriggerType,
			QueueSource:   protocol.InputQueueSourceAgentRoomAction,
			SourceAgentID: strings.TrimSpace(action.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       wakeContent,
			MessageID:     strings.TrimSpace(action.ActionID),
			RequestID:     strings.TrimSpace(action.RequestID),
			ReplyTarget:   action.ReplyTarget,
			ReplyAudience: append([]string(nil), action.AudienceAgentIDs...),
		},
	})
}

func roomActionWakeContent(action protocol.RoomActionRecord) (string, bool) {
	switch action.ActionType {
	case protocol.RoomActionTypePrivateMessage:
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
