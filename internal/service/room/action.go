package room

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// HandleAction 处理 Room 内部协作动作。
func (s *RealtimeService) HandleAction(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomActionRequest,
) (*protocol.RoomActionRecord, error) {
	contextValue, err := s.resolveActionContext(ctx, roomID, conversationID)
	if err != nil {
		return nil, err
	}
	action, err := s.buildRoomActionRecord(contextValue, request)
	if err != nil {
		return nil, err
	}
	if s.actions == nil {
		return nil, errors.New("room action store is not configured")
	}
	if err = s.actions.AppendAction(*action); err != nil {
		return nil, err
	}

	event := newRoomActionEvent(*action)
	s.broadcastSharedEventWithTimeout(ctx, protocol.BuildRoomSharedSessionKey(action.ConversationID), action.RoomID, event)
	s.loggerFor(ctx).Info("Room action 已创建",
		"room_id", action.RoomID,
		"conversation_id", action.ConversationID,
		"action_type", action.ActionType,
		"source_agent_id", action.SourceAgentID,
		"target_agent_id", action.TargetAgentID,
		"content_chars", utf8.RuneCountInString(action.Content),
	)
	if err = s.startRoomActionWake(ctx, contextValue, *action); err != nil {
		s.loggerFor(ctx).Error("启动 Room action 唤醒失败",
			"room_id", action.RoomID,
			"conversation_id", action.ConversationID,
			"action_id", action.ActionID,
			"action_type", action.ActionType,
			"source_agent_id", action.SourceAgentID,
			"target_agent_id", action.TargetAgentID,
			"err", err,
		)
	}
	return action, nil
}

func (s *RealtimeService) resolveActionContext(
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
		return nil, errors.New("room action 仅支持 group room")
	}
	return contextValue, nil
}

func (s *RealtimeService) buildRoomActionRecord(
	contextValue *protocol.ConversationContextAggregate,
	request protocol.CreateRoomActionRequest,
) (*protocol.RoomActionRecord, error) {
	actionType, err := normalizeRoomActionType(request.ActionType)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return nil, errors.New("content is required")
	}
	sourceAgentID := strings.TrimSpace(request.SourceAgentID)
	if sourceAgentID == "" {
		return nil, errors.New("source_agent_id is required")
	}
	memberAgentIDs := roomdomain.ListAgentIDs(contextValue.Members)
	if !roomdomain.ContainsString(memberAgentIDs, sourceAgentID) {
		return nil, ErrRoomMemberNotFound
	}

	targetAgentID := strings.TrimSpace(request.TargetAgentID)
	visibility := normalizeRoomActionVisibility(request.Visibility)
	replyTarget := request.ReplyTarget
	wakePolicy := request.WakePolicy
	audienceAgentIDs := normalizeRoomActionAudience(request.AudienceAgentIDs)

	switch actionType {
	case protocol.RoomActionTypePrivateMessage:
		if targetAgentID == "" {
			return nil, errors.New("target_agent_id is required")
		}
		if !roomdomain.ContainsString(memberAgentIDs, targetAgentID) {
			return nil, ErrRoomMemberNotFound
		}
		visibility = protocol.RoomActionVisibilityPrivate
		if replyTarget == "" {
			replyTarget = protocol.RoomReplyTargetTargetPrivate
		}
	case protocol.RoomActionTypeRequestReply:
		if targetAgentID == "" {
			return nil, errors.New("target_agent_id is required")
		}
		if !roomdomain.ContainsString(memberAgentIDs, targetAgentID) {
			return nil, ErrRoomMemberNotFound
		}
		visibility = protocol.RoomActionVisibilityPrivate
		if replyTarget == "" {
			replyTarget = protocol.RoomReplyTargetPublicFeed
		}
		if wakePolicy == "" {
			wakePolicy = protocol.RoomWakePolicyImmediate
		}
	case protocol.RoomActionTypePrivateNote:
		targetAgentID = sourceAgentID
		visibility = protocol.RoomActionVisibilityPrivate
		if replyTarget == "" {
			replyTarget = protocol.RoomReplyTargetSenderPrivate
		}
	case protocol.RoomActionTypeMarker:
		if replyTarget == "" {
			replyTarget = protocol.RoomReplyTargetSenderPrivate
			if visibility == protocol.RoomActionVisibilityPublic {
				replyTarget = protocol.RoomReplyTargetPublicFeed
			}
		}
		if visibility == protocol.RoomActionVisibilityPrivate &&
			targetAgentID == "" &&
			replyTarget == protocol.RoomReplyTargetSenderPrivate {
			targetAgentID = sourceAgentID
		}
		if targetAgentID != "" && !roomdomain.ContainsString(memberAgentIDs, targetAgentID) {
			return nil, ErrRoomMemberNotFound
		}
	}
	if err = validateRoomReplyTarget(replyTarget, audienceAgentIDs, memberAgentIDs); err != nil {
		return nil, err
	}
	if err = validateRoomWakePolicy(actionType, wakePolicy); err != nil {
		return nil, err
	}

	actionID := newRealtimeID()
	action := &protocol.RoomActionRecord{
		ActionID:         actionID,
		RoomID:           contextValue.Room.ID,
		ConversationID:   contextValue.Conversation.ID,
		ActionType:       actionType,
		SourceAgentID:    sourceAgentID,
		TargetAgentID:    targetAgentID,
		AudienceAgentIDs: audienceAgentIDs,
		Content:          content,
		Visibility:       visibility,
		ReplyTarget:      replyTarget,
		WakePolicy:       wakePolicy,
		Timestamp:        time.Now().UnixMilli(),
	}
	if action.ActionType == protocol.RoomActionTypeRequestReply {
		action.RequestID = actionID
	}
	return action, nil
}

func normalizeRoomActionType(actionType protocol.RoomActionType) (protocol.RoomActionType, error) {
	switch actionType {
	case protocol.RoomActionTypePrivateMessage,
		protocol.RoomActionTypeRequestReply,
		protocol.RoomActionTypePrivateNote,
		protocol.RoomActionTypeMarker:
		return actionType, nil
	default:
		return "", errors.New("action_type 不支持")
	}
}

func normalizeRoomActionVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case protocol.RoomActionVisibilityPublic:
		return protocol.RoomActionVisibilityPublic
	default:
		return protocol.RoomActionVisibilityPrivate
	}
}

func normalizeRoomActionAudience(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" || roomdomain.ContainsString(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func validateRoomReplyTarget(
	replyTarget protocol.RoomReplyTarget,
	audienceAgentIDs []string,
	memberAgentIDs []string,
) error {
	switch replyTarget {
	case protocol.RoomReplyTargetPublicFeed,
		protocol.RoomReplyTargetSenderPrivate,
		protocol.RoomReplyTargetTargetPrivate,
		protocol.RoomReplyTargetNone:
		return nil
	case protocol.RoomReplyTargetAudience:
		if len(audienceAgentIDs) == 0 {
			return errors.New("audience reply_target requires audience_agent_ids")
		}
		for _, agentID := range audienceAgentIDs {
			if !roomdomain.ContainsString(memberAgentIDs, agentID) {
				return ErrRoomMemberNotFound
			}
		}
		return nil
	default:
		return errors.New("reply_target 不支持")
	}
}

func validateRoomWakePolicy(actionType protocol.RoomActionType, wakePolicy protocol.RoomWakePolicy) error {
	if wakePolicy == "" {
		return nil
	}
	if actionType != protocol.RoomActionTypeRequestReply {
		return errors.New("wake_policy 仅支持 request_reply")
	}
	switch wakePolicy {
	case protocol.RoomWakePolicyNone, protocol.RoomWakePolicyImmediate:
		return nil
	default:
		return errors.New("wake_policy 不支持")
	}
}

func newRoomActionEvent(action protocol.RoomActionRecord) protocol.EventMessage {
	data := map[string]any{
		"action_id":       action.ActionID,
		"event_kind":      "created",
		"room_id":         action.RoomID,
		"conversation_id": action.ConversationID,
		"action_type":     string(action.ActionType),
		"source_agent_id": action.SourceAgentID,
		"visibility":      action.Visibility,
		"reply_target":    string(action.ReplyTarget),
		"content_chars":   utf8.RuneCountInString(action.Content),
	}
	if action.RequestID != "" {
		data["request_id"] = action.RequestID
	}
	if action.WakePolicy != "" {
		data["wake_policy"] = string(action.WakePolicy)
	}
	if action.TargetAgentID != "" {
		data["target_agent_id"] = action.TargetAgentID
	}
	if len(action.AudienceAgentIDs) > 0 {
		data["audience_agent_ids"] = append([]string(nil), action.AudienceAgentIDs...)
	}
	if action.ActionType == protocol.RoomActionTypeMarker &&
		action.Visibility == protocol.RoomActionVisibilityPublic &&
		action.ReplyTarget != protocol.RoomReplyTargetNone {
		data["content"] = action.Content
	}
	event := protocol.NewEvent(protocol.EventTypeRoomAction, data)
	event.RoomID = action.RoomID
	event.ConversationID = action.ConversationID
	event.AgentID = action.SourceAgentID
	return event
}

func newRoomActionWakeEvent(
	roundValue *activeRoomRound,
	wake publicMentionWake,
	eventKind string,
	extra map[string]any,
) protocol.EventMessage {
	data := map[string]any{
		"action_id":       strings.TrimSpace(wake.MessageID),
		"event_kind":      strings.TrimSpace(eventKind),
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"source_agent_id": strings.TrimSpace(wake.SourceAgentID),
		"target_agent_id": strings.TrimSpace(wake.TargetAgentID),
		"reply_target":    string(wake.ReplyTarget),
	}
	if requestID := strings.TrimSpace(wake.RequestID); requestID != "" {
		data["request_id"] = requestID
	}
	if len(wake.ReplyAudience) > 0 {
		data["audience_agent_ids"] = append([]string(nil), wake.ReplyAudience...)
	}
	for key, value := range extra {
		data[key] = value
	}
	event := protocol.NewEvent(protocol.EventTypeRoomAction, data)
	event.SessionKey = protocol.BuildRoomSharedSessionKey(roundValue.ConversationID)
	event.RoomID = roundValue.RoomID
	event.ConversationID = roundValue.ConversationID
	event.AgentID = strings.TrimSpace(wake.SourceAgentID)
	event.CausedBy = strings.TrimSpace(wake.MessageID)
	return event
}
