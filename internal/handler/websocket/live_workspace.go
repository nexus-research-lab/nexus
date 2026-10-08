package websocket

import (
	"context"
	"errors"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/textutil"
)

func (h *Handler) handleSubscribeWorkspace(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	agentID := strings.TrimSpace(textutil.AnyString(inbound["agent_id"]))
	if agentID == "" {
		h.sendGatewayError(
			ctx,
			sender,
			"",
			"invalid_workspace_subscription",
			errors.New("agent_id is required"),
			map[string]any{
				"type": "subscribe_workspace",
			},
		)
		return
	}
	if h.workspaceSubs == nil {
		return
	}
	if err := h.workspaceSubs.Subscribe(ctx, sender, agentID); err != nil {
		h.sendGatewayError(ctx, sender, "", "workspace_subscription_error", err, map[string]any{
			"type":     "subscribe_workspace",
			"agent_id": agentID,
		})
	}
}

func (h *Handler) handleUnsubscribeWorkspace(
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	if h.workspaceSubs == nil {
		return
	}
	agentID := strings.TrimSpace(textutil.AnyString(inbound["agent_id"]))
	if agentID == "" {
		return
	}
	h.workspaceSubs.Unsubscribe(sender, agentID)
}
