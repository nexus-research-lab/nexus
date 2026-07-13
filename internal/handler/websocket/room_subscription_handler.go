// [INPUT]: 依赖 Room 订阅请求、权限校验与实时服务的活跃 slot 状态。
// [OUTPUT]: 对新订阅者发送权威 pending slot 快照并建立 Room 事件订阅。
// [POS]: websocket handler 的 Room 订阅恢复入口。
package websocket

import (
	"context"
	"errors"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (h *Handler) handleSubscribeRoom(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
) {
	roomID := handlershared.StringValue(inbound["room_id"])
	conversationID := handlershared.StringValue(inbound["conversation_id"])
	if err := h.validateRoomSubscription(ctx, roomID, conversationID); err != nil {
		h.sendGatewayError(ctx, sender, "", "invalid_room_subscription", err, map[string]any{
			"type":            handlershared.StringValue(inbound["type"]),
			"room_id":         roomID,
			"conversation_id": conversationID,
		})
		return
	}
	var latestRoomSeq int64
	if h.roomSubs != nil {
		latestRoomSeq = h.roomSubs.CurrentRoomSeq(roomID)
	}
	hasPending := h.restoreRoomPendingSlots(ctx, sender, roomID, conversationID)
	if h.roomSubs != nil {
		lastSeenRoomSeq := handlershared.Int64Value(inbound["last_seen_room_seq"])
		var lastSeenPtr *int64
		if lastSeenRoomSeq > 0 {
			lastSeenPtr = &lastSeenRoomSeq
		} else if hasPending && latestRoomSeq > 0 {
			lastSeenPtr = &latestRoomSeq
		}
		if err := h.roomSubs.SubscribeRoom(ctx, sender, roomID, conversationID, lastSeenPtr); err != nil {
			h.sendGatewayError(ctx, sender, "", "room_subscription_error", err, map[string]any{
				"type":            handlershared.StringValue(inbound["type"]),
				"room_id":         roomID,
				"conversation_id": conversationID,
			})
			return
		}
	}
	if h.roomRealtime != nil && strings.TrimSpace(conversationID) != "" {
		event, err := h.roomRealtime.InputQueueSnapshotEvent(ctx, roomID, conversationID)
		if err != nil {
			h.sendGatewayError(ctx, sender, "", "input_queue_error", err, map[string]any{
				"type":            "subscribe_room",
				"room_id":         roomID,
				"conversation_id": conversationID,
			})
			return
		}
		_ = sender.SendEvent(ctx, event)
	}
}

func (h *Handler) handleUnsubscribeRoom(sender *handlershared.WebSocketSender, inbound map[string]any) {
	if h.roomSubs == nil {
		return
	}
	h.roomSubs.UnsubscribeRoom(
		sender,
		handlershared.StringValue(inbound["room_id"]),
		handlershared.StringValue(inbound["conversation_id"]),
	)
}

func (h *Handler) validateRoomSubscription(ctx context.Context, roomID string, conversationID string) error {
	if strings.TrimSpace(roomID) == "" {
		return errors.New("room_id is required")
	}
	if strings.TrimSpace(conversationID) == "" {
		_, err := h.roomService.GetRoom(ctx, roomID)
		return err
	}

	contextValue, err := h.roomService.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	if contextValue.Room.ID != roomID {
		return errors.New("conversation_id does not belong to room_id")
	}
	return nil
}

func (h *Handler) restoreRoomPendingSlots(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	roomID string,
	conversationID string,
) bool {
	if h.roomRealtime == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}

	snapshot := h.roomRealtime.GetActiveRoundSnapshot(conversationID)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	roundID := ""
	pending := []protocol.ChatAckPendingSlot{}
	if snapshot != nil {
		sessionKey = snapshot.SessionKey
		roundID = snapshot.RoundID
		pending = snapshot.Pending
	}

	// 订阅恢复值是后端权威快照；即使为空也要发送，清除浏览器残留的运行占位。
	event := protocol.NewChatPendingSnapshotEvent(sessionKey, roundID, pending)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.RoundID = roundID
	_ = sender.SendEvent(ctx, event)
	return len(pending) > 0
}
