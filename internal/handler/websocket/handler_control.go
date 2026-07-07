package websocket

import (
	"context"
	"errors"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
)

// sendChatFailure 回报 chat 类请求受理失败。此时后端还没有 canonical round_id，
// 前端只按 client_request_id / client_message_id 关联并清理 optimistic 状态。
func (h *Handler) sendChatFailure(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	sessionKey string,
	msgType string,
	clientRequestID string,
	clientMessageID string,
	err error,
) {
	errorType := "chat_error"
	if errors.Is(err, dmsvc.ErrRoomSessionNotImplemented) {
		errorType = "not_implemented"
	}
	details := map[string]any{"type": msgType}
	if clientRequestID != "" {
		details["client_request_id"] = clientRequestID
	}
	if clientMessageID != "" {
		details["client_message_id"] = clientMessageID
	}
	h.sendGatewayError(ctx, sender, sessionKey, errorType, err, details)
}

func (h *Handler) handleControlMessage(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, parsed, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if h.ensureSessionBinding(ctx, sender, inbound, sessionKey) {
		return
	}

	msgType := handlershared.StringValue(inbound["type"])
	switch msgType {
	case "chat":
		var err error
		clientRequestID := handlershared.StringValue(inbound["client_request_id"])
		clientMessageID := handlershared.StringValue(inbound["client_message_id"])
		if parsed.Kind == protocol.SessionKeyKindRoom && h.roomRealtime != nil {
			err = h.roomRealtime.HandleChat(ctx, roompkg.ChatRequest{
				SessionKey:        sessionKey,
				RoomID:            handlershared.StringValue(inbound["room_id"]),
				ConversationID:    handlershared.StringValue(inbound["conversation_id"]),
				AttachmentAgentID: handlershared.StringValue(inbound["agent_id"]),
				Content:           handlershared.StringValue(inbound["content"]),
				Attachments:       protocol.ChatAttachmentsFromAny(inbound["attachments"]),
				ClientRequestID:   clientRequestID,
				ClientMessageID:   clientMessageID,
				DeliveryPolicy:    protocol.NormalizeChatDeliveryPolicy(handlershared.StringValue(inbound["delivery_policy"])),
			})
		} else {
			err = h.dm.HandleChat(ctx, dmsvc.Request{
				SessionKey:      sessionKey,
				AgentID:         handlershared.StringValue(inbound["agent_id"]),
				Content:         handlershared.StringValue(inbound["content"]),
				Attachments:     protocol.ChatAttachmentsFromAny(inbound["attachments"]),
				ClientRequestID: clientRequestID,
				ClientMessageID: clientMessageID,
				DeliveryPolicy:  protocol.NormalizeChatDeliveryPolicy(handlershared.StringValue(inbound["delivery_policy"])),
			})
		}
		if err != nil {
			h.sendChatFailure(ctx, sender, sessionKey, msgType, clientRequestID, clientMessageID, err)
		}
	case "chat_rewrite_last":
		var err error
		clientRequestID := handlershared.StringValue(inbound["client_request_id"])
		clientMessageID := handlershared.StringValue(inbound["client_message_id"])
		if parsed.Kind == protocol.SessionKeyKindRoom {
			err = dmsvc.ErrRoomSessionNotImplemented
		} else {
			err = h.dm.HandleRewriteLastUserMessage(ctx, dmsvc.RewriteRequest{
				SessionKey:      sessionKey,
				AgentID:         handlershared.StringValue(inbound["agent_id"]),
				TargetRoundID:   handlershared.StringValue(inbound["target_round_id"]),
				ClientRequestID: clientRequestID,
				ClientMessageID: clientMessageID,
				Content:         handlershared.StringValue(inbound["content"]),
				Attachments:     protocol.ChatAttachmentsFromAny(inbound["attachments"]),
			})
		}
		if err != nil {
			h.sendChatFailure(ctx, sender, sessionKey, msgType, clientRequestID, clientMessageID, err)
		}
	case "interrupt":
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && h.roomRealtime != nil {
			err = h.roomRealtime.HandleInterrupt(ctx, roompkg.InterruptRequest{
				SessionKey:   sessionKey,
				RoundID:      handlershared.StringValue(inbound["round_id"]),
				AgentRoundID: handlershared.StringValue(inbound["agent_round_id"]),
			})
		} else {
			err = h.dm.HandleInterrupt(ctx, dmsvc.InterruptRequest{
				SessionKey: sessionKey,
				RoundID:    handlershared.StringValue(inbound["round_id"]),
			})
		}
		if err != nil {
			h.sendGatewayError(ctx, sender, sessionKey, "interrupt_error", err, map[string]any{"type": msgType})
		}
	case "input_queue":
		action := firstStringValue(inbound["action"], inbound["action_type"])
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && h.roomRealtime != nil {
			err = h.roomRealtime.HandleInputQueue(ctx, roompkg.InputQueueRequest{
				SessionKey:     sessionKey,
				RoomID:         handlershared.StringValue(inbound["room_id"]),
				ConversationID: handlershared.StringValue(inbound["conversation_id"]),
				Action:         action,
				ItemID:         handlershared.StringValue(inbound["item_id"]),
				Content:        handlershared.StringValue(inbound["content"]),
				Attachments:    protocol.ChatAttachmentsFromAny(inbound["attachments"]),
				OrderedIDs:     stringSliceValue(inbound["ordered_ids"]),
				DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(handlershared.StringValue(inbound["delivery_policy"])),
			})
		} else {
			err = h.dm.HandleInputQueue(ctx, dmsvc.InputQueueRequest{
				SessionKey:     sessionKey,
				AgentID:        handlershared.StringValue(inbound["agent_id"]),
				Action:         action,
				ItemID:         handlershared.StringValue(inbound["item_id"]),
				Content:        handlershared.StringValue(inbound["content"]),
				Attachments:    protocol.ChatAttachmentsFromAny(inbound["attachments"]),
				OrderedIDs:     stringSliceValue(inbound["ordered_ids"]),
				DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(handlershared.StringValue(inbound["delivery_policy"])),
			})
		}
		if err != nil {
			h.sendGatewayError(ctx, sender, sessionKey, "input_queue_error", err, map[string]any{
				"type":   msgType,
				"action": action,
			})
		}
	case "permission_response":
		if !h.permission.HandlePermissionResponse(inbound) {
			_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(
				sessionKey,
				"permission_request_not_found",
				"未找到待确认的权限请求",
				map[string]any{"type": msgType},
			))
		}
	default:
		_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(
			sessionKey,
			"not_implemented",
			"Go 运行时已接管控制面，但该写操作尚未实现",
			map[string]any{"type": msgType},
		))
	}
}
