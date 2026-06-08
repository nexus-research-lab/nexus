package tool

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/room/contract"
)

func scopedToolContext(ctx context.Context, sctx contract.ServerContext) context.Context {
	ownerUserID := strings.TrimSpace(sctx.OwnerUserID)
	if ownerUserID == "" {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: "room_mcp_runtime",
	})
}

func requireRoomScope(sctx contract.ServerContext) (string, string, string, error) {
	if strings.TrimSpace(sctx.SourceContextType) != "room" {
		return "", "", "", errors.New("nexus_room tools are only available inside Room runtime")
	}
	agentID := strings.TrimSpace(sctx.CurrentAgentID)
	if agentID == "" {
		return "", "", "", errors.New("missing current Room agent")
	}
	roomID := strings.TrimSpace(sctx.RoomID)
	if roomID == "" {
		return "", "", "", errors.New("missing current Room id")
	}
	conversationID := strings.TrimSpace(sctx.ConversationID)
	if conversationID == "" {
		return "", "", "", errors.New("missing current Room conversation")
	}
	return agentID, roomID, conversationID, nil
}
