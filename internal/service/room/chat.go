// INPUT: Room 用户输入、内部触发与当前 round/queue 状态。
// OUTPUT: 持久化的共享消息，或串行接力的 Room round。
// POS: Room 输入从受理到 runtime 启动的原子交接边界。
package room

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
)

// roomChatExecution 保存一次 Room 输入从受理到启动 round 的业务态。
type roomChatExecution struct {
	service            *RealtimeService
	ctx                context.Context
	request            ChatRequest
	sessionKey         string
	roomID             string
	conversationID     string
	contextValue       *protocol.ConversationContextAggregate
	attachments        []protocol.ChatAttachment
	runtimeTriggerText string
	agentNameByID      map[string]string
	agentByID          map[string]*protocol.Agent
	targetAgentIDs     []string
	targetResolution   string
	deliveryPolicy     protocol.ChatDeliveryPolicy
	history            []protocol.Message
	userMessage        protocol.Message
}

func (s *RealtimeService) buildAgentDirectory(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) (map[string]string, map[string]*protocol.Agent, error) {
	agentNameByID := make(map[string]string)
	agentByID := make(map[string]*protocol.Agent)
	if contextValue == nil {
		return agentNameByID, agentByID, nil
	}
	memberIDs := make(map[string]struct{})
	for _, member := range contextValue.Members {
		if member.MemberType != protocol.MemberTypeAgent || strings.TrimSpace(member.MemberAgentID) == "" {
			continue
		}
		memberIDs[strings.TrimSpace(member.MemberAgentID)] = struct{}{}
	}
	for _, agentValue := range contextValue.MemberAgents {
		if _, ok := memberIDs[agentValue.AgentID]; !ok {
			continue
		}
		item := agentValue
		agentNameByID[item.AgentID] = item.Name
		agentByID[item.AgentID] = &item
	}
	for agentID := range memberIDs {
		if _, ok := agentByID[agentID]; ok {
			continue
		}
		agentValue, err := s.agents.GetAgent(ctx, agentID)
		if err != nil {
			return nil, nil, err
		}
		agentNameByID[agentValue.AgentID] = agentValue.Name
		agentByID[agentValue.AgentID] = agentValue
	}
	return agentNameByID, agentByID, nil
}

func (s *RealtimeService) scheduleTitleGeneration(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	content string,
	provider string,
	model string,
) {
	if s.titles == nil || contextValue == nil {
		return
	}
	s.titles.Schedule(ctx, titlegen.Request{
		OwnerUserID:              authctx.OwnerUserID(ctx),
		SessionKey:               sessionKey,
		Provider:                 strings.TrimSpace(provider),
		Model:                    strings.TrimSpace(model),
		Content:                  content,
		SessionMessageCount:      -1,
		ConversationID:           contextValue.Conversation.ID,
		ConversationRoomID:       contextValue.Room.ID,
		ConversationTitle:        contextValue.Conversation.Title,
		ConversationRoomName:     contextValue.Room.Name,
		ConversationMessageCount: contextValue.Conversation.MessageCount,
	})
}

func resolveTitleRuntimeTarget(
	targetAgentIDs []string,
	agentByID map[string]*protocol.Agent,
) (string, string) {
	for _, agentID := range targetAgentIDs {
		agentValue := agentByID[strings.TrimSpace(agentID)]
		if agentValue == nil {
			continue
		}
		return strings.TrimSpace(agentValue.Options.Provider), strings.TrimSpace(agentValue.Options.Model)
	}
	return "", ""
}

// HandleChat 处理 Room 主对话消息。
func (s *RealtimeService) HandleChat(ctx context.Context, request ChatRequest) error {
	if request.Internal {
		return s.handleChat(ctx, request)
	}
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()
	return s.handleChat(ctx, request)
}

func (s *RealtimeService) handleChat(ctx context.Context, request ChatRequest) error {
	execution, err := s.prepareRoomChat(ctx, request)
	if err != nil {
		return err
	}
	if err = execution.persistInput(); err != nil {
		return err
	}
	if handled, handleErr := execution.finishWithoutTarget(); handled {
		return handleErr
	}
	if handled, routeErr := execution.routeActiveSlots(); handled {
		return routeErr
	}

	activeRound, pending := execution.buildRound()
	if len(activeRound.Slots) == 0 {
		return execution.reportUnavailableMembers()
	}
	execution.startRound(activeRound, pending)
	return nil
}

func (s *RealtimeService) prepareRoomChat(ctx context.Context, request ChatRequest) (*roomChatExecution, error) {
	sessionKey, conversationID, err := s.validateChatRequest(request)
	if err != nil {
		return nil, err
	}
	ensureRoomChatIDs(&request)

	ctx, contextValue, err := s.internalConversationContext(ctx, conversationID, request.Internal)
	if err != nil {
		return nil, err
	}
	roomID := cmp.Or(strings.TrimSpace(request.RoomID), contextValue.Room.ID)
	attachments := s.normalizeChatAttachments(request.Attachments, request.AttachmentAgentID, roomID, conversationID)
	runtimeContent, err := s.renderRuntimeContentWithAttachments(ctx, request.Content, attachments)
	if err != nil {
		return nil, err
	}
	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	targetAgentIDs, targetResolution, err := resolveChatTargetAgentIDs(request, contextValue, agentNameByID)
	if err != nil {
		return nil, err
	}
	deliveryPolicy := protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy))
	if !request.Internal {
		targetAgentIDs, targetResolution = s.resolveActiveRoomTargets(
			sessionKey,
			conversationID,
			targetAgentIDs,
			targetResolution,
		)
	}
	targetAgentIDs, targetResolution = resolveDefaultRoomTargets(
		contextValue,
		agentNameByID,
		targetAgentIDs,
		targetResolution,
	)
	if len(targetAgentIDs) > 0 {
		if err = s.ensureQuotaAvailable(ctx); err != nil {
			return nil, err
		}
	}

	s.logAcceptedRoomChat(
		ctx,
		request,
		sessionKey,
		roomID,
		conversationID,
		attachments,
		targetAgentIDs,
		targetResolution,
	)
	history, err := s.roomHistory.ReadMessages(conversationID, nil)
	if err != nil {
		return nil, err
	}

	return &roomChatExecution{
		service:            s,
		ctx:                ctx,
		request:            request,
		sessionKey:         sessionKey,
		roomID:             roomID,
		conversationID:     conversationID,
		contextValue:       contextValue,
		attachments:        attachments,
		runtimeTriggerText: runtimeContent.PlainText(),
		agentNameByID:      agentNameByID,
		agentByID:          agentByID,
		targetAgentIDs:     targetAgentIDs,
		targetResolution:   targetResolution,
		deliveryPolicy:     deliveryPolicy,
		history:            history,
		userMessage:        newRoomUserMessage(request, sessionKey, roomID, conversationID, attachments, targetAgentIDs, deliveryPolicy),
	}, nil
}

func ensureRoomChatIDs(request *ChatRequest) {
	if strings.TrimSpace(request.RoundID) == "" {
		request.RoundID = protocol.NewRoundID()
	}
	if strings.TrimSpace(request.UserMessageID) == "" {
		request.UserMessageID = protocol.NewUserMessageID()
	}
}

func resolveDefaultRoomTargets(
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
	targetAgentIDs []string,
	targetResolution string,
) ([]string, string) {
	if len(targetAgentIDs) > 0 {
		return targetAgentIDs, targetResolution
	}
	if len(agentNameByID) == 1 {
		// 单成员 Room 与 DM 共享直聊直觉，不要求用户制造一次无意义的 @mention。
		for agentID := range agentNameByID {
			return []string{agentID}, "single_member_default"
		}
	}
	if hostAgentID, ok := resolveRoomHostDefaultTarget(contextValue, agentNameByID); ok {
		return []string{hostAgentID}, "room_host_default"
	}
	return targetAgentIDs, targetResolution
}

func (s *RealtimeService) logAcceptedRoomChat(
	ctx context.Context,
	request ChatRequest,
	sessionKey string,
	roomID string,
	conversationID string,
	attachments []protocol.ChatAttachment,
	targetAgentIDs []string,
	targetResolution string,
) {
	s.loggerFor(ctx).Info("受理 Room 会话消息",
		"session_key", sessionKey,
		"room_id", roomID,
		"conversation_id", conversationID,
		"round_id", request.RoundID,
		"target_agent_count", len(targetAgentIDs),
		"target_agents", slices.Clone(targetAgentIDs),
		"target_resolution", targetResolution,
		"content_chars", utf8.RuneCountInString(strings.TrimSpace(request.Content)),
		"content_preview", logx.PreviewText(request.Content, 240),
		"attachment_count", len(attachments),
	)
}

func newRoomUserMessage(
	request ChatRequest,
	sessionKey string,
	roomID string,
	conversationID string,
	attachments []protocol.ChatAttachment,
	targetAgentIDs []string,
	deliveryPolicy protocol.ChatDeliveryPolicy,
) protocol.Message {
	result := protocol.Message{
		"message_id":      request.UserMessageID,
		"session_key":     sessionKey,
		"room_id":         roomID,
		"conversation_id": conversationID,
		"agent_id":        "",
		"round_id":        request.RoundID,
		"role":            "user",
		"content":         strings.TrimSpace(request.Content),
		"timestamp":       time.Now().UnixMilli(),
		"delivery_policy": string(deliveryPolicy),
	}
	if len(targetAgentIDs) > 0 {
		result["target_agent_ids"] = slices.Clone(targetAgentIDs)
	}
	if len(attachments) > 0 {
		result["attachments"] = attachments
	}
	return result
}

func (e *roomChatExecution) persistInput() error {
	if !e.request.Internal || e.request.BroadcastUserMessage {
		if err := e.service.persistSharedInlineMessage(e.conversationID, e.userMessage); err != nil {
			return err
		}
		e.history = append(e.history, e.userMessage)
		e.service.broadcastSharedEvent(
			e.ctx,
			e.sessionKey,
			e.roomID,
			roomdomain.WrapMessageEvent(e.roomID, e.conversationID, e.userMessage, e.request.RoundID),
		)
	}
	if e.request.Internal {
		return nil
	}
	titleProvider, titleModel := resolveTitleRuntimeTarget(e.targetAgentIDs, e.agentByID)
	e.service.scheduleTitleGeneration(
		e.ctx,
		e.sessionKey,
		e.contextValue,
		strings.TrimSpace(e.request.Content),
		titleProvider,
		titleModel,
	)
	return nil
}

func (e *roomChatExecution) finishWithoutTarget() (bool, error) {
	if len(e.targetAgentIDs) > 0 {
		return false, nil
	}
	if e.request.Internal {
		return true, errors.New("room internal continuation has no target agent")
	}
	e.service.loggerFor(e.ctx).Warn("Room 消息未命中任何目标成员",
		"session_key", e.sessionKey,
		"room_id", e.roomID,
		"conversation_id", e.conversationID,
		"round_id", e.request.RoundID,
	)
	e.broadcastAck(nil)

	hintMessage := protocol.Message{
		"message_id":      "result_" + e.request.RoundID,
		"session_key":     e.sessionKey,
		"room_id":         e.roomID,
		"conversation_id": e.conversationID,
		"agent_id":        "",
		"round_id":        e.request.RoundID,
		"role":            "result",
		"subtype":         "success",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"result":          "请使用 @AgentName 指定要对话的成员",
		"is_error":        false,
		"timestamp":       time.Now().UnixMilli(),
	}
	if err := e.service.persistSharedInlineMessage(e.conversationID, hintMessage); err != nil {
		return true, err
	}
	e.service.broadcastSharedEvent(
		e.ctx,
		e.sessionKey,
		e.roomID,
		roomdomain.WrapMessageEvent(
			e.roomID,
			e.conversationID,
			message.ProjectResultMessage(nil, hintMessage),
			e.request.RoundID,
		),
	)
	e.service.broadcastSharedEvent(
		e.ctx,
		e.sessionKey,
		e.roomID,
		roomdomain.WrapRoundStatusEvent(e.sessionKey, e.roomID, e.conversationID, e.request.RoundID, "finished", "success"),
	)
	return true, nil
}

func (e *roomChatExecution) routeActiveSlots() (bool, error) {
	if e.request.Internal {
		return false, nil
	}

	var (
		handledAgentIDs map[string]struct{}
		err             error
	)
	switch e.deliveryPolicy {
	case protocol.ChatDeliveryPolicyQueue, protocol.ChatDeliveryPolicyAuto:
		handledAgentIDs, err = e.queueActiveSlots()
	case protocol.ChatDeliveryPolicyGuide:
		handledAgentIDs, err = e.guideActiveSlots()
		e.deliveryPolicy = protocol.ChatDeliveryPolicyQueue
	case protocol.ChatDeliveryPolicyInterrupt:
		err = e.service.interruptAgentSlots(
			e.ctx,
			e.sessionKey,
			e.targetAgentIDs,
			"收到新的用户消息，上一轮已停止",
			true,
		)
	}
	if err != nil {
		return true, err
	}
	if len(handledAgentIDs) == 0 {
		return false, nil
	}
	e.targetAgentIDs = filterHandledAgentIDs(e.targetAgentIDs, handledAgentIDs)
	if len(e.targetAgentIDs) > 0 {
		return false, nil
	}
	e.broadcastAck(nil)
	e.service.broadcastSessionStatus(e.ctx, e.sessionKey)
	return true, nil
}

func (e *roomChatExecution) queueActiveSlots() (map[string]struct{}, error) {
	handledAgentIDs, err := e.service.enqueueForActiveAgentSlots(
		e.ctx,
		e.sessionKey,
		e.roomID,
		e.conversationID,
		e.targetAgentIDs,
		strings.TrimSpace(e.request.Content),
		e.attachments,
		e.request.RoundID,
		authctx.OwnerUserID(e.ctx),
	)
	if err != nil || len(handledAgentIDs) == 0 {
		return handledAgentIDs, err
	}
	if err = e.service.broadcastRoomInputQueueSnapshot(e.ctx, e.sessionKey, e.contextValue); err != nil {
		return nil, err
	}
	return handledAgentIDs, nil
}

func (e *roomChatExecution) guideActiveSlots() (map[string]struct{}, error) {
	handledAgentIDs, err := e.service.guideActiveAgentSlots(
		e.ctx,
		e.sessionKey,
		e.roomID,
		e.conversationID,
		e.targetAgentIDs,
		strings.TrimSpace(e.request.Content),
		e.attachments,
		e.request.RoundID,
		authctx.OwnerUserID(e.ctx),
	)
	if err != nil || len(handledAgentIDs) == 0 {
		return handledAgentIDs, err
	}
	if err = e.service.broadcastRoomInputQueueSnapshot(e.ctx, e.sessionKey, e.contextValue); err != nil {
		return nil, err
	}
	return handledAgentIDs, nil
}

func (e *roomChatExecution) buildRound() (*activeRoomRound, []protocol.ChatAckPendingSlot) {
	sessionsByAgent := make(map[string]protocol.SessionRecord, len(e.contextValue.Sessions))
	for _, item := range e.contextValue.Sessions {
		sessionsByAgent[item.AgentID] = item
	}

	initialTrigger := roomTrigger{
		TriggerType: initialRoomTriggerType(e.request, e.targetResolution),
		Content:     strings.TrimSpace(e.request.Content),
		MessageID:   e.request.UserMessageID,
	}
	activeRound := &activeRoomRound{
		SessionKey:            e.sessionKey,
		RoomID:                e.roomID,
		ConversationID:        e.conversationID,
		RoomType:              e.contextValue.Room.RoomType,
		Context:               e.contextValue,
		RoundID:               e.request.RoundID,
		RootRoundID:           e.request.RoundID,
		OwnerUserID:           authctx.OwnerUserID(e.ctx),
		Internal:              e.request.Internal,
		InputOptions:          e.request.InputOptions,
		PermissionMode:        e.request.PermissionMode,
		PermissionHandler:     e.request.PermissionHandler,
		EventObserver:         e.request.EventObserver,
		GoalContext:           strings.TrimSpace(e.request.GoalContext),
		GoalID:                strings.TrimSpace(e.request.GoalID),
		GoalObjectiveRevision: e.request.GoalObjectiveRevision,
		Slots:                 make(map[string]*activeRoomSlot),
		Done:                  make(chan struct{}),
	}

	pending := make([]protocol.ChatAckPendingSlot, 0, len(e.targetAgentIDs))
	for index, agentID := range e.targetAgentIDs {
		sessionRecord, ok := sessionsByAgent[agentID]
		agentValue := e.agentByID[agentID]
		if !ok || agentValue == nil {
			continue
		}
		msgID := newRealtimeID()
		agentRoundID := protocol.NewAgentRoundID()
		slotTrigger := initialTrigger
		slotTrigger.TargetAgentID = agentID
		slot := &activeRoomSlot{
			RoomSessionID:      sessionRecord.ID,
			SDKSessionID:       strings.TrimSpace(sessionRecord.SDKSessionID),
			AgentID:            agentID,
			AgentRoundID:       agentRoundID,
			MsgID:              msgID,
			RuntimeSessionKey:  protocol.BuildRoomAgentSessionKey(e.conversationID, agentID, e.contextValue.Room.RoomType),
			WorkspacePath:      agentValue.WorkspacePath,
			Status:             "pending",
			Index:              index,
			TimestampMS:        normalizeInt64(e.userMessage["timestamp"]),
			Trigger:            slotTrigger,
			TriggerAttachments: e.attachments,
			Done:               make(chan struct{}),
		}
		if activeRound.GoalID != "" && activeRound.GoalObjectiveRevision > 0 {
			slot.GoalSessionKey = activeRound.SessionKey
			slot.GoalIDForUsage = activeRound.GoalID
			slot.ensureGoalObjectiveRevision(activeRound.GoalObjectiveRevision)
		}
		activeRound.Slots[msgID] = slot
		pending = append(pending, protocol.ChatAckPendingSlot{
			AgentID:      agentID,
			AgentRoundID: agentRoundID,
			MsgID:        msgID,
			Status:       "pending",
			Timestamp:    normalizeInt64(e.userMessage["timestamp"]),
			Index:        index,
		})
	}
	return activeRound, pending
}

func (e *roomChatExecution) reportUnavailableMembers() error {
	e.service.loggerFor(e.ctx).Warn("Room 中没有可用成员会话",
		"session_key", e.sessionKey,
		"room_id", e.roomID,
		"conversation_id", e.conversationID,
		"round_id", e.request.RoundID,
	)
	e.broadcastAck(nil)
	e.service.broadcastSharedEvent(
		e.ctx,
		e.sessionKey,
		e.roomID,
		roomdomain.NewErrorEvent(e.sessionKey, e.roomID, e.conversationID, "room_error", "Room 中没有可用成员会话", e.request.RoundID),
	)
	e.service.broadcastSharedEvent(
		e.ctx,
		e.sessionKey,
		e.roomID,
		roomdomain.WrapRoundStatusEvent(e.sessionKey, e.roomID, e.conversationID, e.request.RoundID, "error", "error"),
	)
	return nil
}

func (e *roomChatExecution) startRound(activeRound *activeRoomRound, pending []protocol.ChatAckPendingSlot) {
	roundCtx, cancel := context.WithCancel(context.Background())
	activeRound.Cancel = cancel
	e.service.registerRound(activeRound)
	e.service.runtime.StartRound(e.sessionKey, e.request.RoundID, cancel)

	e.service.broadcastSharedEvent(
		e.ctx,
		e.sessionKey,
		e.roomID,
		roomdomain.WrapRoundStatusEvent(e.sessionKey, e.roomID, e.conversationID, e.request.RoundID, "running", ""),
	)
	if shouldBroadcastRoomChatAck(e.request) {
		e.broadcastAck(pending)
	}
	e.service.broadcastSessionStatus(e.ctx, e.sessionKey)
	go e.service.runRound(roundCtx, activeRound, e.history, e.agentNameByID, e.agentByID)
}

func (e *roomChatExecution) broadcastAck(pending []protocol.ChatAckPendingSlot) {
	e.service.broadcastSharedEvent(e.ctx, e.sessionKey, e.roomID, roomdomain.WrapChatAckEvent(
		e.sessionKey,
		e.roomID,
		e.conversationID,
		e.request.ClientRequestID,
		e.request.ClientMessageID,
		e.request.RoundID,
		e.request.UserMessageID,
		pending,
	))
}
