// INPUT: Authenticated WebSocket control messages, host commands, and DM/Room
// authorization services.
// OUTPUT: Correlated control results plus bounded detached set_goal mutations
// whose durable effects do not depend on the originating sender remaining open.
// POS: WebSocket control adapter; host command business rules stay in
// service/slashcommand and the DM/Room/Goal services.
package websocket

import (
	"context"
	"errors"
	"strings"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	slashcommandsvc "github.com/nexus-research-lab/nexus/internal/service/slashcommand"
)

const (
	detachedGoalCommandTimeout  = 15 * time.Second
	detachedGoalDeliveryTimeout = 2 * time.Second
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
	msgType := handlershared.StringValue(inbound["type"])
	if msgType == "permission_response" {
		if !h.permission.IsBound(sessionKey, sender) {
			h.sendGatewayError(
				ctx,
				sender,
				sessionKey,
				"permission_request_not_found",
				errors.New("当前连接未绑定该权限请求的会话"),
				map[string]any{"type": msgType},
			)
			return
		}
	} else {
		h.ensureSessionBinding(ctx, sender, sessionKey)
	}
	message := controlMessage{
		handler:    h,
		ctx:        ctx,
		sender:     sender,
		inbound:    inbound,
		sessionKey: sessionKey,
		parsed:     parsed,
		msgType:    msgType,
	}
	if msgType == "set_goal" {
		clientRequestID, clientMessageID := message.clientIDs()
		if err := message.validateDetachedGoalCommand(); err != nil {
			message.reportChatFailure(clientRequestID, clientMessageID, err)
			return
		}
		message.bestEffortDelivery = true
		if dispatcher != nil {
			dispatcher.enqueueDetached(&message, detachedGoalCommandTimeout)
			return
		}
		detachedCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			detachedGoalCommandTimeout,
		)
		defer cancel()
		message.ctx = detachedCtx
		message.dispatch()
		return
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
	// bestEffortDelivery is set only after set_goal passed synchronous scope
	// authorization. Once its detached host mutation starts, sender delivery is
	// an observation channel rather than part of the mutation's success boundary.
	bestEffortDelivery bool
}

type controlMessageHandler func(*controlMessage)

var controlMessageHandlers = map[string]controlMessageHandler{
	"chat":                (*controlMessage).handleChat,
	"chat_rewrite_last":   (*controlMessage).handleRewriteLast,
	"interrupt":           (*controlMessage).handleInterrupt,
	"input_queue":         (*controlMessage).handleInputQueue,
	"permission_response": (*controlMessage).handlePermissionResponse,
	"set_goal":            (*controlMessage).handleSetGoal,
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
	content := m.stringValue("content")
	if handled, hostErr := m.executeHostCommand(
		clientRequestID,
		clientMessageID,
		len(attachments),
		content,
	); handled {
		m.reportChatFailure(clientRequestID, clientMessageID, hostErr)
		return
	}
	var err error
	if m.usesRoomRuntime() {
		err = m.handler.roomRealtime.HandleChat(m.ctx, roomrealtime.ChatRequest{
			SessionKey:                  m.sessionKey,
			RoomID:                      m.stringValue("room_id"),
			ConversationID:              m.stringValue("conversation_id"),
			AttachmentAgentID:           m.stringValue("agent_id"),
			Content:                     m.stringValue("content"),
			TargetAgentIDs:              stringSliceValue(m.inbound["target_agent_ids"]),
			Attachments:                 attachments,
			ClientRequestID:             clientRequestID,
			ClientMessageID:             clientMessageID,
			DeliveryPolicy:              m.deliveryPolicy(),
			TrustedConfigurationContext: true,
		})
	} else {
		err = m.handler.dm.HandleRealtimeChat(m.ctx, dmsvc.Request{
			SessionKey:                  m.sessionKey,
			AgentID:                     m.stringValue("agent_id"),
			Content:                     m.stringValue("content"),
			Attachments:                 attachments,
			ClientRequestID:             clientRequestID,
			ClientMessageID:             clientMessageID,
			DeliveryPolicy:              m.deliveryPolicy(),
			TrustedConfigurationContext: true,
		})
	}
	m.reportChatFailure(clientRequestID, clientMessageID, err)
}

// handleSetGoal 把 Goal Composer 提交转成与 `/goal` 相同的 host command；
// 它不会进入普通 chat/runtime 路径。
func (m *controlMessage) handleSetGoal() {
	clientRequestID, clientMessageID := m.clientIDs()
	objective := strings.TrimSpace(m.stringValue("objective"))
	if objective == "" {
		m.reportChatFailure(clientRequestID, clientMessageID, errors.New("goal objective is required"))
		return
	}
	handled, err := m.executeHostCommand(
		clientRequestID,
		clientMessageID,
		0,
		"/goal "+objective,
	)
	if !handled && err == nil {
		err = errors.New("Goal command is unavailable")
	}
	m.reportChatFailure(clientRequestID, clientMessageID, err)
}

// validateDetachedGoalCommand completes cheap input and owner/session checks
// before the mutation is allowed to outlive the WebSocket connection. The host
// registry repeats authorization inside the detached job as a fail-closed fence.
func (m *controlMessage) validateDetachedGoalCommand() error {
	if strings.TrimSpace(m.stringValue("objective")) == "" {
		return errors.New("goal objective is required")
	}
	if m.handler == nil || m.handler.hostCommands == nil {
		return errors.New("Goal command is unavailable")
	}
	if _, err := goalCommandOptionsValue(m.inbound["goal_options"]); err != nil {
		return err
	}
	return m.handler.authorizeHostCommand(
		m.ctx,
		m.hostCommandScope(),
		slashcommandsvc.Invocation{
			SessionKey:     m.sessionKey,
			AgentID:        firstStringValue(m.inbound["agent_id"], m.parsed.AgentID),
			TargetAgentIDs: stringSliceValue(m.inbound["target_agent_ids"]),
		},
	)
}

func (m *controlMessage) hostCommandScope() slashcommandsvc.Scope {
	if m.parsed.Kind == protocol.SessionKeyKindRoom {
		return slashcommandsvc.ScopeRoom
	}
	return slashcommandsvc.ScopeDM
}

func (m *controlMessage) executeHostCommand(
	clientRequestID string,
	clientMessageID string,
	attachmentCount int,
	commandContent string,
) (bool, error) {
	if m.handler.hostCommands == nil {
		return false, nil
	}
	scope := m.hostCommandScope()
	roundID := protocol.NewRoundID()
	userMessageID := protocol.NewUserMessageID()
	goalOptions := protocol.GoalCommandOptions{}
	if isGoalHostCommandContent(commandContent) {
		var err error
		goalOptions, err = goalCommandOptionsValue(m.inbound["goal_options"])
		if err != nil {
			return true, err
		}
	}
	invocation := slashcommandsvc.Invocation{
		SessionKey:      m.sessionKey,
		AgentID:         firstStringValue(m.inbound["agent_id"], m.parsed.AgentID),
		RoundID:         roundID,
		UserMessageID:   userMessageID,
		ClientRequestID: clientRequestID,
		ClientMessageID: clientMessageID,
		Content:         strings.TrimSpace(commandContent),
		TargetAgentIDs:  stringSliceValue(m.inbound["target_agent_ids"]),
		GoalOptions:     goalOptions,
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
	if result.AfterResponseAttempted != nil {
		defer result.AfterResponseAttempted(context.WithoutCancel(m.ctx))
	}
	deliveryCtx, cancelDelivery := m.deliveryContext()
	defer cancelDelivery()
	ack := protocol.NewTransientChatAckEvent(m.sessionKey, clientRequestID, clientMessageID, roundID, userMessageID)
	if result.UserMessageCommitted {
		ack = protocol.NewChatAckEvent(
			m.sessionKey,
			clientRequestID,
			clientMessageID,
			roundID,
			userMessageID,
			true,
			nil,
		)
	}
	var deliveryErr error
	recordDeliveryError := func(sendErr error) {
		if sendErr != nil && deliveryErr == nil {
			deliveryErr = sendErr
		}
	}
	recordDeliveryError(m.sender.SendEvent(deliveryCtx, ack))
	for _, event := range result.Events {
		// host handler 只能向触发它的 session 回写事件，避免错误实现把事件投到别的会话。
		event.SessionKey = m.sessionKey
		recordDeliveryError(m.sender.SendEvent(deliveryCtx, event))
	}
	if invalidation := result.DirectoryInvalidation; invalidation != nil {
		invalidationCtx, cancelInvalidation := m.deliveryContext()
		m.handler.BroadcastDirectoryChanged(
			invalidationCtx,
			invalidation.Reason,
			invalidation.Data,
		)
		cancelInvalidation()
	}
	recordDeliveryError(m.sender.SendEvent(
		deliveryCtx,
		protocol.NewRoundStatusEvent(
			m.sessionKey,
			roundID,
			protocol.RoundStatusFinished,
			"success",
		),
	))
	if deliveryErr != nil && m.bestEffortDelivery {
		logx.Resolve(deliveryCtx, m.handler.api.BaseLogger()).Debug(
			"Goal command committed after sender delivery became unavailable",
			"session_key", m.sessionKey,
			"client_request_id", clientRequestID,
			"client_message_id", clientMessageID,
			"err", deliveryErr,
		)
		return true, nil
	}
	return true, deliveryErr
}

func isGoalHostCommandContent(content string) bool {
	fields := strings.Fields(strings.TrimSpace(content))
	return len(fields) > 0 && strings.EqualFold(fields[0], "/goal")
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
	clientRequestID := m.stringValue("client_request_id")
	roundID := m.stringValue("round_id")
	agentRoundID := m.stringValue("agent_round_id")
	var err error
	if m.usesRoomRuntime() {
		err = m.handler.roomRealtime.HandleInterrupt(m.ctx, roomrealtime.InterruptRequest{
			SessionKey:   m.sessionKey,
			RoundID:      roundID,
			AgentRoundID: agentRoundID,
		})
	} else {
		err = m.handler.dm.HandleInterrupt(m.ctx, dmsvc.InterruptRequest{
			SessionKey: m.sessionKey,
			RoundID:    roundID,
		})
	}
	if err != nil {
		m.reportGatewayFailure("interrupt_error", err, map[string]any{
			"type":              m.msgType,
			"client_request_id": clientRequestID,
			"round_id":          roundID,
			"agent_round_id":    agentRoundID,
		})
		return
	}
	if clientRequestID == "" {
		return
	}
	if ackErr := m.sender.SendEvent(
		m.ctx,
		protocol.NewInterruptAckEvent(
			m.sessionKey,
			clientRequestID,
			roundID,
			agentRoundID,
		),
	); ackErr != nil {
		logx.Resolve(m.ctx, m.handler.api.BaseLogger()).Warn("WebSocket interrupt ACK 发送失败",
			"session_key", m.sessionKey,
			"client_request_id", clientRequestID,
			"round_id", roundID,
			"agent_round_id", agentRoundID,
			"err", ackErr,
		)
	}
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
			SessionKey:                  m.sessionKey,
			RoomID:                      m.stringValue("room_id"),
			ConversationID:              m.stringValue("conversation_id"),
			ClientMessageID:             clientMessageID,
			Action:                      action,
			ItemID:                      itemID,
			Content:                     m.stringValue("content"),
			Attachments:                 m.attachments(),
			TargetAgentIDs:              stringSliceValue(m.inbound["target_agent_ids"]),
			OrderedIDs:                  stringSliceValue(m.inbound["ordered_ids"]),
			DeliveryPolicy:              m.deliveryPolicy(),
			TrustedConfigurationContext: true,
		})
	} else {
		result, err = m.handler.dm.HandleInputQueue(m.ctx, dmsvc.InputQueueRequest{
			SessionKey:                  m.sessionKey,
			AgentID:                     m.stringValue("agent_id"),
			ClientMessageID:             clientMessageID,
			Action:                      action,
			ItemID:                      itemID,
			Content:                     m.stringValue("content"),
			Attachments:                 m.attachments(),
			OrderedIDs:                  stringSliceValue(m.inbound["ordered_ids"]),
			DeliveryPolicy:              m.deliveryPolicy(),
			TrustedConfigurationContext: true,
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
	if m.handler.permission.HandlePermissionResponse(m.ctx, m.sessionKey, m.inbound) {
		return
	}
	if m.handler.automationPermissions != nil {
		handled, err := m.handler.automationPermissions.ResolveSessionPermissionResponse(
			m.ctx,
			m.sessionKey,
			m.inbound,
		)
		if err != nil {
			errorType := "permission_response_error"
			if errors.Is(err, automationdomain.ErrPermissionRequestResolved) ||
				errors.Is(err, automationdomain.ErrPermissionRequestStale) {
				errorType = "permission_request_not_found"
			}
			m.handler.sendGatewayError(
				m.ctx,
				m.sender,
				m.sessionKey,
				errorType,
				err,
				map[string]any{"type": m.msgType},
			)
			return
		}
		if handled {
			return
		}
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
		deliveryCtx, cancelDelivery := m.deliveryContext()
		defer cancelDelivery()
		m.handler.sendChatFailure(deliveryCtx, m.sender, m.sessionKey, m.msgType, clientRequestID, clientMessageID, err)
	}
}

// deliveryContext separates the accepted Goal mutation deadline from its final
// transport attempt. It preserves authenticated context values, but removing a
// canceled business deadline never restarts or extends the mutation itself.
func (m *controlMessage) deliveryContext() (context.Context, context.CancelFunc) {
	if !m.bestEffortDelivery {
		return m.ctx, func() {}
	}
	return context.WithTimeout(
		context.WithoutCancel(m.ctx),
		detachedGoalDeliveryTimeout,
	)
}

func (m *controlMessage) reportGatewayFailure(errorType string, err error, details map[string]any) {
	if err != nil {
		m.handler.sendGatewayError(m.ctx, m.sender, m.sessionKey, errorType, err, details)
	}
}
