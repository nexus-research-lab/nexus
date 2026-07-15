package websocket

import (
	"context"
	"errors"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
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
	logx.Resolve(ctx, h.api.BaseLogger()).Warn("WebSocket chat 请求失败",
		"session_key", sessionKey,
		"type", msgType,
		"client_request_id", clientRequestID,
		"client_message_id", clientMessageID,
		"err", err,
	)
	h.sendGatewayError(ctx, sender, sessionKey, errorType, err, details)
}

func (h *Handler) handleControlMessage(
	ctx context.Context,
	sender *handlershared.WebSocketSender,
	inbound map[string]any,
	dispatcher *controlMessageDispatcher,
) {
	sessionKey, parsed, ok := h.validateSessionKey(ctx, sender, inbound)
	if !ok {
		return
	}
	h.ensureSessionBinding(ctx, sender, sessionKey)
	message := controlMessage{
		handler:    h,
		ctx:        ctx,
		sender:     sender,
		inbound:    inbound,
		sessionKey: sessionKey,
		parsed:     parsed,
		msgType:    handlershared.StringValue(inbound["type"]),
	}
	if dispatcher != nil {
		dispatcher.enqueue(&message)
		return
	}
	message.dispatch()
}

type controlMessage struct {
	handler    *Handler
	ctx        context.Context
	sender     *handlershared.WebSocketSender
	inbound    map[string]any
	sessionKey string
	parsed     protocol.SessionKey
	msgType    string
}

type controlMessageHandler func(*controlMessage)

var controlMessageHandlers = map[string]controlMessageHandler{
	"chat":                (*controlMessage).handleChat,
	"chat_rewrite_last":   (*controlMessage).handleRewriteLast,
	"interrupt":           (*controlMessage).handleInterrupt,
	"input_queue":         (*controlMessage).handleInputQueue,
	"permission_response": (*controlMessage).handlePermissionResponse,
}

func (m *controlMessage) dispatch() {
	handler := controlMessageHandlers[m.msgType]
	if handler != nil {
		handler(m)
		return
	}
	_ = m.sender.SendEvent(m.ctx, m.handler.newGatewayErrorEvent(
		m.sessionKey,
		"not_implemented",
		"Go 运行时已接管控制面，但该写操作尚未实现",
		map[string]any{"type": m.msgType},
	))
}

func (m *controlMessage) handleChat() {
	clientRequestID, clientMessageID := m.clientIDs()
	var err error
	if m.usesRoomRuntime() {
		err = m.handler.roomRealtime.HandleChat(m.ctx, roompkg.ChatRequest{
			SessionKey:        m.sessionKey,
			RoomID:            m.stringValue("room_id"),
			ConversationID:    m.stringValue("conversation_id"),
			AttachmentAgentID: m.stringValue("agent_id"),
			Content:           m.stringValue("content"),
			TargetAgentIDs:    stringSliceValue(m.inbound["target_agent_ids"]),
			Attachments:       m.attachments(),
			ClientRequestID:   clientRequestID,
			ClientMessageID:   clientMessageID,
			DeliveryPolicy:    m.deliveryPolicy(),
		})
	} else {
		err = m.handler.dm.HandleChat(m.ctx, dmsvc.Request{
			SessionKey:      m.sessionKey,
			AgentID:         m.stringValue("agent_id"),
			Content:         m.stringValue("content"),
			Attachments:     m.attachments(),
			ClientRequestID: clientRequestID,
			ClientMessageID: clientMessageID,
			DeliveryPolicy:  m.deliveryPolicy(),
		})
	}
	m.reportChatFailure(clientRequestID, clientMessageID, err)
}

func (m *controlMessage) handleRewriteLast() {
	clientRequestID, clientMessageID := m.clientIDs()
	if m.parsed.Kind == protocol.SessionKeyKindRoom {
		m.reportChatFailure(clientRequestID, clientMessageID, dmsvc.ErrRoomSessionNotImplemented)
		return
	}
	err := m.handler.dm.HandleRewriteLastUserMessage(m.ctx, dmsvc.RewriteRequest{
		SessionKey:      m.sessionKey,
		AgentID:         m.stringValue("agent_id"),
		TargetRoundID:   m.stringValue("target_round_id"),
		ClientRequestID: clientRequestID,
		ClientMessageID: clientMessageID,
		Content:         m.stringValue("content"),
		Attachments:     m.attachments(),
	})
	m.reportChatFailure(clientRequestID, clientMessageID, err)
}

func (m *controlMessage) handleInterrupt() {
	var err error
	if m.usesRoomRuntime() {
		err = m.handler.roomRealtime.HandleInterrupt(m.ctx, roompkg.InterruptRequest{
			SessionKey:   m.sessionKey,
			RoundID:      m.stringValue("round_id"),
			AgentRoundID: m.stringValue("agent_round_id"),
		})
	} else {
		err = m.handler.dm.HandleInterrupt(m.ctx, dmsvc.InterruptRequest{
			SessionKey: m.sessionKey,
			RoundID:    m.stringValue("round_id"),
		})
	}
	m.reportGatewayFailure("interrupt_error", err, map[string]any{"type": m.msgType})
}

func (m *controlMessage) handleInputQueue() {
	action := firstStringValue(m.inbound["action"], m.inbound["action_type"])
	var err error
	if m.usesRoomRuntime() {
		err = m.handler.roomRealtime.HandleInputQueue(m.ctx, roompkg.InputQueueRequest{
			SessionKey:     m.sessionKey,
			RoomID:         m.stringValue("room_id"),
			ConversationID: m.stringValue("conversation_id"),
			Action:         action,
			ItemID:         m.stringValue("item_id"),
			Content:        m.stringValue("content"),
			Attachments:    m.attachments(),
			TargetAgentIDs: stringSliceValue(m.inbound["target_agent_ids"]),
			OrderedIDs:     stringSliceValue(m.inbound["ordered_ids"]),
			DeliveryPolicy: m.deliveryPolicy(),
		})
	} else {
		err = m.handler.dm.HandleInputQueue(m.ctx, dmsvc.InputQueueRequest{
			SessionKey:     m.sessionKey,
			AgentID:        m.stringValue("agent_id"),
			Action:         action,
			ItemID:         m.stringValue("item_id"),
			Content:        m.stringValue("content"),
			Attachments:    m.attachments(),
			OrderedIDs:     stringSliceValue(m.inbound["ordered_ids"]),
			DeliveryPolicy: m.deliveryPolicy(),
		})
	}
	m.reportGatewayFailure("input_queue_error", err, map[string]any{"type": m.msgType, "action": action})
}

func (m *controlMessage) handlePermissionResponse() {
	if m.handler.permission.HandlePermissionResponse(m.inbound) {
		return
	}
	_ = m.sender.SendEvent(m.ctx, m.handler.newGatewayErrorEvent(
		m.sessionKey,
		"permission_request_not_found",
		"未找到待确认的权限请求",
		map[string]any{"type": m.msgType},
	))
}

func (m *controlMessage) usesRoomRuntime() bool {
	return m.parsed.Kind == protocol.SessionKeyKindRoom && m.handler.roomRealtime != nil
}

func (m *controlMessage) stringValue(key string) string {
	return handlershared.StringValue(m.inbound[key])
}

func (m *controlMessage) clientIDs() (string, string) {
	return m.stringValue("client_request_id"), m.stringValue("client_message_id")
}

func (m *controlMessage) attachments() []protocol.ChatAttachment {
	return protocol.ChatAttachmentsFromAny(m.inbound["attachments"])
}

func (m *controlMessage) deliveryPolicy() protocol.ChatDeliveryPolicy {
	return protocol.NormalizeChatDeliveryPolicy(m.stringValue("delivery_policy"))
}

func (m *controlMessage) reportChatFailure(clientRequestID string, clientMessageID string, err error) {
	if err != nil {
		m.handler.sendChatFailure(m.ctx, m.sender, m.sessionKey, m.msgType, clientRequestID, clientMessageID, err)
	}
}

func (m *controlMessage) reportGatewayFailure(errorType string, err error, details map[string]any) {
	if err != nil {
		m.handler.sendGatewayError(m.ctx, m.sender, m.sessionKey, errorType, err, details)
	}
}
