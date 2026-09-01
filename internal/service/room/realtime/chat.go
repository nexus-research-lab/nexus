// INPUT: Room 用户输入、owner-scoped Slash 展开、内部触发与当前 round/queue 状态。
// OUTPUT: 保留共享消息原文、把 Slash 作为独立原子输入投递给 runtime 的串行 Room round。
// POS: Room 输入从受理到 runtime 启动的原子交接边界。
package realtime

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	conversationsvc "github.com/nexus-research-lab/nexus/internal/service/conversation"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// roomChatExecution 保存一次 Room 输入从受理到启动 round 的业务态。
type roomChatExecution struct {
	service            *Service
	ctx                context.Context
	request            ChatRequest
	sessionKey         string
	roomID             string
	conversationID     string
	contextValue       *protocol.ConversationContextAggregate
	attachments        []protocol.ChatAttachment
	runtimeTriggerText string
	atomicRuntimeInput string
	agentNameByID      map[string]string
	agentByID          map[string]*protocol.Agent
	targetAgentIDs     []string
	targetResolution   string
	deliveryPolicy     protocol.ChatDeliveryPolicy
	history            []protocol.Message
	userMessage        protocol.Message
}

func (s *Service) buildRuntimeAgentDirectory(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) (map[string]string, map[string]*protocol.Agent, error) {
	agentNameByID := make(map[string]string)
	agentByID := make(map[string]*protocol.Agent)
	if contextValue == nil {
		return agentNameByID, agentByID, nil
	}
	memberIDs := make([]string, 0, len(contextValue.Members))
	seenMemberIDs := make(map[string]struct{}, len(contextValue.Members))
	for _, member := range contextValue.Members {
		agentID := strings.TrimSpace(member.MemberAgentID)
		if member.MemberType != protocol.MemberTypeAgent || agentID == "" {
			continue
		}
		if _, duplicate := seenMemberIDs[agentID]; duplicate {
			continue
		}
		seenMemberIDs[agentID] = struct{}{}
		memberIDs = append(memberIDs, agentID)
	}
	if len(memberIDs) == 0 {
		return agentNameByID, agentByID, nil
	}
	// ConversationContext.MemberAgents 是 Room 展示读模型，不是 runtime 配置
	// 权威。每次准备 round 都以一个批量查询重新水合完整 Agent 配置，避免
	// Skill、permission 或未来 runtime 字段在 Room 副本中静默漂移。
	agents, err := s.agents.GetAgentsByIDs(ctx, memberIDs)
	if err != nil {
		return nil, nil, err
	}
	for index := range agents {
		agentValue := &agents[index]
		agentNameByID[agentValue.AgentID] = agentValue.Name
		agentByID[agentValue.AgentID] = agentValue
	}
	for _, agentID := range memberIDs {
		if _, ok := agentByID[agentID]; !ok {
			return nil, nil, fmt.Errorf("room member agent %q is not active", agentID)
		}
	}
	return agentNameByID, agentByID, nil
}

// buildMemberNameDirectory projects display identity only. It intentionally
// cannot supply permission, Skill or runtime options; those require
// buildRuntimeAgentDirectory and the canonical Agent service.
func buildMemberNameDirectory(contextValue *protocol.ConversationContextAggregate) map[string]string {
	result := make(map[string]string)
	if contextValue == nil {
		return result
	}
	memberIDs := make(map[string]struct{}, len(contextValue.Members))
	for _, member := range contextValue.Members {
		agentID := strings.TrimSpace(member.MemberAgentID)
		if member.MemberType == protocol.MemberTypeAgent && agentID != "" {
			memberIDs[agentID] = struct{}{}
			result[agentID] = agentID
		}
	}
	for _, agentValue := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(agentValue.AgentID)
		if _, ok := memberIDs[agentID]; !ok {
			continue
		}
		if name := strings.TrimSpace(agentValue.Name); name != "" {
			result[agentID] = name
		}
	}
	return result
}

func (s *Service) scheduleTitleGeneration(
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
func (s *Service) HandleChat(ctx context.Context, request ChatRequest) error {
	return s.handleChat(ctx, request)
}

// handleChat 负责获取 conversation 级派发闸门。
func (s *Service) handleChat(ctx context.Context, request ChatRequest) error {
	sessionKey, conversationID, err := s.validateChatRequest(request)
	if err != nil {
		return err
	}
	lease := s.lockRoomDispatch(sessionKey, conversationID)
	defer lease.Unlock()
	return s.handleChatLocked(ctx, request)
}

// handleChatLocked 在已经持有 conversation 派发闸门时执行输入交接。
func (s *Service) handleChatLocked(ctx context.Context, request ChatRequest) error {
	execution, err := s.prepareRoomChat(ctx, request)
	if err != nil {
		return err
	}
	if err = s.cancelActiveRoomGoalForUser(execution.ctx, execution.sessionKey, request.Content); err != nil {
		return err
	}
	if len(execution.targetAgentIDs) == 0 {
		if err = execution.persistInput(); err != nil {
			return err
		}
		_, handleErr := execution.finishWithoutTarget()
		return handleErr
	}
	if handled, routeErr := execution.routeActiveSlots(); handled {
		return routeErr
	}
	if err = execution.persistInput(); err != nil {
		return err
	}

	activeRound, pending := execution.buildRound()
	if len(activeRound.Slots) == 0 {
		return execution.reportUnavailableMembers()
	}
	if err = execution.startRound(activeRound, pending); err != nil {
		return err
	}
	return nil
}

func (s *Service) prepareRoomChat(ctx context.Context, request ChatRequest) (*roomChatExecution, error) {
	sessionKey, conversationID, err := s.validateChatRequest(request)
	if err != nil {
		return nil, err
	}
	ensureRoomChatIDs(&request)

	ctx, contextValue, err := s.internalConversationContext(ctx, conversationID, request.Internal)
	if err != nil {
		return nil, err
	}
	if err = requireGroupRoomContext(contextValue); err != nil {
		return nil, err
	}
	roomID := cmp.Or(strings.TrimSpace(request.RoomID), contextValue.Room.ID)
	attachments := s.normalizeChatAttachments(request.Attachments, request.AttachmentAgentID, roomID, conversationID)
	expandedRuntimeContent, err := s.expandRuntimeSlashPrompt(ctx, request.Content)
	if err != nil {
		return nil, err
	}
	runtimeTriggerText := expandedRuntimeContent
	atomicRuntimeInput := ""
	if conversationsvc.IsSlashCommandInput(request.Content) {
		runtimeTriggerText = strings.TrimSpace(request.Content)
		atomicRuntimeInput = expandedRuntimeContent
	}
	if _, err = s.renderRuntimeContentWithAttachments(ctx, expandedRuntimeContent, attachments); err != nil {
		return nil, err
	}
	agentNameByID, agentByID, err := s.buildRuntimeAgentDirectory(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	if err = s.reconcileRoomGoalLead(ctx, sessionKey, contextValue, agentNameByID); err != nil {
		return nil, err
	}
	targetAgentIDs, targetResolution, err := resolveChatTargetAgentIDs(request, contextValue, agentNameByID)
	if err != nil {
		return nil, err
	}
	targetAgentIDs, targetResolution = resolveDefaultRoomTargets(
		contextValue,
		agentNameByID,
		targetAgentIDs,
		targetResolution,
	)
	if request.Internal {
		_, pausedTargetAgentIDs := partitionRoomParticipationTargets(
			contextValue.Members,
			targetAgentIDs,
		)
		if len(pausedTargetAgentIDs) > 0 {
			return nil, errors.New("Room member participation is paused")
		}
	}
	admissionRound := &activeRoomRound{
		SessionKey:         sessionKey,
		RoomID:             roomID,
		ConversationID:     conversationID,
		CoordinatorAgentID: roomCoordinatorAgentID(request.CoordinatorAgentID, contextValue),
		Context:            contextValue,
		RootRoundID:        request.RoundID,
		OwnerUserID:        contextValue.Room.OwnerUserID,
	}
	for _, targetAgentID := range targetAgentIDs {
		if err = s.authorizeManagedExecutionTarget(
			ctx,
			admissionRound,
			targetAgentID,
			nil,
		); err != nil {
			return nil, err
		}
	}
	deliveryPolicy := safeRoomDeliveryPolicy(request)
	if !request.Internal {
		targetAgentIDs, targetResolution = s.resolveActiveRoomTargets(
			sessionKey,
			conversationID,
			targetAgentIDs,
			targetResolution,
		)
	}
	if len(targetAgentIDs) > 0 {
		if err = s.ensureQuotaAvailable(ctx); err != nil {
			if request.Internal && strings.TrimSpace(request.GoalID) != "" {
				s.recordGoalQuotaLimit(ctx, sessionKey, request.RoundID, err)
			}
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
	history, err := s.roomHistory.ReadMessages(contextValue.Room.OwnerUserID, conversationID, nil)
	if err != nil {
		return nil, err
	}

	userMessage := newRoomUserMessage(request, sessionKey, roomID, conversationID, attachments, targetAgentIDs, deliveryPolicy)
	annotateRoomUserMessage(contextValue, userMessage)
	return &roomChatExecution{
		service:            s,
		ctx:                ctx,
		request:            request,
		sessionKey:         sessionKey,
		roomID:             roomID,
		conversationID:     conversationID,
		contextValue:       contextValue,
		attachments:        attachments,
		runtimeTriggerText: runtimeTriggerText,
		atomicRuntimeInput: atomicRuntimeInput,
		agentNameByID:      agentNameByID,
		agentByID:          agentByID,
		targetAgentIDs:     targetAgentIDs,
		targetResolution:   targetResolution,
		deliveryPolicy:     deliveryPolicy,
		history:            history,
		userMessage:        userMessage,
	}, nil
}

func safeRoomDeliveryPolicy(request ChatRequest) protocol.ChatDeliveryPolicy {
	policy := protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy))
	if !request.TrustedConfigurationContext && policy == protocol.ChatDeliveryPolicyGuide {
		return protocol.ChatDeliveryPolicyQueue
	}
	return policy
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

func (s *Service) logAcceptedRoomChat(
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
	if request.Internal || request.InputOptions.HiddenFromUser {
		result["hidden_from_user"] = true
	}
	if request.Internal || request.InputOptions.Synthetic {
		result["is_synthetic"] = true
	}
	return result
}

func (e *roomChatExecution) persistInput() error {
	if !e.request.Internal || e.request.BroadcastUserMessage {
		if err := e.service.persistSharedInlineMessage(
			e.contextValue.Room.OwnerUserID,
			e.conversationID,
			e.userMessage,
		); err != nil {
			return err
		}
		if roomRequestHasCanonicalUserInput(e.request) {
			if err := e.service.markConversationStarted(
				e.ctx,
				e.conversationID,
				roomMessageActivityTime(e.userMessage),
			); err != nil {
				return err
			}
		}
		e.history = append(e.history, e.userMessage)
		realtimeUserMessage := protocol.Clone(e.userMessage)
		if clientMessageID := strings.TrimSpace(e.request.ClientMessageID); clientMessageID != "" {
			// client_message_id 只用于当前连接把 durable 广播原子替换到 optimistic
			// 位置；它不是历史消息身份，不能写入持久化记录。
			realtimeUserMessage["client_message_id"] = clientMessageID
		}
		e.service.broadcastSharedEvent(
			e.ctx,
			e.sessionKey,
			e.roomID,
			roomdomain.WrapMessageEvent(e.roomID, e.conversationID, realtimeUserMessage, e.request.RoundID),
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

func roomRequestHasCanonicalUserInput(request ChatRequest) bool {
	return !request.Internal &&
		!request.InputOptions.HiddenFromUser &&
		!request.InputOptions.Synthetic
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
	e.broadcastAck(nil, true)

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
	if err := e.service.persistSharedInlineMessage(
		e.contextValue.Room.OwnerUserID,
		e.conversationID,
		hintMessage,
	); err != nil {
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
	pausedAgentIDs, err := e.queuePausedTargets()
	if err != nil {
		return true, err
	}
	if len(pausedAgentIDs) > 0 {
		e.targetAgentIDs = filterHandledAgentIDs(
			e.targetAgentIDs,
			pausedAgentIDs,
		)
		if len(e.targetAgentIDs) == 0 {
			e.broadcastAck(nil, false)
			e.service.broadcastSessionStatus(e.ctx, e.sessionKey)
			return true, nil
		}
	}

	var (
		handledAgentIDs map[string]struct{}
		routeErr        error
	)
	switch e.deliveryPolicy {
	case protocol.ChatDeliveryPolicyQueue, protocol.ChatDeliveryPolicyAuto:
		handledAgentIDs, routeErr = e.queueActiveSlots()
	case protocol.ChatDeliveryPolicyGuide:
		handledAgentIDs, routeErr = e.guideActiveSlots()
		// 多目标分散在不同 root，或只有部分目标忙碌时，不能把同一条
		// public message 注入多个 root。忙碌目标各自排队，空闲目标在
		// 后续 buildRound 中立即启动。
		if routeErr == nil && len(handledAgentIDs) == 0 {
			handledAgentIDs, routeErr = e.queueActiveSlots()
		}
		e.deliveryPolicy = protocol.ChatDeliveryPolicyQueue
	case protocol.ChatDeliveryPolicyInterrupt:
		routeErr = e.service.interruptAgentSlots(
			e.ctx,
			e.sessionKey,
			e.targetAgentIDs,
			"收到新的用户消息，上一轮已停止",
			true,
		)
	}
	if routeErr != nil {
		return true, routeErr
	}
	if len(handledAgentIDs) == 0 {
		return false, nil
	}
	e.targetAgentIDs = filterHandledAgentIDs(e.targetAgentIDs, handledAgentIDs)
	if len(e.targetAgentIDs) > 0 {
		return false, nil
	}
	e.broadcastAck(nil, false)
	e.service.broadcastSessionStatus(e.ctx, e.sessionKey)
	return true, nil
}

func (e *roomChatExecution) queuePausedTargets() (map[string]struct{}, error) {
	queuedAgentIDs, err := e.service.enqueueForPausedAgentTargets(
		e.ctx,
		e.contextValue,
		e.targetAgentIDs,
		strings.TrimSpace(e.request.Content),
		e.attachments,
		e.request.RoundID,
		e.request.UserMessageID,
		authctx.OwnerUserID(e.ctx),
	)
	if err != nil || len(queuedAgentIDs) == 0 {
		return queuedAgentIDs, err
	}
	if err = e.service.broadcastRoomInputQueueSnapshot(
		e.ctx,
		e.sessionKey,
		e.contextValue,
	); err != nil {
		return nil, err
	}
	return queuedAgentIDs, nil
}

func (e *roomChatExecution) queueActiveSlots() (map[string]struct{}, error) {
	handledAgentIDs, err := e.service.enqueueForActiveAgentSlotsWithTrust(
		e.ctx,
		e.sessionKey,
		e.roomID,
		e.conversationID,
		e.targetAgentIDs,
		strings.TrimSpace(e.request.Content),
		e.attachments,
		e.request.RoundID,
		e.request.UserMessageID,
		authctx.OwnerUserID(e.ctx),
		e.request.TrustedConfigurationContext,
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
		protocol.InputQueueItem{
			ID:              e.request.RoundID,
			SourceMessageID: e.request.UserMessageID,
			Source:          protocol.InputQueueSourceUser,
			Content:         strings.TrimSpace(e.request.Content),
			Attachments:     e.attachments,
			OwnerUserID:     authctx.OwnerUserID(e.ctx),
		},
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
		Content:     strings.TrimSpace(e.runtimeTriggerText),
		MessageID:   e.request.UserMessageID,
	}
	activeRound := &activeRoomRound{
		SessionKey:                        e.sessionKey,
		RoomID:                            e.roomID,
		ConversationID:                    e.conversationID,
		CoordinatorAgentID:                roomCoordinatorAgentID(e.request.CoordinatorAgentID, e.contextValue),
		RoomType:                          e.contextValue.Room.RoomType,
		Context:                           e.contextValue,
		RoundID:                           e.request.RoundID,
		RootRoundID:                       e.request.RoundID,
		OwnerUserID:                       authctx.OwnerUserID(e.ctx),
		Internal:                          e.request.Internal,
		AuthorityEpoch:                    e.contextValue.Room.AuthorityEpoch,
		TrustedConfigurationContext:       e.request.TrustedConfigurationContext,
		ExecutionOrigin:                   strings.TrimSpace(e.request.ExecutionOrigin),
		trustedQueuedConfigurationContext: e.request.trustedQueuedConfigurationContext,
		InputOptions:                      e.request.InputOptions,
		PermissionMode:                    e.request.PermissionMode,
		PermissionHandler:                 e.request.PermissionHandler,
		RuntimeToolPolicy:                 cloneRuntimeToolPolicy(e.request.RuntimeToolPolicy),
		AutomationRun:                     cloneAutomationRunContext(e.request.AutomationRun),
		EventObserver:                     e.request.EventObserver,
		GoalContext:                       strings.TrimSpace(e.request.GoalContext),
		GoalID:                            strings.TrimSpace(e.request.GoalID),
		GoalObjectiveRevision:             e.request.GoalObjectiveRevision,
		ExecutionID:                       strings.TrimSpace(e.request.ExecutionID),
		Slots:                             make(map[string]*activeRoomSlot),
		Done:                              make(chan struct{}),
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
			RoomSessionID:         sessionRecord.ID,
			OwnerUserID:           activeRound.OwnerUserID,
			AgentID:               agentID,
			AgentRoundID:          agentRoundID,
			GoalUsageScopeRoundID: roomRootRoundID(activeRound),
			MsgID:                 msgID,
			RuntimeSessionKey:     protocol.BuildRoomAgentSessionKey(e.conversationID, agentID, e.contextValue.Room.RoomType),
			WorkspacePath:         agentValue.WorkspacePath,
			Index:                 index,
			TimestampMS:           normalizeInt64(e.userMessage["timestamp"]),
			HiddenFromUser:        activeRound.Internal || activeRound.InputOptions.HiddenFromUser,
			Trigger:               slotTrigger,
			TriggerAttachments:    e.attachments,
			AtomicRuntimeInput:    strings.TrimSpace(e.atomicRuntimeInput),
		}
		slot.setSDKSessionID(strings.TrimSpace(sessionRecord.SDKSessionID))
		slot.setStatus("pending")
		slot.doneChannel()
		if activeRound.GoalID != "" && activeRound.GoalObjectiveRevision > 0 {
			slot.setGoalBinding(activeRound.SessionKey, activeRound.GoalID)
			slot.ensureGoalObjectiveRevision(activeRound.GoalObjectiveRevision)
		}
		activeRound.Slots[msgID] = slot
		pending = append(pending, protocol.ChatAckPendingSlot{
			AgentID:        agentID,
			AgentRoundID:   agentRoundID,
			MsgID:          msgID,
			RoundID:        roomRootRoundID(activeRound),
			HiddenFromUser: roomSlotHiddenFromUser(slot),
			Source:         slot.QueueSource,
			Status:         "pending",
			Timestamp:      normalizeInt64(e.userMessage["timestamp"]),
			Index:          index,
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
	e.broadcastAck(nil, true)
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
		roomdomain.WrapRoundStatusErrorEvent(
			e.sessionKey,
			e.roomID,
			e.conversationID,
			e.request.RoundID,
			"Room 中没有可用成员会话",
		),
	)
	return nil
}

func (e *roomChatExecution) startRound(activeRound *activeRoomRound, pending []protocol.ChatAckPendingSlot) error {
	roundCtx, cancel := context.WithCancel(context.WithoutCancel(e.ctx))
	activeRound.Cancel = cancel
	e.service.registerRound(activeRound)
	if err := e.service.runtime.StartRound(roundCtx, e.sessionKey, e.request.RoundID, cancel); err != nil {
		e.service.finishRound(activeRound)
		return err
	}
	if e.request.continuationStartAdmission != nil {
		goalID := strings.TrimSpace(e.request.GoalID)
		if goalID != "" {
			for _, slot := range activeRound.Slots {
				if slot == nil {
					continue
				}
				e.service.runtime.RegisterGoalAccountingIdentity(
					e.sessionKey,
					slot.AgentRoundID,
					func() string { return goalID },
				)
			}
		}
		if err := e.request.continuationStartAdmission(e.ctx); err != nil {
			cancel()
			for _, slot := range activeRound.Slots {
				if slot == nil {
					continue
				}
				e.service.runtime.RegisterGoalAccountingIdentity(e.sessionKey, slot.AgentRoundID, nil)
				slot.closeDone()
			}
			e.service.runtime.MarkRoundFinished(e.sessionKey, e.request.RoundID)
			e.service.rounds.unregister(activeRound)
			activeRound.doneOnce.Do(func() { close(activeRound.Done) })
			return err
		}
	}

	e.service.broadcastSharedEvent(
		e.ctx,
		e.sessionKey,
		e.roomID,
		roomdomain.WrapRoundStatusEvent(e.sessionKey, e.roomID, e.conversationID, e.request.RoundID, "running", ""),
	)
	if shouldBroadcastRoomChatAck(e.request) {
		e.broadcastAck(pending, true)
	}
	e.service.broadcastSessionStatus(e.ctx, e.sessionKey)
	go e.service.runRound(roundCtx, activeRound, e.history, e.agentNameByID, e.agentByID)
	return nil
}

func (e *roomChatExecution) broadcastAck(pending []protocol.ChatAckPendingSlot, userMessageCommitted bool) {
	e.service.broadcastSharedEvent(e.ctx, e.sessionKey, e.roomID, roomdomain.WrapChatAckEvent(
		e.sessionKey,
		e.roomID,
		e.conversationID,
		e.request.ClientRequestID,
		e.request.ClientMessageID,
		e.request.RoundID,
		e.request.UserMessageID,
		userMessageCommitted,
		pending,
	))
}

// INPUT: Room 请求里的显式目标、@mention、默认投递策略与当前活跃 round。
// OUTPUT: 保持显式/房主路由优先，并为仍无目标的 follow-up 选择最近活跃 root round。
// POS: Room 用户输入目标解析的唯一真相源；目标 Agent 的 slot 状态决定立即启动或排队。
const activeRoundDefaultTargetResolution = "active_round_default"

func (s *Service) resolveActiveRoomTargets(
	sessionKey string,
	conversationID string,
	targetAgentIDs []string,
	targetResolution string,
) ([]string, string) {
	if len(targetAgentIDs) > 0 {
		return targetAgentIDs, targetResolution
	}
	activeAgentIDs := s.latestActiveRootRoundAgentIDs(sessionKey, conversationID)
	if len(activeAgentIDs) == 0 {
		return targetAgentIDs, targetResolution
	}
	return activeAgentIDs, activeRoundDefaultTargetResolution
}

type activeRoomTargetSlot struct {
	agentID      string
	agentRoundID string
	timestampMS  int64
	index        int
}

func (s *Service) latestActiveRootRoundAgentIDs(sessionKey string, conversationID string) []string {
	sessionKey = strings.TrimSpace(sessionKey)
	conversationID = strings.TrimSpace(conversationID)

	slotsByRoot := make(map[string][]activeRoomTargetSlot)
	latestTimestampByRoot := make(map[string]int64)
	latestSequenceByRoot := make(map[string]uint64)
	for _, roundValue := range s.rounds.snapshotConversation(conversationID) {
		if roundValue == nil ||
			roundValue.SessionKey != sessionKey ||
			roundValue.ConversationID != conversationID {
			continue
		}
		rootRoundID := roomRoundIdentity(roundValue)
		if rootRoundID == "" {
			continue
		}
		if roundValue.registrationSequence > latestSequenceByRoot[rootRoundID] {
			latestSequenceByRoot[rootRoundID] = roundValue.registrationSequence
		}
		for _, slot := range roundValue.Slots {
			if !isActiveDeliverySlot(slot) {
				continue
			}
			slotsByRoot[rootRoundID] = append(slotsByRoot[rootRoundID], activeRoomTargetSlot{
				agentID:      strings.TrimSpace(slot.AgentID),
				agentRoundID: strings.TrimSpace(slot.AgentRoundID),
				timestampMS:  slot.TimestampMS,
				index:        slot.Index,
			})
			if slot.TimestampMS > latestTimestampByRoot[rootRoundID] {
				latestTimestampByRoot[rootRoundID] = slot.TimestampMS
			}
		}
	}

	selectedRoot := ""
	var selectedTimestamp int64
	var selectedSequence uint64
	for rootRoundID, slots := range slotsByRoot {
		if len(slots) == 0 {
			continue
		}
		sequence := latestSequenceByRoot[rootRoundID]
		timestamp := latestTimestampByRoot[rootRoundID]
		if selectedRoot == "" || sequence > selectedSequence ||
			(sequence == selectedSequence && timestamp > selectedTimestamp) ||
			(sequence == selectedSequence && timestamp == selectedTimestamp && rootRoundID < selectedRoot) {
			selectedRoot = rootRoundID
			selectedSequence = sequence
			selectedTimestamp = timestamp
		}
	}
	selected := slotsByRoot[selectedRoot]
	sort.Slice(selected, func(i int, j int) bool {
		if selected[i].timestampMS != selected[j].timestampMS {
			return selected[i].timestampMS < selected[j].timestampMS
		}
		if selected[i].index != selected[j].index {
			return selected[i].index < selected[j].index
		}
		if selected[i].agentID != selected[j].agentID {
			return selected[i].agentID < selected[j].agentID
		}
		return selected[i].agentRoundID < selected[j].agentRoundID
	})
	result := make([]string, 0, len(selected))
	seen := make(map[string]struct{}, len(selected))
	for _, slot := range selected {
		if slot.agentID == "" {
			continue
		}
		if _, ok := seen[slot.agentID]; ok {
			continue
		}
		seen[slot.agentID] = struct{}{}
		result = append(result, slot.agentID)
	}
	return result
}

func resolveRoomHostDefaultTarget(
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
) (string, bool) {
	if contextValue == nil || !contextValue.Room.HostAutoReplyEnabled {
		return "", false
	}
	hostAgentID := strings.TrimSpace(contextValue.Room.HostAgentID)
	if hostAgentID == "" {
		return "", false
	}
	if _, ok := agentNameByID[hostAgentID]; !ok {
		return "", false
	}
	return hostAgentID, true
}

func initialRoomTriggerType(request ChatRequest, targetResolution string) string {
	if request.Internal && strings.TrimSpace(request.InputOptions.Purpose) == "goal_continuation" {
		return "goal_continuation"
	}
	if targetResolution == "room_host_default" {
		return "room_host_default"
	}
	return "public_chat"
}

func shouldBroadcastRoomChatAck(request ChatRequest) bool {
	if !request.Internal {
		return true
	}
	return strings.TrimSpace(request.InputOptions.Purpose) == "goal_continuation"
}

func (s *Service) validateChatRequest(request ChatRequest) (string, string, error) {
	// durable user queue 只承载真实用户输入；隐藏或 synthetic 消息必须走
	// internal 路径，避免排队后丢失来源语义并误消费 conversation draft。
	if !request.Internal && !roomRequestHasCanonicalUserInput(request) {
		return "", "", errors.New("hidden or synthetic input must be internal")
	}
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", "", err
	}
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return "", "", errors.New("session_key must be room shared key")
	}
	if !protocol.HasChatInput(request.Content, request.Attachments) &&
		!(request.Internal && strings.TrimSpace(request.GoalContext) != "") {
		return "", "", errors.New("content is required")
	}
	if request.Internal &&
		strings.TrimSpace(request.InputOptions.Purpose) == "goal_continuation" &&
		(strings.TrimSpace(request.GoalID) == "" ||
			request.GoalObjectiveRevision <= 0) {
		return "", "", errors.New(
			"goal continuation requires exact goal and objective revision",
		)
	}
	conversationID := cmp.Or(strings.TrimSpace(request.ConversationID), protocol.ParseRoomConversationID(sessionKey))
	if conversationID == "" {
		return "", "", errors.New("conversation_id is required")
	}
	return sessionKey, conversationID, nil
}

func resolveChatTargetAgentIDs(
	request ChatRequest,
	contextValue *protocol.ConversationContextAggregate,
	agentNameByID map[string]string,
) ([]string, string, error) {
	if len(request.TargetAgentIDs) > 0 {
		targetAgentIDs := normalizeExplicitTargetAgentIDs(request.TargetAgentIDs)
		if len(targetAgentIDs) == 0 {
			return nil, "", errors.New("target_agent_ids must not be empty")
		}
		for _, agentID := range targetAgentIDs {
			if !roomdomain.IsMemberAgent(contextValue.Members, agentID) {
				return nil, "", fmt.Errorf("target_agent_id is not a room member: %s", agentID)
			}
		}
		return targetAgentIDs, "explicit_target", nil
	}
	targetAgentIDs := roomdomain.ResolveMentionAgentIDs(request.Content, reverseAgentNames(agentNameByID))
	return targetAgentIDs, roomTargetResolution(targetAgentIDs), nil
}

func normalizeExplicitTargetAgentIDs(values []string) []string {
	return normalizeRoomAgentIDs(values)
}

func (s *Service) persistSharedInlineMessage(
	ownerUserID string,
	conversationID string,
	message protocol.Message,
) error {
	if err := s.roomHistory.AppendInlineMessage(ownerUserID, conversationID, message); err != nil {
		return err
	}
	s.touchSharedConversationActivity(context.Background(), conversationID, roomMessageActivityTime(message))
	return nil
}

func (s *Service) persistSharedDurableMessage(
	ownerUserID string,
	conversationID string,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	if slot == nil || !protocol.IsTranscriptNativeMessage(protocol.Message(message)) {
		return s.persistSharedInlineMessage(ownerUserID, conversationID, message)
	}
	if err := s.roomHistory.AppendTranscriptReference(
		ownerUserID,
		conversationID,
		slot.WorkspacePath,
		slot.RuntimeSessionKey,
		message,
	); err != nil {
		return err
	}
	s.touchSharedConversationActivity(context.Background(), conversationID, roomMessageActivityTime(message))
	return nil
}

func (s *Service) touchSharedConversationActivity(ctx context.Context, conversationID string, activityAt time.Time) {
	if s == nil || s.rooms == nil {
		return
	}
	if activityAt.IsZero() {
		activityAt = time.Now().UTC()
	}
	if err := s.rooms.TouchConversationActivity(ctx, conversationID, activityAt); err != nil {
		s.loggerFor(ctx).Error("更新 Room conversation 活动时间失败",
			"conversation_id", conversationID,
			"activity_at", activityAt,
			"err", err,
		)
	}
}

func (s *Service) markConversationStarted(
	ctx context.Context,
	conversationID string,
	activityAt time.Time,
) error {
	if s == nil || s.rooms == nil {
		return nil
	}
	if activityAt.IsZero() {
		activityAt = time.Now().UTC()
	}
	return s.rooms.MarkConversationStarted(ctx, conversationID, activityAt)
}

func roomMessageActivityTime(message protocol.Message) time.Time {
	if len(message) == 0 {
		return time.Now().UTC()
	}
	return roomTimestampActivityTime(message["timestamp"])
}

func roomTimestampActivityTime(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC()
	case json.Number:
		return roomUnixMilliActivityTime(typed.String())
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" {
			return time.Now().UTC()
		}
		if parsed, err := time.Parse(time.RFC3339Nano, normalized); err == nil {
			return parsed.UTC()
		}
		if parsed, err := time.Parse(time.RFC3339, normalized); err == nil {
			return parsed.UTC()
		}
		return roomUnixMilliActivityTime(normalized)
	case int:
		return time.UnixMilli(int64(typed)).UTC()
	case int64:
		return time.UnixMilli(typed).UTC()
	case int32:
		return time.UnixMilli(int64(typed)).UTC()
	case float64:
		return time.UnixMilli(int64(typed)).UTC()
	case float32:
		return time.UnixMilli(int64(typed)).UTC()
	default:
		return time.Now().UTC()
	}
}

func roomUnixMilliActivityTime(value string) time.Time {
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.UnixMilli(parsed).UTC()
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return time.UnixMilli(int64(parsed)).UTC()
	}
	return time.Now().UTC()
}

// INPUT: Room 消息目标、活跃 round slot 与持久化输入队列。
// OUTPUT: queue 按 Agent 独立投递到最新 slot；guide 只原子投递到同一 active root。
// POS: Room 活跃执行目标解析与输入登记的数据面。
// newActiveSlotQueueEntry 把 slot 的运行时位置投影为统一的 Room 队列项。
// queue 与 guide 只负责填写来源差异，不能各自复制一套位置和目标字段。
func newActiveSlotQueueEntry(
	slot *activeRoomSlot,
	ownerUserID string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
) workspacestore.InputQueueEnqueue {
	item.Scope = protocol.InputQueueScopeRoom
	item.SessionKey = slot.RuntimeSessionKey
	item.RoomID = roomID
	item.ConversationID = conversationID
	item.AgentID = slot.AgentID
	item.TargetAgentIDs = []string{slot.AgentID}
	item.Attachments = protocol.NormalizeChatAttachments(item.Attachments, slot.AgentID)
	return workspacestore.InputQueueEnqueue{
		Location: workspacestore.InputQueueLocation{
			OwnerUserID:    strings.TrimSpace(ownerUserID),
			Scope:          protocol.InputQueueScopeRoom,
			WorkspacePath:  slot.WorkspacePath,
			SessionKey:     slot.RuntimeSessionKey,
			RoomID:         roomID,
			ConversationID: conversationID,
		},
		Item: item,
	}
}

func (s *Service) enqueueForActiveAgentSlots(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	targetAgentIDs []string,
	content string,
	attachments []protocol.ChatAttachment,
	roundID string,
	userMessageID string,
	ownerUserID string,
) (map[string]struct{}, error) {
	return s.enqueueForActiveAgentSlotsWithTrust(
		ctx,
		sessionKey,
		roomID,
		conversationID,
		targetAgentIDs,
		content,
		attachments,
		roundID,
		userMessageID,
		ownerUserID,
		false,
	)
}

func (s *Service) enqueueForActiveAgentSlotsWithTrust(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	targetAgentIDs []string,
	content string,
	attachments []protocol.ChatAttachment,
	roundID string,
	userMessageID string,
	ownerUserID string,
	trustedConfigurationContext bool,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findActiveDeliverySlotsByAgent(sessionKey, conversationID, targetAgentIDs)
	queuedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(slotsByAgentID))
	for _, agentID := range slices.Sorted(maps.Keys(slotsByAgentID)) {
		slot := slotsByAgentID[agentID]
		if slot == nil {
			continue
		}
		entries = append(entries, newActiveSlotQueueEntry(slot, ownerUserID, roomID, conversationID, protocol.InputQueueItem{
			ID:              strings.TrimSpace(roundID),
			SourceMessageID: strings.TrimSpace(userMessageID),
			Source:          protocol.InputQueueSourceUser,
			Content:         strings.TrimSpace(content),
			Attachments:     attachments,
			DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
			OwnerUserID:     strings.TrimSpace(ownerUserID),
			RootRoundID:     strings.TrimSpace(roundID),
		}))
	}
	committedItems, err := s.inputQueue.EnqueueBatchWithItems(entries)
	if err != nil {
		return queuedAgentIDs, err
	}
	if err = s.recordTrustedRoomQueueAdmissions(
		ctx,
		entries,
		committedItems,
		trustedConfigurationContext,
	); err != nil {
		s.rollbackRoomQueueAdmissions(ctx, entries, committedItems)
		return queuedAgentIDs, err
	}
	for _, entry := range entries {
		agentID := entry.Item.AgentID
		slot := slotsByAgentID[agentID]
		queuedAgentIDs[agentID] = struct{}{}
		s.loggerFor(ctx).Info("Room 公区消息写入目标 agent 待处理队列",
			"session_key", sessionKey,
			"conversation_id", conversationID,
			"agent_id", agentID,
			"round_id", roundID,
			"active_round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"content_chars", utf8.RuneCountInString(strings.TrimSpace(content)),
			"content_preview", logx.PreviewText(content, 240),
		)
	}
	return queuedAgentIDs, nil
}

func (s *Service) enqueueForPausedAgentTargets(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	targetAgentIDs []string,
	content string,
	attachments []protocol.ChatAttachment,
	roundID string,
	userMessageID string,
	ownerUserID string,
) (map[string]struct{}, error) {
	queuedAgentIDs := make(map[string]struct{})
	if contextValue == nil {
		return queuedAgentIDs, nil
	}
	_, pausedAgentIDs := partitionRoomParticipationTargets(
		contextValue.Members,
		targetAgentIDs,
	)
	if len(pausedAgentIDs) == 0 {
		return queuedAgentIDs, nil
	}
	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(pausedAgentIDs))
	for _, agentID := range pausedAgentIDs {
		location, ok := locationsByAgentID[agentID]
		if !ok {
			continue
		}
		entries = append(entries, workspacestore.InputQueueEnqueue{
			Location: location.Location,
			Item: protocol.InputQueueItem{
				ID:              strings.TrimSpace(roundID),
				Scope:           protocol.InputQueueScopeRoom,
				SessionKey:      location.Location.SessionKey,
				RoomID:          contextValue.Room.ID,
				ConversationID:  contextValue.Conversation.ID,
				AgentID:         agentID,
				TargetAgentIDs:  []string{agentID},
				SourceMessageID: strings.TrimSpace(userMessageID),
				Source:          protocol.InputQueueSourceUser,
				Content:         strings.TrimSpace(content),
				Attachments:     protocol.NormalizeChatAttachments(attachments, agentID),
				DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
				OwnerUserID:     strings.TrimSpace(ownerUserID),
				RootRoundID:     strings.TrimSpace(roundID),
			},
		})
	}
	if err = s.inputQueue.EnqueueBatch(entries); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		queuedAgentIDs[entry.Item.AgentID] = struct{}{}
		s.loggerFor(ctx).Info(
			"Room 成员暂停参与，用户输入保留在目标队列",
			"conversation_id", contextValue.Conversation.ID,
			"agent_id", entry.Item.AgentID,
			"round_id", roundID,
		)
	}
	return queuedAgentIDs, nil
}

// findActiveDeliverySlotsByAgent 为每个目标独立选择最新活跃 slot。它用于
// queue/空闲判断：一个目标忙碌不能让同一条多目标输入把它再次启动，也不能
// 阻止其他空闲目标立即开始。
func (s *Service) findActiveDeliverySlotsByAgent(
	sessionKey string,
	conversationID string,
	targetAgentIDs []string,
) map[string]*activeRoomSlot {
	targets := make(map[string]struct{}, len(targetAgentIDs))
	for _, agentID := range targetAgentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" {
			targets[agentID] = struct{}{}
		}
	}
	result := make(map[string]*activeRoomSlot, len(targets))
	if len(targets) == 0 {
		return result
	}

	for _, roundValue := range s.rounds.snapshotConversation(conversationID) {
		if roundValue == nil ||
			roundValue.SessionKey != sessionKey ||
			roundValue.ConversationID != conversationID {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || !isActiveDeliverySlot(slot) {
				continue
			}
			if _, ok := targets[slot.AgentID]; !ok {
				continue
			}
			current := result[slot.AgentID]
			if current == nil || slot.TimestampMS > current.TimestampMS ||
				(slot.TimestampMS == current.TimestampMS && slot.AgentRoundID < current.AgentRoundID) {
				result[slot.AgentID] = slot
			}
		}
	}
	return result
}

func (s *Service) findActiveDeliverySlots(
	sessionKey string,
	conversationID string,
	targetAgentIDs []string,
) map[string]*activeRoomSlot {
	targets := make(map[string]struct{}, len(targetAgentIDs))
	for _, agentID := range targetAgentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" {
			targets[agentID] = struct{}{}
		}
	}
	if len(targets) == 0 {
		return map[string]*activeRoomSlot{}
	}

	// guide 会把用户消息挂到正在流式输出的 root；多目标只有同属一个
	// active root 才能原子注入，避免同一 public message 被不同 root 反复改写。
	slotsByRoot := make(map[string]map[string]*activeRoomSlot)
	latestTimestampByRoot := make(map[string]int64)
	for _, roundValue := range s.rounds.snapshotConversation(conversationID) {
		if roundValue == nil ||
			roundValue.SessionKey != sessionKey ||
			roundValue.ConversationID != conversationID {
			continue
		}
		rootRoundID := roomRoundIdentity(roundValue)
		for _, slot := range roundValue.Slots {
			if slot == nil || !isActiveDeliverySlot(slot) {
				continue
			}
			if _, ok := targets[slot.AgentID]; !ok {
				continue
			}
			slots := slotsByRoot[rootRoundID]
			if slots == nil {
				slots = make(map[string]*activeRoomSlot, len(targets))
				slotsByRoot[rootRoundID] = slots
			}
			current := slots[slot.AgentID]
			if current == nil || slot.TimestampMS > current.TimestampMS {
				slots[slot.AgentID] = slot
			}
			if slot.TimestampMS > latestTimestampByRoot[rootRoundID] {
				latestTimestampByRoot[rootRoundID] = slot.TimestampMS
			}
		}
	}

	selectedRoot := ""
	var selectedTimestamp int64
	for rootRoundID, slots := range slotsByRoot {
		if len(slots) != len(targets) {
			continue
		}
		timestamp := latestTimestampByRoot[rootRoundID]
		if selectedRoot == "" || timestamp > selectedTimestamp ||
			(timestamp == selectedTimestamp && rootRoundID < selectedRoot) {
			selectedRoot = rootRoundID
			selectedTimestamp = timestamp
		}
	}
	if selectedRoot == "" {
		return map[string]*activeRoomSlot{}
	}
	return slotsByRoot[selectedRoot]
}

func isActiveDeliverySlot(slot *activeRoomSlot) bool {
	if slot == nil {
		return false
	}
	switch slot.getStatus() {
	case "finished", "error", "cancelled":
		return false
	default:
		return true
	}
}

func filterHandledAgentIDs(agentIDs []string, handled map[string]struct{}) []string {
	if len(handled) == 0 {
		return agentIDs
	}
	result := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		if _, ok := handled[agentID]; ok {
			continue
		}
		result = append(result, agentID)
	}
	return result
}

func (s *Service) guideActiveAgentSlots(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	targetAgentIDs []string,
	sourceItem protocol.InputQueueItem,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findActiveDeliverySlots(sessionKey, conversationID, targetAgentIDs)
	guidedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(slotsByAgentID))
	for _, agentID := range slices.Sorted(maps.Keys(slotsByAgentID)) {
		slot := slotsByAgentID[agentID]
		if slot == nil {
			continue
		}
		entries = append(entries, newActiveSlotQueueEntry(slot, sourceItem.OwnerUserID, roomID, conversationID, protocol.InputQueueItem{
			ID:              strings.TrimSpace(sourceItem.ID),
			SourceAgentID:   strings.TrimSpace(sourceItem.SourceAgentID),
			SourceMessageID: strings.TrimSpace(sourceItem.SourceMessageID),
			HandoffID:       strings.TrimSpace(sourceItem.HandoffID),
			Source:          protocol.NormalizeInputQueueSource(string(sourceItem.Source)),
			Content:         strings.TrimSpace(sourceItem.Content),
			Attachments:     sourceItem.Attachments,
			DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
			ReplyRoute:      sourceItem.ReplyRoute,
			OwnerUserID:     strings.TrimSpace(sourceItem.OwnerUserID),
			RootRoundID:     slot.AgentRoundID,
			HopIndex:        sourceItem.HopIndex,
		}))
	}
	if err := s.inputQueue.EnqueueBatch(entries); err != nil {
		return guidedAgentIDs, err
	}
	for _, entry := range entries {
		agentID := entry.Item.AgentID
		slot := slotsByAgentID[agentID]
		guidedAgentIDs[agentID] = struct{}{}
		s.loggerFor(ctx).Info("持久化 Room 引导消息等待 PostToolUse 注入",
			"session_key", sessionKey,
			"room_id", roomID,
			"runtime_session_key", slot.RuntimeSessionKey,
			"conversation_id", conversationID,
			"agent_id", agentID,
			"round_id", sourceItem.ID,
			"active_round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"content_chars", utf8.RuneCountInString(strings.TrimSpace(sourceItem.Content)),
			"content_preview", logx.PreviewText(sourceItem.Content, 240),
		)
	}
	return guidedAgentIDs, nil
}
