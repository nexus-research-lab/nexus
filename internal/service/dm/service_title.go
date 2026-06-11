package dm

import (
	"context"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
)

func (s *Service) scheduleTitleGeneration(
	ctx context.Context,
	parsed protocol.SessionKey,
	sessionItem protocol.Session,
	content string,
	initialMessageCount int,
	provider string,
	model string,
) {
	if s.titles == nil {
		return
	}
	roomID := ""
	conversationID := ""
	if !isExternalIMSession(parsed, sessionItem) {
		roomID = strings.TrimSpace(dmdomain.StringPointerValue(sessionItem.RoomID))
		if roomID != "" {
			conversationID = strings.TrimSpace(dmdomain.StringPointerValue(sessionItem.ConversationID))
		}
	}
	conversationMessageCount := 0
	if conversationID == "" {
		conversationMessageCount = -1
	}
	s.titles.Schedule(ctx, titlegen.Request{
		OwnerUserID:              authctx.OwnerUserID(ctx),
		SessionKey:               sessionItem.SessionKey,
		Provider:                 strings.TrimSpace(provider),
		Model:                    strings.TrimSpace(model),
		Content:                  content,
		SessionTitle:             sessionItem.Title,
		SessionMessageCount:      initialMessageCount,
		ConversationID:           conversationID,
		ConversationRoomID:       roomID,
		ConversationMessageCount: conversationMessageCount,
	})
}

func runtimeSelectionFromSession(sessionItem protocol.Session) (string, string) {
	if sessionItem.Options == nil {
		return "", ""
	}
	provider, _ := sessionItem.Options[protocol.OptionRuntimeProvider].(string)
	model, _ := sessionItem.Options[protocol.OptionRuntimeModel].(string)
	return strings.TrimSpace(provider), strings.TrimSpace(model)
}

func isExternalIMSession(parsed protocol.SessionKey, sessionItem protocol.Session) bool {
	channel := protocol.NormalizeStoredChannelType(parsed.Channel)
	if channel == "" {
		channel = protocol.NormalizeStoredChannelType(sessionItem.ChannelType)
	}
	switch channel {
	case "", protocol.SessionChannelWebSocket, protocol.SessionChannelInternalSegment:
		return false
	default:
		return true
	}
}
