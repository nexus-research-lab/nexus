package websocket

import (
	"context"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (h *Handler) dispatchWebSocketMessage(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	msgType := handlershared.StringValue(inbound["type"])
	if _, ok := inbound["method"]; ok {
		h.handleAppServerRPC(ctx, sender, inbound)
		return
	}
	switch msgType {
	case "ping":
		_ = sender.SendEvent(ctx, protocol.NewPongEvent(handlershared.StringValue(inbound["session_key"])))
	case "subscribe_workspace":
		h.handleSubscribeWorkspace(ctx, sender, inbound)
	case "unsubscribe_workspace":
		h.handleUnsubscribeWorkspace(sender, inbound)
	case "subscribe_app_events":
		h.handleSubscribeAppEvents(sender)
	case "unsubscribe_app_events":
		h.handleUnsubscribeAppEvents(sender)
	case "subscribe_room":
		h.handleSubscribeRoom(ctx, sender, inbound)
	case "unsubscribe_room":
		h.handleUnsubscribeRoom(sender, inbound)
	case "bind_session":
		h.handleBindSession(ctx, sender, inbound)
	case "unbind_session":
		h.handleUnbindSession(ctx, sender, inbound)
	case "chat", "chat_rewrite_last", "interrupt", "permission_response", "input_queue":
		h.handleControlMessage(ctx, sender, inbound)
	default:
		_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(
			handlershared.StringValue(inbound["session_key"]),
			"unknown_message_type",
			"Go HTTP 服务已接管入口，但该消息类型尚未实现",
			map[string]any{"type": msgType},
		))
	}
}

func (h *Handler) handleSubscribeAppEvents(sender *handlershared.WebSocketSender) {
	if h.appEventSubs == nil {
		return
	}
	h.appEventSubs.Subscribe(sender)
}

func (h *Handler) handleUnsubscribeAppEvents(sender *handlershared.WebSocketSender) {
	if h.appEventSubs == nil {
		return
	}
	h.appEventSubs.Unsubscribe(sender)
}
