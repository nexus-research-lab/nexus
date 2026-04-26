package websocket

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	channelspkg "github.com/nexus-research-lab/nexus/internal/channels"
	chatsvc "github.com/nexus-research-lab/nexus/internal/chat"
	gatewayshared "github.com/nexus-research-lab/nexus/internal/gateway/shared"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roompkg "github.com/nexus-research-lab/nexus/internal/room"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	websocketReadLimit   = 4 << 20
	websocketReadTimeout = 90 * time.Second
	websocketPingEvery   = 30 * time.Second
)

// Handler 封装 WebSocket 生命周期与控制消息分发。
type Handler struct {
	api           *gatewayshared.API
	roomService   *roompkg.Service
	roomRealtime  *roompkg.RealtimeService
	chat          *chatsvc.Service
	permission    *permissionctx.Context
	runtime       *runtimectx.Manager
	channels      *channelspkg.Router
	roomSubs      *roomSubscriptionRegistry
	workspaceSubs *workspaceSubscriptionRegistry
}

// NewHandler 创建 WebSocket handler。
func NewHandler(
	api *gatewayshared.API,
	roomService *roompkg.Service,
	roomRealtime *roompkg.RealtimeService,
	chat *chatsvc.Service,
	permission *permissionctx.Context,
	runtime *runtimectx.Manager,
	channels *channelspkg.Router,
	workspaceService *workspacepkg.Service,
	runtimeProvider func(string) RuntimeSnapshot,
) *Handler {
	handler := &Handler{
		api:           api,
		roomService:   roomService,
		roomRealtime:  roomRealtime,
		chat:          chat,
		permission:    permission,
		runtime:       runtime,
		channels:      channels,
		roomSubs:      newRoomSubscriptionRegistry(128),
		workspaceSubs: newWorkspaceSubscriptionRegistry(workspaceService, runtimeProvider),
	}
	if roomRealtime != nil {
		roomRealtime.SetRoomBroadcaster(handler.roomSubs)
	}
	return handler
}

// HandleWebSocket 处理 WebSocket 会话。
func (h *Handler) HandleWebSocket(writer http.ResponseWriter, request *http.Request) {
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	connection.SetReadLimit(websocketReadLimit)
	sender := gatewayshared.NewWebSocketSender(connection)
	defer func() {
		sender.MarkClosed()
		if h.workspaceSubs != nil {
			h.workspaceSubs.UnregisterSender(sender)
		}
		if h.roomSubs != nil {
			h.roomSubs.UnregisterSender(sender)
		}
		_ = connection.Close(websocket.StatusNormalClosure, "closed")
		h.broadcastSessionStatus(request.Context(), h.permission.UnregisterSender(sender)...)
	}()

	ctx := request.Context()
	go h.keepWebSocketAlive(ctx, connection, sender)
	for {
		var inbound map[string]any
		readCtx, cancel := context.WithTimeout(ctx, websocketReadTimeout)
		err := wsjson.Read(readCtx, connection, &inbound)
		cancel()
		if err != nil {
			return
		}
		h.dispatchWebSocketMessage(ctx, sender, inbound)
	}
}

// BroadcastRoomEvent 广播共享 room 事件。
func (h *Handler) BroadcastRoomEvent(
	ctx context.Context,
	roomID string,
	eventType protocol.EventType,
	data map[string]any,
) {
	if h.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	event := protocol.NewEvent(eventType, data)
	event.RoomID = strings.TrimSpace(roomID)
	h.roomSubs.Broadcast(ctx, event.RoomID, event)
}

// BroadcastRoomResyncRequired 广播 room resync 通知。
func (h *Handler) BroadcastRoomResyncRequired(
	ctx context.Context,
	roomID string,
	conversationID string,
	reason string,
) {
	if h.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	data := map[string]any{
		"room_id":         strings.TrimSpace(roomID),
		"conversation_id": strings.TrimSpace(conversationID),
		"reason":          strings.TrimSpace(reason),
	}
	event := protocol.NewEvent(protocol.EventTypeRoomResyncRequired, data)
	event.RoomID = data["room_id"].(string)
	h.roomSubs.Broadcast(ctx, event.RoomID, event)
}

// RemoveRoom 从 room 广播注册表中移除目标 room。
func (h *Handler) RemoveRoom(roomID string) {
	if h.roomSubs == nil {
		return
	}
	h.roomSubs.RemoveRoom(roomID)
}

func (h *Handler) keepWebSocketAlive(
	ctx context.Context,
	connection *websocket.Conn,
	sender *gatewayshared.WebSocketSender,
) {
	ticker := time.NewTicker(websocketPingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if sender.IsClosed() {
				return
			}
			pingCtx, cancel := context.WithTimeout(ctx, gatewayshared.WebSocketWriteTimeout)
			err := connection.Ping(pingCtx)
			cancel()
			if err != nil {
				sender.MarkClosed()
				_ = connection.Close(websocket.StatusPolicyViolation, "ping timeout")
				return
			}
		}
	}
}

func (h *Handler) dispatchWebSocketMessage(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
) {
	msgType := gatewayshared.StringValue(inbound["type"])
	switch msgType {
	case "ping":
		_ = sender.SendEvent(ctx, protocol.NewPongEvent(gatewayshared.StringValue(inbound["session_key"])))
	case "subscribe_workspace":
		h.handleSubscribeWorkspace(ctx, sender, inbound)
	case "unsubscribe_workspace":
		h.handleUnsubscribeWorkspace(sender, inbound)
	case "subscribe_room":
		h.handleSubscribeRoom(ctx, sender, inbound)
	case "unsubscribe_room":
		h.handleUnsubscribeRoom(sender, inbound)
	case "bind_session":
		h.handleBindSession(ctx, sender, inbound)
	case "unbind_session":
		h.handleUnbindSession(ctx, sender, inbound)
	case "chat", "interrupt", "permission_response", "input_queue":
		h.handleControlMessage(ctx, sender, inbound)
	default:
		_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(
			gatewayshared.StringValue(inbound["session_key"]),
			"unknown_message_type",
			"Go 网关已接管入口，但该消息类型尚未实现",
			map[string]any{"type": msgType},
		))
	}
}

func (h *Handler) handleSubscribeRoom(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
) {
	roomID := gatewayshared.StringValue(inbound["room_id"])
	conversationID := gatewayshared.StringValue(inbound["conversation_id"])
	if err := h.validateRoomSubscription(ctx, roomID, conversationID); err != nil {
		h.sendGatewayError(ctx, sender, "", "invalid_room_subscription", err, map[string]any{
			"type":            gatewayshared.StringValue(inbound["type"]),
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
		lastSeenRoomSeq := gatewayshared.Int64Value(inbound["last_seen_room_seq"])
		var lastSeenPtr *int64
		if lastSeenRoomSeq > 0 {
			lastSeenPtr = &lastSeenRoomSeq
		} else if hasPending && latestRoomSeq > 0 {
			lastSeenPtr = &latestRoomSeq
		}
		if err := h.roomSubs.SubscribeRoom(ctx, sender, roomID, conversationID, lastSeenPtr); err != nil {
			h.sendGatewayError(ctx, sender, "", "room_subscription_error", err, map[string]any{
				"type":            gatewayshared.StringValue(inbound["type"]),
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

func (h *Handler) handleUnsubscribeRoom(sender *gatewayshared.WebSocketSender, inbound map[string]any) {
	if h.roomSubs == nil {
		return
	}
	h.roomSubs.UnsubscribeRoom(sender, gatewayshared.StringValue(inbound["room_id"]))
}

func (h *Handler) handleBindSession(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, parsed, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if parsed.Kind == protocol.SessionKeyKindUnknown {
		return
	}
	requestControl, exists := gatewayshared.BoolValue(inbound["request_control"])
	if !exists {
		requestControl = true
	}
	h.permission.BindSession(
		sessionKey,
		sender,
		gatewayshared.StringValue(inbound["client_id"]),
		requestControl,
	)
	if h.channels != nil {
		_ = h.channels.RememberWebSocketRoute(ctx, sessionKey)
	}
	h.broadcastSessionStatus(ctx, sessionKey)
	if parsed.Kind == protocol.SessionKeyKindAgent && h.chat != nil {
		if err := h.chat.SendInputQueueSnapshot(ctx, sessionKey, gatewayshared.StringValue(inbound["agent_id"])); err != nil {
			h.sendGatewayError(ctx, sender, sessionKey, "input_queue_error", err, map[string]any{"type": "bind_session"})
		}
	}
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
	sender *gatewayshared.WebSocketSender,
	roomID string,
	conversationID string,
) bool {
	if h.roomRealtime == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}

	snapshot := h.roomRealtime.GetActiveRoundSnapshot(conversationID)
	if snapshot == nil || len(snapshot.Pending) == 0 {
		return false
	}

	event := protocol.NewChatAckEvent(snapshot.SessionKey, snapshot.RoundID, snapshot.RoundID, snapshot.Pending)
	event.RoomID = roomID
	event.ConversationID = conversationID
	event.CausedBy = snapshot.RoundID
	_ = sender.SendEvent(ctx, event)
	return true
}

func (h *Handler) handleUnbindSession(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, _, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	h.permission.UnbindSession(sessionKey, sender)
	h.broadcastSessionStatus(ctx, sessionKey)
}

func (h *Handler) handleControlMessage(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
) {
	sessionKey, parsed, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if h.ensureSessionBinding(ctx, sender, inbound, sessionKey) {
		return
	}
	if h.rejectControlMessageFromObserver(ctx, sender, inbound, sessionKey) {
		return
	}

	msgType := gatewayshared.StringValue(inbound["type"])
	switch msgType {
	case "chat":
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && h.roomRealtime != nil {
			err = h.roomRealtime.HandleChat(ctx, roompkg.ChatRequest{
				SessionKey:     sessionKey,
				RoomID:         gatewayshared.StringValue(inbound["room_id"]),
				ConversationID: gatewayshared.StringValue(inbound["conversation_id"]),
				Content:        gatewayshared.StringValue(inbound["content"]),
				RoundID:        gatewayshared.StringValue(inbound["round_id"]),
				ReqID:          gatewayshared.StringValue(inbound["req_id"]),
				DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(gatewayshared.StringValue(inbound["delivery_policy"])),
			})
		} else {
			err = h.chat.HandleChat(ctx, chatsvc.Request{
				SessionKey:     sessionKey,
				AgentID:        gatewayshared.StringValue(inbound["agent_id"]),
				Content:        gatewayshared.StringValue(inbound["content"]),
				RoundID:        gatewayshared.StringValue(inbound["round_id"]),
				ReqID:          gatewayshared.StringValue(inbound["req_id"]),
				DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(gatewayshared.StringValue(inbound["delivery_policy"])),
			})
		}
		if err != nil {
			errorType := "chat_error"
			if errors.Is(err, chatsvc.ErrRoomChatNotImplemented) {
				errorType = "not_implemented"
			}
			roundID := gatewayshared.StringValue(inbound["round_id"])
			details := map[string]any{"type": msgType}
			if roundID != "" {
				details["round_id"] = roundID
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
				MsgID:      gatewayshared.StringValue(inbound["msg_id"]),
			})
		} else {
			err = h.chat.HandleInterrupt(ctx, chatsvc.InterruptRequest{
				SessionKey: sessionKey,
				RoundID:    gatewayshared.StringValue(inbound["round_id"]),
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
				RoomID:         gatewayshared.StringValue(inbound["room_id"]),
				ConversationID: gatewayshared.StringValue(inbound["conversation_id"]),
				Action:         action,
				ItemID:         gatewayshared.StringValue(inbound["item_id"]),
				Content:        gatewayshared.StringValue(inbound["content"]),
				OrderedIDs:     stringSliceValue(inbound["ordered_ids"]),
				DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(gatewayshared.StringValue(inbound["delivery_policy"])),
			})
		} else {
			err = h.chat.HandleInputQueue(ctx, chatsvc.InputQueueRequest{
				SessionKey:     sessionKey,
				AgentID:        gatewayshared.StringValue(inbound["agent_id"]),
				Action:         action,
				ItemID:         gatewayshared.StringValue(inbound["item_id"]),
				Content:        gatewayshared.StringValue(inbound["content"]),
				OrderedIDs:     stringSliceValue(inbound["ordered_ids"]),
				DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(gatewayshared.StringValue(inbound["delivery_policy"])),
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

func (h *Handler) ensureSessionBinding(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
	sessionKey string,
) bool {
	if h.permission.IsBound(sessionKey, sender) {
		return false
	}
	if h.permission.HasBindings(sessionKey) {
		return false
	}
	h.permission.BindSession(
		sessionKey,
		sender,
		gatewayshared.StringValue(inbound["client_id"]),
		true,
	)
	h.broadcastSessionStatus(ctx, sessionKey)
	return false
}

func (h *Handler) rejectControlMessageFromObserver(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
	sessionKey string,
) bool {
	if h.permission.IsSessionController(sessionKey, sender) {
		return false
	}
	actionLabel := map[string]string{
		"chat":                "发送消息",
		"input_queue":         "更新待发送队列",
		"interrupt":           "停止生成",
		"permission_response": "确认权限",
	}[gatewayshared.StringValue(inbound["type"])]
	if actionLabel == "" {
		actionLabel = "执行操作"
	}
	_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(
		sessionKey,
		"session_control_denied",
		"当前窗口不是该会话的控制端，无法"+actionLabel,
		map[string]any{"type": gatewayshared.StringValue(inbound["type"])},
	))
	return true
}

func (h *Handler) validateSessionKey(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	inbound map[string]any,
) (string, protocol.SessionKey, bool) {
	sessionKey := gatewayshared.StringValue(inbound["session_key"])
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		errorType := "invalid_session_key"
		if err.Error() == "session_key is required" {
			errorType = "validation_error"
		}
		h.sendGatewayError(ctx, sender, sessionKey, errorType, err, map[string]any{"type": gatewayshared.StringValue(inbound["type"])})
		return "", protocol.SessionKey{}, false
	}
	return normalized, protocol.ParseSessionKey(normalized), true
}

func (h *Handler) sendGatewayError(
	ctx context.Context,
	sender *gatewayshared.WebSocketSender,
	sessionKey string,
	errorType string,
	err error,
	details map[string]any,
) {
	message := h.gatewayErrorDetail(errorType, err)
	_ = sender.SendEvent(ctx, h.newGatewayErrorEvent(sessionKey, errorType, message, details))
}

func (h *Handler) gatewayErrorDetail(errorType string, err error) string {
	if err == nil {
		return "请求失败"
	}
	message := strings.TrimSpace(err.Error())
	switch errorType {
	case "validation_error", "invalid_room_subscription", "invalid_workspace_subscription":
		if gatewayshared.IsClientMessageText(message) {
			return message
		}
		return "请求参数错误"
	case "invalid_session_key":
		return "session_key 不合法"
	case "permission_request_not_found":
		return "未找到待确认的权限请求"
	case "session_control_denied":
		return message
	default:
		if gatewayshared.IsClientMessageError(err) || gatewayshared.IsStructuredSessionKeyError(err) {
			return message
		}
		return "服务内部错误"
	}
}

func (h *Handler) newGatewayErrorEvent(
	sessionKey string,
	errorType string,
	message string,
	details map[string]any,
) protocol.EventMessage {
	data := map[string]any{
		"message":    message,
		"error_type": errorType,
	}
	for key, value := range details {
		data[key] = value
	}
	event := protocol.NewEvent(protocol.EventTypeError, data)
	event.SessionKey = sessionKey
	return event
}

func (h *Handler) broadcastSessionStatus(ctx context.Context, sessionKeys ...string) {
	for _, sessionKey := range sessionKeys {
		if strings.TrimSpace(sessionKey) == "" {
			continue
		}
		_ = h.permission.BroadcastSessionStatus(ctx, sessionKey, h.runtime.GetRunningRoundIDs(sessionKey))
	}
}

func firstStringValue(values ...any) string {
	for _, value := range values {
		if text := gatewayshared.StringValue(value); text != "" {
			return text
		}
	}
	return ""
}

func stringSliceValue(value any) []string {
	rawItems, ok := value.([]any)
	if !ok {
		if typed, ok := value.([]string); ok {
			return typed
		}
		return nil
	}
	result := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		text := gatewayshared.StringValue(item)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}
