package websocket

import (
	"context"
	"errors"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roompkg "github.com/nexus-research-lab/nexus/internal/service/room"
)

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
		if parsed.Kind == protocol.SessionKeyKindRoom && h.roomRealtime != nil {
			err = h.roomRealtime.HandleChat(ctx, roompkg.ChatRequest{
				SessionKey:        sessionKey,
				RoomID:            handlershared.StringValue(inbound["room_id"]),
				ConversationID:    handlershared.StringValue(inbound["conversation_id"]),
				AttachmentAgentID: handlershared.StringValue(inbound["agent_id"]),
				Content:           handlershared.StringValue(inbound["content"]),
				Attachments:       protocol.ChatAttachmentsFromAny(inbound["attachments"]),
				RoundID:           handlershared.StringValue(inbound["round_id"]),
				ReqID:             handlershared.StringValue(inbound["req_id"]),
				DeliveryPolicy:    protocol.NormalizeChatDeliveryPolicy(handlershared.StringValue(inbound["delivery_policy"])),
			})
		} else {
			err = h.dm.HandleChat(ctx, dmsvc.Request{
				SessionKey:     sessionKey,
				AgentID:        handlershared.StringValue(inbound["agent_id"]),
				Content:        handlershared.StringValue(inbound["content"]),
				Attachments:    protocol.ChatAttachmentsFromAny(inbound["attachments"]),
				RoundID:        handlershared.StringValue(inbound["round_id"]),
				ReqID:          handlershared.StringValue(inbound["req_id"]),
				DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(handlershared.StringValue(inbound["delivery_policy"])),
			})
		}
		if err != nil {
			errorType := "chat_error"
			if errors.Is(err, dmsvc.ErrRoomSessionNotImplemented) {
				errorType = "not_implemented"
			}
			roundID := handlershared.StringValue(inbound["round_id"])
			details := map[string]any{"type": msgType}
			if roundID != "" {
				details["round_id"] = roundID
			}
			if roundID != "" {
				reqID := firstStringValue(inbound["req_id"], roundID)
				_ = sender.SendEvent(ctx, protocol.NewChatAckEvent(sessionKey, reqID, roundID, []map[string]any{}))
			}
			h.sendGatewayError(ctx, sender, sessionKey, errorType, err, details)
			if roundID != "" {
				_ = sender.SendEvent(ctx, protocol.NewRoundStatusEvent(sessionKey, roundID, "error", "error"))
			}
		}
	case "interrupt":
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && h.roomRealtime != nil {
			err = h.roomRealtime.HandleInterrupt(ctx, roompkg.InterruptRequest{
				SessionKey: sessionKey,
				MsgID:      handlershared.StringValue(inbound["msg_id"]),
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
