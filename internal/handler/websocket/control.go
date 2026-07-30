package websocket

import (
	"context"
	"errors"
	"strings"

	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
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
	attachments := m.attachments()
	if handled, err := m.executeHostCommand(
		clientRequestID,
		clientMessageID,
		len(attachments),
	); handled {
		m.reportChatFailure(clientRequestID, clientMessageID, err)
		return
	}
	var err error
	if m.usesRoomRuntime() {
		err = m.handler.roomRealtime.HandleChat(m.ctx, roomrealtime.ChatRequest{
			SessionKey:        m.sessionKey,
			RoomID:            m.stringValue("room_id"),
			ConversationID:    m.stringValue("conversation_id"),
			AttachmentAgentID: m.stringValue("agent_id"),
			Content:           m.stringValue("content"),
			TargetAgentIDs:    stringSliceValue(m.inbound["target_agent_ids"]),
			Attachments:       attachments,
			ClientRequestID:   clientRequestID,
			ClientMessageID:   clientMessageID,
			DeliveryPolicy:    m.deliveryPolicy(),
		})
	} else {
		err = m.handler.dm.HandleChat(m.ctx, dmsvc.Request{
			SessionKey:      m.sessionKey,
			AgentID:         m.stringValue("agent_id"),
			Content:         m.stringValue("content"),
			Attachments:     attachments,
			ClientRequestID: clientRequestID,
			ClientMessageID: clientMessageID,
			DeliveryPolicy:  m.deliveryPolicy(),
		})
	}
	m.reportChatFailure(clientRequestID, clientMessageID, err)
}

func (m *controlMessage) executeHostCommand(
	clientRequestID string,
	clientMessageID string,
	attachmentCount int,
) (bool, error) {
	if m.handler.hostCommands == nil {
		return false, nil
	}
	scope := slashcommandsvc.ScopeDM
	if m.parsed.Kind == protocol.SessionKeyKindRoom {
		scope = slashcommandsvc.ScopeRoom
	}
	invocation := slashcommandsvc.Invocation{
		SessionKey:      m.sessionKey,
		AgentID:         firstStringValue(m.inbound["agent_id"], m.parsed.AgentID),
		Content:         m.stringValue("content"),
		AttachmentCount: attachmentCount,
	}
	result, matched, err := m.handler.hostCommands.ExecuteAuthorized(
		m.ctx,
		scope,
		invocation,
		func(ctx context.Context, authorizedInvocation slashcommandsvc.Invocation) error {
			return m.handler.authorizeHostCommand(ctx, scope, authorizedInvocation)
		},
	)
	if !matched || err != nil {
		return matched, err
	}
	roundID := protocol.NewRoundID()
	ack := protocol.NewChatAckEvent(
		m.sessionKey,
		clientRequestID,
		clientMessageID,
		roundID,
		protocol.NewUserMessageID(),
		false,
		nil,
	)
	if err = m.sender.SendEvent(m.ctx, ack); err != nil {
		return true, err
	}
	for _, event := range result.Events {
		// host handler 只能向触发它的 session 回写事件，避免错误实现把事件投到别的会话。
		event.SessionKey = m.sessionKey
		if err = m.sender.SendEvent(m.ctx, event); err != nil {
			return true, err
		}
	}
	return true, nil
}

func (h *Handler) authorizeHostCommand(
	ctx context.Context,
	scope slashcommandsvc.Scope,
	invocation slashcommandsvc.Invocation,
) error {
	switch scope {
	case slashcommandsvc.ScopeDM:
		if h == nil || h.dm == nil {
			return errors.New("DM service is unavailable")
		}
		return h.dm.AuthorizeHostCommand(ctx, invocation.SessionKey, invocation.AgentID)
	case slashcommandsvc.ScopeRoom:
		if h == nil || h.roomService == nil {
			return errors.New("Room service is unavailable")
		}
		parsed := protocol.ParseSessionKey(invocation.SessionKey)
		if parsed.Kind != protocol.SessionKeyKindRoom || !parsed.IsShared {
			return errors.New("host Slash requires a shared Room session")
		}
		contextValue, err := h.roomService.GetConversationContext(ctx, parsed.ConversationID)
		if err != nil {
			return err
		}
		if contextValue == nil || contextValue.Room.RoomType != protocol.RoomTypeGroup {
			return errors.New("host Slash requires a group Room")
		}
		if agentID := strings.TrimSpace(invocation.AgentID); agentID != "" &&
			!roomHasAgent(contextValue.Members, agentID) {
			return errors.New("agent_id is not a Room member")
		}
		return nil
	default:
		return errors.New("unsupported host Slash scope")
	}
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
		err = m.handler.roomRealtime.HandleInterrupt(m.ctx, roomrealtime.InterruptRequest{
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
	if action == "" {
		action = "enqueue"
	}
	clientRequestID, clientMessageID := m.clientIDs()
	itemID := m.stringValue("item_id")
	var (
		result protocol.InputQueueMutationResult
		err    error
	)
	if m.usesRoomRuntime() {
		result, err = m.handler.roomRealtime.HandleInputQueue(m.ctx, roomrealtime.InputQueueRequest{
			SessionKey:      m.sessionKey,
			RoomID:          m.stringValue("room_id"),
			ConversationID:  m.stringValue("conversation_id"),
			ClientMessageID: clientMessageID,
			Action:          action,
			ItemID:          itemID,
			Content:         m.stringValue("content"),
			Attachments:     m.attachments(),
			TargetAgentIDs:  stringSliceValue(m.inbound["target_agent_ids"]),
			OrderedIDs:      stringSliceValue(m.inbound["ordered_ids"]),
			DeliveryPolicy:  m.deliveryPolicy(),
		})
	} else {
		result, err = m.handler.dm.HandleInputQueue(m.ctx, dmsvc.InputQueueRequest{
			SessionKey:      m.sessionKey,
			AgentID:         m.stringValue("agent_id"),
			ClientMessageID: clientMessageID,
			Action:          action,
			ItemID:          itemID,
			Content:         m.stringValue("content"),
			Attachments:     m.attachments(),
			OrderedIDs:      stringSliceValue(m.inbound["ordered_ids"]),
			DeliveryPolicy:  m.deliveryPolicy(),
		})
	}
	if err != nil {
		m.reportGatewayFailure("input_queue_error", err, map[string]any{
			"type":              m.msgType,
			"action":            action,
			"item_id":           itemID,
			"client_request_id": clientRequestID,
			"client_message_id": clientMessageID,
		})
		return
	}
	if ackErr := m.sender.SendEvent(
		m.ctx,
		protocol.NewInputQueueAckEvent(m.sessionKey, clientRequestID, clientMessageID, result),
	); ackErr != nil {
		logx.Resolve(m.ctx, m.handler.api.BaseLogger()).Warn("WebSocket input_queue ACK 发送失败",
			"session_key", m.sessionKey,
			"action", result.Action,
			"item_id", result.ItemID,
			"client_request_id", clientRequestID,
			"client_message_id", clientMessageID,
			"err", ackErr,
		)
	}
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
