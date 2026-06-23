package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
)

func (s *RealtimeService) scheduleTitleGeneration(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	content string,
	provider string,
	model string,
) {
	if s.titles == nil || contextValue == nil {
		return
	}
	s.titles.Schedule(ctx, titlegen.Request{
		OwnerUserID:              authctx.OwnerUserID(ctx),
		SessionKey:               sessionKey,
		Provider:                 strings.TrimSpace(provider),
		Model:                    strings.TrimSpace(model),
		Content:                  content,
		SessionMessageCount:      -1,
		ConversationID:           contextValue.Conversation.ID,
		ConversationRoomID:       contextValue.Room.ID,
		ConversationTitle:        contextValue.Conversation.Title,
		ConversationRoomName:     contextValue.Room.Name,
		ConversationMessageCount: contextValue.Conversation.MessageCount,
	})
}

func resolveTitleRuntimeTarget(
	targetAgentIDs []string,
	agentByID map[string]*protocol.Agent,
) (string, string) {
	for _, agentID := range targetAgentIDs {
		agentValue := agentByID[strings.TrimSpace(agentID)]
		if agentValue == nil {
			continue
		}
		return strings.TrimSpace(agentValue.Options.Provider), strings.TrimSpace(agentValue.Options.Model)
	}
	return "", ""
}
