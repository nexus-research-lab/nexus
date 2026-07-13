package websocket

import (
	"context"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (h *Handler) handleBindSession(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, parsed, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if parsed.Kind == protocol.SessionKeyKindUnknown {
		return
	}
	h.permission.BindSession(sessionKey, sender)
	if h.channels != nil {
		_ = h.channels.RememberWebSocketRoute(ctx, sessionKey)
	}
	h.broadcastSessionStatus(ctx, sessionKey)
	if parsed.Kind == protocol.SessionKeyKindAgent && h.dm != nil {
		if err := h.dm.SendInputQueueSnapshot(ctx, sessionKey, handlershared.StringValue(inbound["agent_id"])); err != nil {
			h.sendGatewayError(ctx, sender, sessionKey, "input_queue_error", err, map[string]any{"type": "bind_session"})
		}
	}
}

func (h *Handler) handleUnbindSession(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, _, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	h.permission.UnbindSession(sessionKey, sender)
	h.broadcastSessionStatus(ctx, sessionKey)
}

func (h *Handler) ensureSessionBinding(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	sessionKey string,
) {
	if h.permission.IsBound(sessionKey, sender) {
		return
	}
	h.permission.BindSession(sessionKey, sender)
	h.broadcastSessionStatus(ctx, sessionKey)
}

func (h *Handler) validateSessionKey(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) (string, protocol.SessionKey, bool) {
	sessionKey := handlershared.StringValue(inbound["session_key"])
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		errorType := "invalid_session_key"
		if err.Error() == "session_key is required" {
			errorType = "validation_error"
		}
		h.sendGatewayError(ctx, sender, sessionKey, errorType, err, map[string]any{"type": handlershared.StringValue(inbound["type"])})
		return "", protocol.SessionKey{}, false
	}
	return normalized, protocol.ParseSessionKey(normalized), true
}

func (h *Handler) broadcastSessionStatus(ctx context.Context, sessionKeys ...string) {
	for _, sessionKey := range sessionKeys {
		if strings.TrimSpace(sessionKey) == "" {
			continue
		}
		_ = h.permission.BroadcastSessionStatus(ctx, sessionKey, h.runtime.GetRunningRoundIDs(sessionKey))
	}
}
