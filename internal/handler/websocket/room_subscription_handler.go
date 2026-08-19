// [INPUT]: 依赖 Room 订阅请求、权限校验、活跃 slot、待确认人工交互与 durable Room 序号。
// [OUTPUT]: 先发送权威执行/交互快照，再从游标或快照前序号边界建立订阅并补放期间的 durable 事件。
// [POS]: websocket handler 的 Room 权威快照与无缝事件恢复入口。
package websocket

import (
	"context"
	"errors"
	"slices"
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
	h.restoreRoomActivitySnapshot(ctx, sender, roomID, conversationID)
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

func (h *Handler) restoreRoomActivitySnapshot(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	roomID string,
	conversationID string,
) {
	pendingInteractionRequestIDs := []string{}
	if h.permission != nil {
		pendingInteractionRequestIDs = h.permission.PendingRequestIDsForRoom(roomID, conversationID)
	}
	if h.automationPermissions != nil {
		requestIDs, err := h.automationPermissions.PendingPermissionRequestIDsForRoom(
			ctx,
			roomID,
			conversationID,
		)
		if err == nil {
			pendingInteractionRequestIDs = appendUniqueStrings(
				pendingInteractionRequestIDs,
				requestIDs...,
			)
		}
	}
	if strings.TrimSpace(conversationID) == "" {
		event := protocol.NewChatContainerActivitySnapshotEvent(
			pendingInteractionRequestIDs,
			h.activeChatActivitySources(roomID),
		)
		event.RoomID = roomID
		_ = sender.SendEvent(ctx, event)
		return
	}
	if h.roomRealtime == nil {
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
	event := protocol.NewChatPendingSnapshotEvent(
		sessionKey,
		roundID,
		pending,
		pendingInteractionRequestIDs,
	)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.RoundID = roundID
	_ = sender.SendEvent(ctx, event)
}

func (h *Handler) activeChatActivitySources(roomID string) []protocol.ChatActivitySourceSnapshot {
	if h.permission == nil || h.runtime == nil || strings.TrimSpace(roomID) == "" {
		return []protocol.ChatActivitySourceSnapshot{}
	}
	type sourceState struct {
		conversationID string
		roundIDs       map[string]struct{}
	}
	bySessionKey := make(map[string]*sourceState)
	for _, routeSnapshot := range h.permission.SessionActivityRoutesForRoom(roomID) {
		runningRoundIDs := h.runtime.GetRunningRoundIDs(routeSnapshot.SessionKey)
		if len(runningRoundIDs) == 0 {
			continue
		}
		sourceSessionKey := strings.TrimSpace(routeSnapshot.Route.DispatchSessionKey)
		if sourceSessionKey == "" {
			sourceSessionKey = strings.TrimSpace(routeSnapshot.SessionKey)
		}
		conversationID := strings.TrimSpace(routeSnapshot.Route.ConversationID)
		if sourceSessionKey == "" || conversationID == "" {
			continue
		}
		state := bySessionKey[sourceSessionKey]
		if state == nil {
			state = &sourceState{
				conversationID: conversationID,
				roundIDs:       make(map[string]struct{}),
			}
			bySessionKey[sourceSessionKey] = state
		}
		for _, roundID := range runningRoundIDs {
			if roundID = strings.TrimSpace(roundID); roundID != "" {
				state.roundIDs[roundID] = struct{}{}
			}
		}
	}
	sessionKeys := make([]string, 0, len(bySessionKey))
	for sessionKey := range bySessionKey {
		sessionKeys = append(sessionKeys, sessionKey)
	}
	slices.Sort(sessionKeys)
	result := make([]protocol.ChatActivitySourceSnapshot, 0, len(sessionKeys))
	for _, sessionKey := range sessionKeys {
		state := bySessionKey[sessionKey]
		roundIDs := make([]string, 0, len(state.roundIDs))
		for roundID := range state.roundIDs {
			roundIDs = append(roundIDs, roundID)
		}
		slices.Sort(roundIDs)
		if len(roundIDs) == 0 {
			continue
		}
		result = append(result, protocol.ChatActivitySourceSnapshot{
			SessionKey:      sessionKey,
			ConversationID:  state.conversationID,
			RunningRoundIDs: roundIDs,
		})
	}
	return result
}

func appendUniqueStrings(values []string, candidates ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(candidates))
	result := make([]string, 0, len(values)+len(candidates))
	for _, value := range append(values, candidates...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
