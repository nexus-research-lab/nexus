package dm

import (
	"context"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
)

func (s *Service) scheduleTitleGeneration(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionItem protocol.Session,
	content string,
	initialMessageCount int,
) {
	if s.titles == nil {
		return
	}
	conversationID := strings.TrimSpace(dmdomain.StringPointerValue(sessionItem.ConversationID))
	if conversationID == "" && parsed.ChatType == "dm" {
		conversationID = strings.TrimSpace(parsed.Ref)
	}
	roomID := strings.TrimSpace(dmdomain.StringPointerValue(sessionItem.RoomID))
	conversationMessageCount := 0
	if conversationID == "" {
		conversationMessageCount = -1
	}
	s.titles.Schedule(ctx, titlegen.Request{
		SessionKey:               sessionItem.SessionKey,
		Provider:                 "",
		Content:                  content,
		SessionTitle:             sessionItem.Title,
		SessionMessageCount:      initialMessageCount,
		ConversationID:           conversationID,
		ConversationRoomID:       roomID,
		ConversationMessageCount: conversationMessageCount,
	})
}
