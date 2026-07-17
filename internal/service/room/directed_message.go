package room

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomDirectedMessageMaxDelaySeconds = 86400
const roomReplyRouteMaxDepth = 4

// HandleDirectedMessage 处理 Room 内部私域 directed message。
func (s *RealtimeService) HandleDirectedMessage(
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
	if err = s.directedMessages.AppendMessage(*message); err != nil {
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
	}
	return message, nil
}

func (s *RealtimeService) resolveDirectedMessageContext(
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
		return nil, ErrConversationNotFound
	}
	if contextValue.Room.RoomType != protocol.RoomTypeGroup {
		return nil, errors.New("room directed message 仅支持 group room")
	}
	return contextValue, nil
}

func (s *RealtimeService) buildRoomDirectedMessageRecord(
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
		return nil, ErrRoomMemberNotFound
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
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" || slices.Contains(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func validateRoomDirectedMessageRecipients(recipients []string, memberAgentIDs []string) error {
	for _, agentID := range recipients {
		if !slices.Contains(memberAgentIDs, agentID) {
			return ErrRoomMemberNotFound
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
