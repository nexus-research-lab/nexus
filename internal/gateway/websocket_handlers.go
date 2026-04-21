// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：websocket_handlers.go
// @Date   ：2026/04/17 10:30:00
// @Author ：leemysw
// 2026/04/17 10:30:00   Create
// =====================================================

package gateway

import (
	"context"
	"errors"
	"net/http"
	"strings"

	chatsvc "github.com/nexus-research-lab/nexus/internal/chat"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	room2 "github.com/nexus-research-lab/nexus/internal/room"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func (s *Server) handleWebSocket(writer http.ResponseWriter, request *http.Request) {
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		return
	}
	sender := newWebSocketSender(connection)
	defer func() {
		sender.MarkClosed()
		if s.workspaceSubs != nil {
			s.workspaceSubs.UnregisterSender(sender)
		}
		if s.roomSubs != nil {
			s.roomSubs.UnregisterSender(sender)
		}
		_ = connection.Close(websocket.StatusNormalClosure, "closed")
		s.broadcastSessionStatus(request.Context(), s.permission.UnregisterSender(sender)...)
	}()

	ctx := request.Context()
	for {
		var inbound map[string]any
		if err := wsjson.Read(ctx, connection, &inbound); err != nil {
			return
		}
		s.dispatchWebSocketMessage(ctx, sender, inbound)
	}
}

func (s *Server) dispatchWebSocketMessage(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	msgType := stringValue(inbound["type"])
	switch msgType {
	case "ping":
		_ = sender.SendEvent(ctx, protocol.NewPongEvent(stringValue(inbound["session_key"])))
		return
	case "subscribe_workspace":
		s.handleSubscribeWorkspace(ctx, sender, inbound)
		return
	case "unsubscribe_workspace":
		s.handleUnsubscribeWorkspace(sender, inbound)
		return
	case "subscribe_room":
		s.handleSubscribeRoom(ctx, sender, inbound)
		return
	case "unsubscribe_room":
		s.handleUnsubscribeRoom(sender, inbound)
		return
	case "bind_session":
		s.handleBindSession(ctx, sender, inbound)
		return
	case "unbind_session":
		s.handleUnbindSession(ctx, sender, inbound)
		return
	case "chat", "interrupt", "permission_response":
		s.handleControlMessage(ctx, sender, inbound)
		return
	default:
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			stringValue(inbound["session_key"]),
			"unknown_message_type",
			"Go 网关已接管入口，但该消息类型尚未实现",
			map[string]any{"type": msgType},
		))
		return
	}
}

func (s *Server) handleSubscribeRoom(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	roomID := stringValue(inbound["room_id"])
	conversationID := stringValue(inbound["conversation_id"])
	if err := s.validateRoomSubscription(ctx, roomID, conversationID); err != nil {
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			"",
			"invalid_room_subscription",
			err.Error(),
			map[string]any{
				"type":            stringValue(inbound["type"]),
				"room_id":         roomID,
				"conversation_id": conversationID,
			},
		))
		return
	}
	var latestRoomSeq int64
	if s.roomSubs != nil {
		latestRoomSeq = s.roomSubs.CurrentRoomSeq(roomID)
	}
	hasPending := s.restoreRoomPendingSlots(ctx, sender, roomID, conversationID)
	if s.roomSubs != nil {
		lastSeenRoomSeq := int64Value(inbound["last_seen_room_seq"])
		var lastSeenPtr *int64
		if lastSeenRoomSeq > 0 {
			lastSeenPtr = &lastSeenRoomSeq
		} else if hasPending && latestRoomSeq > 0 {
			lastSeenPtr = &latestRoomSeq
		}
		if err := s.roomSubs.SubscribeRoom(ctx, sender, roomID, conversationID, lastSeenPtr); err != nil {
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				"",
				"room_subscription_error",
				err.Error(),
				map[string]any{
					"type":            stringValue(inbound["type"]),
					"room_id":         roomID,
					"conversation_id": conversationID,
				},
			))
			return
		}
	}
}

func (s *Server) handleUnsubscribeRoom(sender *websocketSender, inbound map[string]any) {
	if s.roomSubs == nil {
		return
	}
	s.roomSubs.UnsubscribeRoom(sender, stringValue(inbound["room_id"]))
}

func (s *Server) handleBindSession(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	sessionKey, parsed, ok := s.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if parsed.Kind == protocol.SessionKeyKindUnknown {
		return
	}
	requestControl, requestControlExists := boolValue(inbound["request_control"])
	if !requestControlExists {
		requestControl = true
	}
	s.permission.BindSession(
		sessionKey,
		sender,
		stringValue(inbound["client_id"]),
		requestControl,
	)
	if s.channels != nil {
		_ = s.channels.RememberWebSocketRoute(ctx, sessionKey)
	}
	s.broadcastSessionStatus(ctx, sessionKey)
}

func (s *Server) validateRoomSubscription(ctx context.Context, roomID string, conversationID string) error {
	if strings.TrimSpace(roomID) == "" {
		return errors.New("room_id is required")
	}
	if strings.TrimSpace(conversationID) == "" {
		_, err := s.roomService.GetRoom(ctx, roomID)
		return err
	}

	contextValue, err := s.roomService.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	if contextValue.Room.ID != roomID {
		return errors.New("conversation_id does not belong to room_id")
	}
	return nil
}

func (s *Server) restoreRoomPendingSlots(ctx context.Context, sender *websocketSender, roomID string, conversationID string) bool {
	if s.roomRealtime == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}

	snapshot := s.roomRealtime.GetActiveRoundSnapshot(conversationID)
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

func (s *Server) handleUnbindSession(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	sessionKey, _, ok := s.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	s.permission.UnbindSession(sessionKey, sender)
	s.broadcastSessionStatus(ctx, sessionKey)
}

func (s *Server) handleControlMessage(ctx context.Context, sender *websocketSender, inbound map[string]any) {
	sessionKey, parsed, ok := s.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	if s.ensureSessionBinding(ctx, sender, inbound, sessionKey) {
		return
	}
	if s.rejectControlMessageFromObserver(ctx, sender, inbound, sessionKey) {
		return
	}

	msgType := stringValue(inbound["type"])
	switch msgType {
	case "chat":
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && s.roomRealtime != nil {
			err = s.roomRealtime.HandleChat(ctx, room2.ChatRequest{
				SessionKey:     sessionKey,
				RoomID:         stringValue(inbound["room_id"]),
				ConversationID: stringValue(inbound["conversation_id"]),
				Content:        stringValue(inbound["content"]),
				RoundID:        stringValue(inbound["round_id"]),
				ReqID:          stringValue(inbound["req_id"]),
			})
		} else {
			err = s.chat.HandleChat(ctx, chatsvc.Request{
				SessionKey: sessionKey,
				AgentID:    stringValue(inbound["agent_id"]),
				Content:    stringValue(inbound["content"]),
				RoundID:    stringValue(inbound["round_id"]),
				ReqID:      stringValue(inbound["req_id"]),
			})
		}
		if err != nil {
			errorType := "chat_error"
			if errors.Is(err, chatsvc.ErrRoomChatNotImplemented) {
				errorType = "not_implemented"
			}
			roundID := stringValue(inbound["round_id"])
			details := map[string]any{"type": msgType}
			if roundID != "" {
				details["round_id"] = roundID
			}
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				sessionKey,
				errorType,
				err.Error(),
				details,
			))
			if roundID != "" {
				_ = sender.SendEvent(ctx, protocol.NewRoundStatusEvent(sessionKey, roundID, "error", "error"))
			}
		}
	case "interrupt":
		var err error
		if parsed.Kind == protocol.SessionKeyKindRoom && s.roomRealtime != nil {
			err = s.roomRealtime.HandleInterrupt(ctx, room2.InterruptRequest{
				SessionKey: sessionKey,
				MsgID:      stringValue(inbound["msg_id"]),
			})
		} else {
			err = s.chat.HandleInterrupt(ctx, chatsvc.InterruptRequest{
				SessionKey: sessionKey,
				RoundID:    stringValue(inbound["round_id"]),
			})
		}
		if err != nil {
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				sessionKey,
				"interrupt_error",
				err.Error(),
				map[string]any{"type": msgType},
			))
		}
	case "permission_response":
		if !s.permission.HandlePermissionResponse(inbound) {
			_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
				sessionKey,
				"permission_request_not_found",
				"未找到待确认的权限请求",
				map[string]any{"type": msgType},
			))
		}
	default:
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			sessionKey,
			"not_implemented",
			"Go 运行时已接管控制面，但该写操作尚未实现",
			map[string]any{"type": msgType},
		))
	}
}

func (s *Server) ensureSessionBinding(ctx context.Context, sender *websocketSender, inbound map[string]any, sessionKey string) bool {
	if s.permission.IsBound(sessionKey, sender) {
		return false
	}
	if s.permission.HasBindings(sessionKey) {
		return false
	}
	s.permission.BindSession(
		sessionKey,
		sender,
		stringValue(inbound["client_id"]),
		true,
	)
	s.broadcastSessionStatus(ctx, sessionKey)
	return false
}

func (s *Server) rejectControlMessageFromObserver(ctx context.Context, sender *websocketSender, inbound map[string]any, sessionKey string) bool {
	if s.permission.IsSessionController(sessionKey, sender) {
		return false
	}
	actionLabel := map[string]string{
		"chat":                "发送消息",
		"interrupt":           "停止生成",
		"permission_response": "确认权限",
	}[stringValue(inbound["type"])]
	if actionLabel == "" {
		actionLabel = "执行操作"
	}
	_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
		sessionKey,
		"session_control_denied",
		"当前窗口不是该会话的控制端，无法"+actionLabel,
		map[string]any{"type": stringValue(inbound["type"])},
	))
	return true
}

func (s *Server) validateSessionKey(ctx context.Context, sender *websocketSender, inbound map[string]any) (string, protocol.SessionKey, bool) {
	sessionKey := stringValue(inbound["session_key"])
	normalized, err := protocol.RequireStructuredSessionKey(sessionKey)
	if err != nil {
		errorType := "invalid_session_key"
		if err.Error() == "session_key is required" {
			errorType = "validation_error"
		}
		_ = sender.SendEvent(ctx, s.newGatewayErrorEvent(
			sessionKey,
			errorType,
			err.Error(),
			map[string]any{"type": stringValue(inbound["type"])},
		))
		return "", protocol.SessionKey{}, false
	}
	return normalized, protocol.ParseSessionKey(normalized), true
}

func (s *Server) newGatewayErrorEvent(sessionKey string, errorType string, message string, details map[string]any) protocol.EventMessage {
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

func (s *Server) broadcastRoomEvent(
	ctx context.Context,
	roomID string,
	eventType protocol.EventType,
	data map[string]any,
) {
	if s.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	event := protocol.NewEvent(eventType, data)
	event.RoomID = strings.TrimSpace(roomID)
	s.roomSubs.Broadcast(ctx, event.RoomID, event)
}

func (s *Server) broadcastRoomResyncRequired(
	ctx context.Context,
	roomID string,
	conversationID string,
	reason string,
) {
	if s.roomSubs == nil || strings.TrimSpace(roomID) == "" {
		return
	}
	data := map[string]any{
		"room_id":         strings.TrimSpace(roomID),
		"conversation_id": strings.TrimSpace(conversationID),
		"reason":          strings.TrimSpace(reason),
	}
	event := protocol.NewEvent(protocol.EventTypeRoomResyncRequired, data)
	event.RoomID = data["room_id"].(string)
	s.roomSubs.Broadcast(ctx, event.RoomID, event)
}

func (s *Server) broadcastSessionStatus(ctx context.Context, sessionKeys ...string) {
	for _, sessionKey := range sessionKeys {
		if strings.TrimSpace(sessionKey) == "" {
			continue
		}
		_ = s.permission.BroadcastSessionStatus(ctx, sessionKey, s.runtime.GetRunningRoundIDs(sessionKey))
	}
}
