package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomActionTriggerType = "room_action"
const roomPrivateMessageWakeContent = "收到一条 Room private_message；请读取 <room_actions> 中投影给你的内容。"

func (s *RealtimeService) startRoomActionWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	action protocol.RoomActionRecord,
) error {
	if contextValue == nil || action.ActionType != protocol.RoomActionTypePrivateMessage {
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
			Content:       roomPrivateMessageWakeContent,
			MessageID:     strings.TrimSpace(action.ActionID),
			ReplyTarget:   action.ReplyTarget,
			ReplyAudience: append([]string(nil), action.AudienceAgentIDs...),
		},
	})
}
