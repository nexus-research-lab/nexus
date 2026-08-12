// INPUT: Room 成员的定向消息请求、reply route 与实时 Room 权限聚合。
// OUTPUT: 经私聊开关、成员和 authority epoch 校验的私域记录与唤醒。
// POS: Room 私域消息创建、回复投影和唤醒编排边界。
package realtime

import (
	"context"
	"errors"
	"fmt"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const roomDirectedMessageMaxDelaySeconds = 86400
const roomReplyRouteMaxDepth = 4

// HandleDirectedMessage 处理 Room 内部私域 directed message。
func (s *Service) HandleDirectedMessage(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	contextValue, err := s.resolveDirectedMessageContext(ctx, roomID, conversationID)
	if err != nil {
		return nil, err
	}
	message, err := s.buildRoomDirectedMessageRecord(contextValue, request)
	if err != nil {
		return nil, err
	}
	if s.directedMessages == nil {
		return nil, errors.New("room directed message store is not configured")
	}
	if err = s.directedMessages.AppendMessage(contextValue.Room.OwnerUserID, *message); err != nil {
		return nil, err
	}
	s.touchSharedConversationActivity(ctx, message.ConversationID, time.UnixMilli(message.Timestamp).UTC())

	event := newRoomDirectedMessageEvent(*message)
	s.broadcastSharedEventWithTimeout(ctx, protocol.BuildRoomSharedSessionKey(message.ConversationID), message.RoomID, event)
	s.loggerFor(ctx).Info("Room directed message 已创建",
		"room_id", message.RoomID,
		"conversation_id", message.ConversationID,
		"message_id", message.MessageID,
		"source_agent_id", message.SourceAgentID,
		"recipient_agent_ids", message.Recipients,
		"wake_policy", message.WakePolicy,
		"reply_route", message.ReplyRoute.Mode,
		"reply_recipients", message.ReplyRoute.Recipients,
		"reply_wake_policy", message.ReplyRoute.WakePolicy,
		"delay_seconds", message.DelaySeconds,
		"content_chars", utf8.RuneCountInString(message.Content),
	)
	if err = s.startRoomDirectedMessageWake(ctx, contextValue, *message); err != nil {
		s.loggerFor(ctx).Error("启动 Room directed message 唤醒失败",
			"room_id", message.RoomID,
			"conversation_id", message.ConversationID,
			"message_id", message.MessageID,
			"source_agent_id", message.SourceAgentID,
			"recipient_agent_ids", message.Recipients,
			"wake_policy", message.WakePolicy,
			"delay_seconds", message.DelaySeconds,
			"err", err,
		)
		return message, fmt.Errorf(
			"directed message %s 已持久化，但唤醒未启动: %w",
			message.MessageID,
			err,
		)
	}
	return message, nil
}

func (s *Service) resolveDirectedMessageContext(
	ctx context.Context,
	roomID string,
	conversationID string,
) (*protocol.ConversationContextAggregate, error) {
	contextValue, err := s.resolveGroupMessageContext(ctx, roomID, conversationID)
	if err != nil {
		return nil, err
	}
	// Runtime 注入的工具列表和 permission handler 只负责体验层热重载。
	// 安全撤销必须以每次调用时重新读取到的 Room 真相源为准，避免旧 slot
	// 在 private_messages_enabled 关闭后继续使用已经注入的工具。
	if !contextValue.Room.PrivateMessagesEnabled {
		return nil, errors.New("Room private messaging is disabled")
	}
	return contextValue, nil
}

func (s *Service) resolveGroupMessageContext(
	ctx context.Context,
	roomID string,
	conversationID string,
) (*protocol.ConversationContextAggregate, error) {
	if s.rooms == nil {
		return nil, errors.New("room service is not configured")
	}
	normalizedRoomID := strings.TrimSpace(roomID)
	normalizedConversationID := strings.TrimSpace(conversationID)
	if normalizedRoomID == "" {
		return nil, errors.New("room_id is required")
	}
	if normalizedConversationID == "" {
		return nil, errors.New("conversation_id is required")
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, normalizedConversationID)
	if err != nil {
		return nil, err
	}
	if contextValue.Room.ID != normalizedRoomID {
		return nil, roomsvc.ErrConversationNotFound
	}
	if contextValue.Room.RoomType != protocol.RoomTypeGroup {
		return nil, errors.New("Room message 仅支持 group room")
	}
	return contextValue, nil
}

func (s *Service) buildRoomDirectedMessageRecord(
	contextValue *protocol.ConversationContextAggregate,
	request protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return nil, errors.New("content is required")
	}
	sourceAgentID := strings.TrimSpace(request.SourceAgentID)
	if sourceAgentID == "" {
		return nil, errors.New("source_agent_id is required")
	}
	memberAgentIDs := roomdomain.ListAgentIDs(contextValue.Members)
	if !slices.Contains(memberAgentIDs, sourceAgentID) {
		return nil, roomsvc.ErrRoomMemberNotFound
	}
	recipients := normalizeRoomDirectedMessageRecipients(request.Recipients)
	if len(recipients) == 0 {
		return nil, errors.New("recipients is required")
	}
	if err := validateRoomDirectedMessageRecipients(recipients, memberAgentIDs); err != nil {
		return nil, err
	}
	wakePolicy := request.WakePolicy
	if wakePolicy == "" {
		wakePolicy = protocol.RoomWakePolicyNone
	}
	if err := validateRoomDirectedMessageWakePolicy(wakePolicy); err != nil {
		return nil, err
	}
	if err := validateRoomDirectedMessageDelay(wakePolicy, request.DelaySeconds); err != nil {
		return nil, err
	}
	wakeTargets, err := normalizeRoomDirectedMessageWakeTargets(request.WakeTargets, recipients, wakePolicy)
	if err != nil {
		return nil, err
	}
	replyRoute, err := normalizeRoomReplyRoute(request.ReplyRoute, memberAgentIDs)
	if err != nil {
		return nil, err
	}

	rootRoundID, causedByRoundID, hopIndex := s.resolveRoomMessageCausality(
		contextValue.Conversation.ID,
		sourceAgentID,
		request.RootRoundID,
	)
	return &protocol.RoomDirectedMessageRecord{
		MessageID:       newRealtimeID(),
		RoomID:          contextValue.Room.ID,
		ConversationID:  contextValue.Conversation.ID,
		SourceAgentID:   sourceAgentID,
		Recipients:      recipients,
		WakeTargets:     wakeTargets,
		Content:         content,
		WakePolicy:      wakePolicy,
		ReplyRoute:      replyRoute,
		DelaySeconds:    request.DelaySeconds,
		CorrelationID:   strings.TrimSpace(request.CorrelationID),
		RootRoundID:     rootRoundID,
		CausedByRoundID: causedByRoundID,
		HopIndex:        hopIndex,
		Timestamp:       time.Now().UnixMilli(),
	}, nil
}

func normalizeRoomDirectedMessageRecipients(values []string) []string {
	return normalizeRoomAgentIDs(values)
}

func validateRoomDirectedMessageRecipients(recipients []string, memberAgentIDs []string) error {
	for _, agentID := range recipients {
		if !slices.Contains(memberAgentIDs, agentID) {
			return roomsvc.ErrRoomMemberNotFound
		}
	}
	return nil
}

func normalizeRoomDirectedMessageWakeTargets(
	values []string,
	recipients []string,
	wakePolicy protocol.RoomWakePolicy,
) ([]string, error) {
	targets := normalizeRoomDirectedMessageRecipients(values)
	if wakePolicy == protocol.RoomWakePolicyNone {
		if len(targets) > 0 {
			return nil, errors.New("wake_targets 仅支持会触发运行的 wake_policy")
		}
		return nil, nil
	}
	if len(targets) == 0 {
		return slices.Clone(recipients), nil
	}
	for _, target := range targets {
		if !slices.Contains(recipients, target) {
			return nil, errors.New("wake_targets must be a subset of recipients")
		}
	}
	return targets, nil
}

func normalizeRoomReplyRoute(
	route protocol.RoomReplyRoute,
	memberAgentIDs []string,
) (protocol.RoomReplyRoute, error) {
	return normalizeRoomReplyRouteDepth(route, memberAgentIDs, 0)
}

func normalizeRoomReplyRouteDepth(
	route protocol.RoomReplyRoute,
	memberAgentIDs []string,
	depth int,
) (protocol.RoomReplyRoute, error) {
	if depth > roomReplyRouteMaxDepth {
		return protocol.RoomReplyRoute{}, errors.New("reply_route next_reply_route 嵌套过深")
	}
	mode := route.Mode
	if mode == "" {
		mode = protocol.RoomReplyRouteNone
	}
	switch mode {
	case protocol.RoomReplyRoutePublic:
		return normalizeTerminalRoomReplyRoute(route, protocol.RoomReplyRoutePublic)
	case protocol.RoomReplyRouteNone:
		return normalizeTerminalRoomReplyRoute(route, protocol.RoomReplyRouteNone)
	case protocol.RoomReplyRoutePrivate:
		return normalizePrivateRoomReplyRoute(route, memberAgentIDs, depth)
	default:
		return protocol.RoomReplyRoute{}, errors.New("reply_route mode 不支持")
	}
}

func normalizeTerminalRoomReplyRoute(
	route protocol.RoomReplyRoute,
	mode protocol.RoomReplyRouteMode,
) (protocol.RoomReplyRoute, error) {
	if route.NextReplyRoute != nil {
		return protocol.RoomReplyRoute{}, errors.New("next_reply_route 仅支持 reply_route=private")
	}
	return protocol.RoomReplyRoute{Mode: mode}, nil
}

func normalizePrivateRoomReplyRoute(
	route protocol.RoomReplyRoute,
	memberAgentIDs []string,
	depth int,
) (protocol.RoomReplyRoute, error) {
	recipients := normalizeRoomDirectedMessageRecipients(route.Recipients)
	if len(recipients) == 0 {
		return protocol.RoomReplyRoute{}, errors.New("reply_route private requires recipients")
	}
	if err := validateRoomDirectedMessageRecipients(recipients, memberAgentIDs); err != nil {
		return protocol.RoomReplyRoute{}, err
	}
	wakePolicy := route.WakePolicy
	if wakePolicy == "" {
		wakePolicy = protocol.RoomWakePolicyNone
	}
	if wakePolicy != protocol.RoomWakePolicyNone && wakePolicy != protocol.RoomWakePolicyImmediate {
		return protocol.RoomReplyRoute{}, errors.New("reply_route private wake_policy must be none or immediate")
	}
	normalized := protocol.RoomReplyRoute{
		Mode:       protocol.RoomReplyRoutePrivate,
		Recipients: recipients,
		WakePolicy: wakePolicy,
	}
	if route.NextReplyRoute == nil {
		return normalized, nil
	}
	if wakePolicy != protocol.RoomWakePolicyImmediate {
		return protocol.RoomReplyRoute{}, errors.New("next_reply_route requires reply_route private wake_policy=immediate")
	}
	nextReplyRoute, err := normalizeRoomReplyRouteDepth(*route.NextReplyRoute, memberAgentIDs, depth+1)
	if err != nil {
		return protocol.RoomReplyRoute{}, err
	}
	normalized.NextReplyRoute = &nextReplyRoute
	return normalized, nil
}

func validateRoomDirectedMessageWakePolicy(wakePolicy protocol.RoomWakePolicy) error {
	switch wakePolicy {
	case protocol.RoomWakePolicyNone, protocol.RoomWakePolicyImmediate, protocol.RoomWakePolicyDelayed:
		return nil
	default:
		return errors.New("wake_policy 不支持")
	}
}

func validateRoomDirectedMessageDelay(wakePolicy protocol.RoomWakePolicy, delaySeconds int) error {
	if wakePolicy == protocol.RoomWakePolicyDelayed {
		if delaySeconds <= 0 {
			return errors.New("wake_policy=delayed requires delay_seconds")
		}
		if delaySeconds > roomDirectedMessageMaxDelaySeconds {
			return errors.New("delay_seconds 超出最大值")
		}
		return nil
	}
	if delaySeconds != 0 {
		return errors.New("delay_seconds 仅支持 wake_policy=delayed")
	}
	return nil
}

func newRoomDirectedMessageEvent(message protocol.RoomDirectedMessageRecord) protocol.EventMessage {
	data := map[string]any{
		"message_id":         message.MessageID,
		"event_kind":         "created",
		"room_id":            message.RoomID,
		"conversation_id":    message.ConversationID,
		"source_agent_id":    message.SourceAgentID,
		"recipients":         slices.Clone(message.Recipients),
		"wake_targets":       slices.Clone(message.WakeTargets),
		"reply_route":        message.ReplyRoute,
		"content_chars":      utf8.RuneCountInString(message.Content),
		"root_round_id":      message.RootRoundID,
		"caused_by_round_id": message.CausedByRoundID,
		"hop_index":          message.HopIndex,
	}
	if message.WakePolicy != "" {
		data["wake_policy"] = string(message.WakePolicy)
	}
	if message.DelaySeconds > 0 {
		data["delay_seconds"] = message.DelaySeconds
	}
	if strings.TrimSpace(message.CorrelationID) != "" {
		data["correlation_id"] = strings.TrimSpace(message.CorrelationID)
	}
	event := protocol.NewEvent(protocol.EventTypeRoomDirectedMessage, data)
	event.RoomID = message.RoomID
	event.ConversationID = message.ConversationID
	event.AgentID = message.SourceAgentID
	event.MessageID = message.MessageID
	return event
}

func newRoomDirectedMessageWakeEvent(
	roundValue *activeRoomRound,
	wake publicMentionWake,
	eventKind string,
	extra map[string]any,
) protocol.EventMessage {
	data := map[string]any{
		"message_id":      strings.TrimSpace(wake.MessageID),
		"event_kind":      strings.TrimSpace(eventKind),
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"source_agent_id": strings.TrimSpace(wake.SourceAgentID),
		"target_agent_id": strings.TrimSpace(wake.TargetAgentID),
		"reply_route":     wake.ReplyRoute,
	}
	for key, value := range extra {
		data[key] = value
	}
	event := protocol.NewEvent(protocol.EventTypeRoomDirectedMessage, data)
	event.SessionKey = protocol.BuildRoomSharedSessionKey(roundValue.ConversationID)
	event.RoomID = roundValue.RoomID
	event.ConversationID = roundValue.ConversationID
	event.AgentID = strings.TrimSpace(wake.SourceAgentID)
	event.MessageID = strings.TrimSpace(wake.MessageID)
	return event
}

func newRoomDirectedMessageScheduledWakeEvent(message protocol.RoomDirectedMessageRecord) protocol.EventMessage {
	data := map[string]any{
		"message_id":      message.MessageID,
		"event_kind":      "wake_scheduled",
		"room_id":         message.RoomID,
		"conversation_id": message.ConversationID,
		"source_agent_id": message.SourceAgentID,
		"recipients":      slices.Clone(message.Recipients),
		"wake_targets":    slices.Clone(message.WakeTargets),
		"reply_route":     message.ReplyRoute,
		"wake_policy":     string(message.WakePolicy),
		"delay_seconds":   message.DelaySeconds,
	}
	event := protocol.NewEvent(protocol.EventTypeRoomDirectedMessage, data)
	event.SessionKey = protocol.BuildRoomSharedSessionKey(message.ConversationID)
	event.RoomID = message.RoomID
	event.ConversationID = message.ConversationID
	event.AgentID = message.SourceAgentID
	event.MessageID = message.MessageID
	return event
}

func roomSlotReplyRoute(slot *activeRoomSlot) protocol.RoomReplyRoute {
	route := slot.replyRoute()
	if route.Mode == "" {
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRoutePublic}
	}
	return route
}

func roomSlotPublishesPublicOutput(slot *activeRoomSlot) bool {
	return roomSlotReplyRoute(slot).Mode == protocol.RoomReplyRoutePublic
}

func roomSlotShouldDropPublicOutputEvent(slot *activeRoomSlot, event protocol.EventMessage) bool {
	if roomSlotPublishesPublicOutput(slot) {
		return false
	}
	return event.EventType == protocol.EventTypeStream || event.EventType == protocol.EventTypeMessage
}

func (s *Service) recordRoomDirectedMessageReply(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	assistantMessage protocol.Message,
) error {
	if s.directedMessages == nil || roundValue == nil || slot == nil || strings.TrimSpace(slot.replySourceMessage()) == "" {
		return nil
	}
	replyRoute := roomSlotReplyRoute(slot)
	if replyRoute.Mode != protocol.RoomReplyRoutePrivate {
		return nil
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return err
	}
	recipients := normalizeRoomDirectedMessageRecipients(replyRoute.Recipients)
	if len(recipients) == 0 {
		return nil
	}
	content := strings.TrimSpace(roomdomain.ExtractAssistantResultText(assistantMessage))
	if content == "" {
		return nil
	}

	message := protocol.RoomDirectedMessageRecord{
		MessageID:       newRealtimeID(),
		RoomID:          roundValue.RoomID,
		ConversationID:  roundValue.ConversationID,
		SourceAgentID:   strings.TrimSpace(slot.AgentID),
		Recipients:      recipients,
		WakeTargets:     nil,
		Content:         content,
		WakePolicy:      protocol.RoomWakePolicyNone,
		ReplyRoute:      roomReplyRouteAfterPrivateHandback(replyRoute),
		RootRoundID:     roomRootRoundID(roundValue),
		CausedByRoundID: strings.TrimSpace(roundValue.RoundID),
		HopIndex:        roundValue.HopIndex,
		Timestamp:       time.Now().UnixMilli(),
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return err
	}
	if err := s.directedMessages.AppendMessage(roundValue.OwnerUserID, message); err != nil {
		return err
	}
	if err := s.ensureSlotOutputAuthorized(ctx, roundValue, slot); err != nil {
		return err
	}
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, newRoomDirectedMessageEvent(message))
	if replyRoute.WakePolicy == protocol.RoomWakePolicyImmediate {
		wakeMessage := message
		wakeMessage.WakePolicy = protocol.RoomWakePolicyImmediate
		wakeMessage.WakeTargets = slices.Clone(recipients)
		return s.runRoomDirectedMessageWake(ctx, roundValue.Context, wakeMessage)
	}
	return nil
}

func roomReplyRouteAfterPrivateHandback(route protocol.RoomReplyRoute) protocol.RoomReplyRoute {
	if route.NextReplyRoute == nil {
		return protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone}
	}
	return cloneRoomReplyRoute(*route.NextReplyRoute)
}

func cloneRoomReplyRoute(route protocol.RoomReplyRoute) protocol.RoomReplyRoute {
	cloned := protocol.RoomReplyRoute{
		Mode:       route.Mode,
		Recipients: slices.Clone(route.Recipients),
		WakePolicy: route.WakePolicy,
	}
	if route.NextReplyRoute != nil {
		next := cloneRoomReplyRoute(*route.NextReplyRoute)
		cloned.NextReplyRoute = &next
	}
	return cloned
}

// INPUT: 已持久化的 Room directed message 与其唤醒目标。
// OUTPUT: 去重、有界、可过期且能短窗口合并的 Agent 唤醒队列。
// POS: directed message 从“可见记录”进入“执行轮次”的唯一入口。
const (
	roomDirectedWakeQueueCapacity = 64
	roomDirectedWakeQueueTTL      = 24 * time.Hour
	roomDirectedWakeBatchWindow   = 200 * time.Millisecond
)

func (s *Service) enqueueRoomDirectedMessageWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	message protocol.RoomDirectedMessageRecord,
) error {
	wakeContent, ok := roomDirectedMessageWakeContent(message)
	if !ok || contextValue == nil {
		return nil
	}
	targetAgentIDs := roomDirectedMessageWakeTargetAgentIDs(message)
	if len(targetAgentIDs) == 0 {
		return nil
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return err
	}
	now := time.Now()
	accepted := false
	for _, targetAgentID := range targetAgentIDs {
		location, exists := locations[targetAgentID]
		if !exists {
			continue
		}
		_, inserted, enqueueErr := s.inputQueue.EnqueueBounded(location.Location, protocol.InputQueueItem{
			Scope:           protocol.InputQueueScopeRoom,
			SessionKey:      location.Location.SessionKey,
			RoomID:          message.RoomID,
			ConversationID:  message.ConversationID,
			AgentID:         targetAgentID,
			SourceAgentID:   strings.TrimSpace(message.SourceAgentID),
			SourceMessageID: strings.TrimSpace(message.MessageID),
			TargetAgentIDs:  []string{targetAgentID},
			Source:          protocol.InputQueueSourceAgentRoomMessage,
			Content:         wakeContent,
			DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
			ReplyRoute:      message.ReplyRoute,
			OwnerUserID:     authctx.OwnerUserID(ctx),
			RootRoundID:     firstNonEmptyString(message.RootRoundID, message.MessageID),
			HopIndex:        message.HopIndex,
			ExpiresAt:       now.Add(roomDirectedWakeQueueTTL).UnixMilli(),
		}, roomDirectedWakeQueueCapacity)
		if enqueueErr != nil {
			return enqueueErr
		}
		accepted = accepted || inserted
	}
	if !accepted {
		return nil
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(message.ConversationID)
	if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
		return err
	}
	s.scheduleRoomDirectedQueueDispatch(
		contextWithExactQueueOwner(context.Background(), authctx.OwnerUserID(ctx)),
		sessionKey,
		message.RoomID,
		message.ConversationID,
	)
	return nil
}

func (s *Service) scheduleRoomDirectedQueueDispatch(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
) {
	key := strings.TrimSpace(conversationID)
	if key == "" {
		return
	}
	ownerUserID := authctx.OwnerUserID(ctx)
	s.wakeTimers.ScheduleDispatch(key, roomDirectedWakeBatchWindow, func() {
		s.startSessionBackgroundTask(
			sessionKey,
			ownerUserID,
			func(taskCtx context.Context) {
				s.dispatchNextInputQueueItem(taskCtx, sessionKey, roomID, conversationID)
			},
		)
	})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if normalized := strings.TrimSpace(value); normalized != "" {
			return normalized
		}
	}
	return ""
}

const roomDirectedMessageTriggerType = "room_directed_message"
const roomDirectedMessageWakeRetryDelay = 30 * time.Second

func (s *Service) startRoomDirectedMessageWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	message protocol.RoomDirectedMessageRecord,
) error {
	if contextValue == nil {
		return nil
	}
	if message.WakePolicy == protocol.RoomWakePolicyNone {
		return nil
	}
	if message.WakePolicy == protocol.RoomWakePolicyDelayed {
		return s.scheduleRoomDirectedMessageWake(ctx, message)
	}
	return s.runRoomDirectedMessageWake(ctx, contextValue, message)
}

func (s *Service) runRoomDirectedMessageWake(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	message protocol.RoomDirectedMessageRecord,
) error {
	return s.enqueueRoomDirectedMessageWake(ctx, contextValue, message)
}

func (s *Service) scheduleRoomDirectedMessageWake(ctx context.Context, message protocol.RoomDirectedMessageRecord) error {
	delay := time.Duration(message.DelaySeconds) * time.Second
	if delay <= 0 {
		return errors.New("delay_seconds must be positive")
	}
	if s.directedWakes == nil {
		return errors.New("room directed wake store is not configured")
	}
	wake := workspacestore.RoomDirectedMessageWake{
		WakeID:      strings.TrimSpace(message.MessageID),
		OwnerUserID: authctx.OwnerUserID(ctx),
		Message:     message,
		DueAt:       time.Now().Add(delay).UnixMilli(),
		CreatedAt:   time.Now().UnixMilli(),
	}
	if err := s.directedWakes.Schedule(wake); err != nil {
		return err
	}
	s.schedulePersistedRoomDirectedWake(wake, delay)
	sessionKey := protocol.BuildRoomSharedSessionKey(message.ConversationID)
	s.broadcastSharedEventWithTimeout(ctx, sessionKey, message.RoomID, newRoomDirectedMessageScheduledWakeEvent(message))
	s.loggerFor(ctx).Info("Room directed message 延迟唤醒已计划",
		"room_id", message.RoomID,
		"conversation_id", message.ConversationID,
		"message_id", message.MessageID,
		"recipient_agent_ids", message.Recipients,
		"delay_seconds", message.DelaySeconds,
	)
	return nil
}

// StartDelayedWakeScheduler 恢复宕机前未完成的 Room 延迟唤醒。
func (s *Service) StartDelayedWakeScheduler(context.Context) (func(), error) {
	if s.directedWakes == nil {
		return nil, nil
	}
	pending, err := s.directedWakes.PendingAll()
	if err != nil {
		return nil, err
	}
	s.wakeTimers.Start()
	for _, wake := range pending {
		delay := time.Until(time.UnixMilli(wake.DueAt))
		if delay < 0 {
			delay = 0
		}
		s.schedulePersistedRoomDirectedWake(wake, delay)
	}
	return s.stopRoomWakeSchedulers, nil
}

func (s *Service) schedulePersistedRoomDirectedWake(
	wake workspacestore.RoomDirectedMessageWake,
	delay time.Duration,
) {
	wakeID := strings.TrimSpace(wake.WakeID)
	if wakeID == "" {
		return
	}
	s.wakeTimers.ScheduleDelayed(wakeID, delay, func() {
		s.executePersistedRoomDirectedWake(wake)
	})
}

func (s *Service) executePersistedRoomDirectedWake(wake workspacestore.RoomDirectedMessageWake) {
	wakeCtx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     strings.TrimSpace(wake.OwnerUserID),
		Username:   strings.TrimSpace(wake.OwnerUserID),
		Role:       authctx.RoleOwner,
		AuthMethod: "room_directed_message_delayed",
	})
	message := wake.Message
	contextValue, err := s.resolveDirectedMessageContext(wakeCtx, message.RoomID, message.ConversationID)
	if err == nil {
		err = s.runRoomDirectedMessageWake(wakeCtx, contextValue, message)
	}
	if err != nil {
		s.loggerFor(wakeCtx).Error("执行 Room directed message 延迟唤醒失败，稍后重试",
			"room_id", message.RoomID,
			"conversation_id", message.ConversationID,
			"message_id", message.MessageID,
			"err", err,
		)
		s.schedulePersistedRoomDirectedWake(wake, roomDirectedMessageWakeRetryDelay)
		return
	}
	if err = s.directedWakes.Complete(wake.OwnerUserID, wake.WakeID); err != nil {
		s.loggerFor(wakeCtx).Error("记录 Room directed message 延迟唤醒完成失败", "wake_id", wake.WakeID, "err", err)
	}
}

func (s *Service) stopRoomWakeSchedulers() {
	s.wakeTimers.Stop()
}

func roomDirectedMessageWakeContent(message protocol.RoomDirectedMessageRecord) (string, bool) {
	if message.WakePolicy != protocol.RoomWakePolicyImmediate &&
		message.WakePolicy != protocol.RoomWakePolicyDelayed {
		return "", false
	}
	return "A Room directed message was delivered to you. Read the content projected in <room_directed_messages> and answer according to reply_route.", true
}

func roomDirectedMessageWakeTargetAgentIDs(message protocol.RoomDirectedMessageRecord) []string {
	targets := message.WakeTargets
	if len(targets) == 0 && message.WakePolicy != protocol.RoomWakePolicyNone {
		targets = message.Recipients
	}
	return normalizeRoomAgentIDs(targets)
}

// INPUT: delayed wake 和短窗口 dispatch 的唯一键计时任务。
// OUTPUT: 去重调度、回调前释放与统一停机。
// POS: Room 唤醒计时器的封装边界，避免 Service 持有多组锁和 map。
type roomWakeTimerRegistry struct {
	mu       sync.Mutex
	delayed  map[string]*time.Timer
	dispatch map[string]*time.Timer
	stopped  bool
}

func newRoomWakeTimerRegistry() *roomWakeTimerRegistry {
	return &roomWakeTimerRegistry{
		delayed:  make(map[string]*time.Timer),
		dispatch: make(map[string]*time.Timer),
	}
}

func (r *roomWakeTimerRegistry) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = false
}

func (r *roomWakeTimerRegistry) ScheduleDelayed(key string, delay time.Duration, callback func()) {
	r.schedule(r.delayed, key, delay, callback)
}

func (r *roomWakeTimerRegistry) ScheduleDispatch(key string, delay time.Duration, callback func()) {
	r.schedule(r.dispatch, key, delay, callback)
}

func (r *roomWakeTimerRegistry) schedule(
	timers map[string]*time.Timer,
	key string,
	delay time.Duration,
	callback func(),
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped || key == "" {
		return
	}
	if _, exists := timers[key]; exists {
		return
	}
	timers[key] = time.AfterFunc(delay, func() {
		r.mu.Lock()
		delete(timers, key)
		r.mu.Unlock()
		if callback != nil {
			callback()
		}
	})
}

func (r *roomWakeTimerRegistry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	stopRoomWakeTimers(r.delayed)
	stopRoomWakeTimers(r.dispatch)
}

func stopRoomWakeTimers(timers map[string]*time.Timer) {
	for key, timer := range timers {
		timer.Stop()
		delete(timers, key)
	}
}
