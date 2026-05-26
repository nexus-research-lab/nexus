package room

import (
	"context"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func buildRoomGuidanceMessage(
	sessionKey string,
	roomID string,
	conversationID string,
	slot *activeRoomSlot,
	sourceRoundID string,
	content string,
) protocol.Message {
	if slot == nil {
		return protocol.Message{}
	}
	return roomdomain.BuildGuidanceMessage(roomdomain.GuidanceMessageInput{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		AgentID:        slot.AgentID,
		AgentRoundID:   slot.AgentRoundID,
		SourceRoundID:  sourceRoundID,
		Content:        content,
		SDKSessionID:   slot.getSDKSessionID(),
	})
}

func (s *RealtimeService) broadcastSlotGuidanceMessage(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ string,
	_ protocol.Message,
) {
	// 引导消息只进入运行中 slot 的执行链路，不能作为公区输出事件展示。
}
