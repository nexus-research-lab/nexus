// [INPUT]: 依赖 Room 订阅请求、权限校验、实时服务的活跃 slot 状态与当前 durable Room 序号。
// [OUTPUT]: 先发送可为空/多 root 的权威 pending slot 快照，再从客户端游标或快照前捕获的序号边界建立订阅并补放期间的 durable server pending 事件。
// [POS]: websocket handler 的 Room 权威快照与无缝事件恢复入口。
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
	h.restoreRoomPendingSlots(ctx, sender, roomID, conversationID)
	if h.roomSubs != nil {
		lastSeenRoomSeq := handlershared.Int64Value(inbound["last_seen_room_seq"])
		replayBoundary := latestRoomSeq
		if lastSeenRoomSeq > 0 {
			replayBoundary = lastSeenRoomSeq
		}
		if err := h.roomSubs.SubscribeRoom(
			ctx,
			sender,
			roomID,
			conversationID,
			&replayBoundary,
		); err != nil {
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
) {
	if h.roomRealtime == nil || strings.TrimSpace(conversationID) == "" {
		return
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

	// 订阅恢复值是后端权威快照；即使为空也要发送，多 root 则由每个 slot
	// 自己的 round_id 定位，以清除或重建浏览器中的运行占位。
	event := protocol.NewChatPendingSnapshotEvent(sessionKey, roundID, pending)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.RoundID = roundID
	_ = sender.SendEvent(ctx, event)
}
