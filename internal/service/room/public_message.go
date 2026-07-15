package room

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// HandlePublicMessage 处理 Room 成员通过受控工具主动发布的公区消息。
func (s *RealtimeService) HandlePublicMessage(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	contextValue, err := s.resolveDirectedMessageContext(ctx, roomID, conversationID)
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
	if !slices.Contains(memberAgentIDs, sourceAgentID) {
		return nil, ErrRoomMemberNotFound
	}

	messageID := newRealtimeID()
	roundID := protocol.NewRoundID()
	rootRoundID, causedByRoundID, hopIndex := s.resolveRoomMessageCausality(
		contextValue.Conversation.ID,
		sourceAgentID,
		request.RootRoundID,
	)
	if rootRoundID == "" {
		rootRoundID = messageID
	}
	if causedByRoundID == "" {
		causedByRoundID = messageID
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID)
	message := protocol.Message{
		"message_id":      messageID,
		"session_key":     sessionKey,
		"room_id":         contextValue.Room.ID,
		"conversation_id": contextValue.Conversation.ID,
		"agent_id":        sourceAgentID,
		"round_id":        roundID,
		"role":            "assistant",
		"content": []map[string]any{
			{"type": "text", "text": content},
		},
		"is_complete":           true,
		"stop_reason":           "room_public_message",
		"room_message_source":   "nexus_room.publish_public_message",
		"room_message_protocol": "public_feed",
		"root_round_id":         rootRoundID,
		"caused_by_round_id":    causedByRoundID,
		"hop_index":             hopIndex,
		"timestamp":             time.Now().UnixMilli(),
	}
	if mentions := buildPublicMessageMentionAnnotations(contextValue, sourceAgentID, messageID, content); len(mentions) > 0 {
		message["agent_mentions"] = mentions
	}
	if correlationID := strings.TrimSpace(request.CorrelationID); correlationID != "" {
		message["correlation_id"] = correlationID
	}
	if err = s.detectPublicMessageHandoffs(contextValue, sourceAgentID, messageID, content, rootRoundID, hopIndex); err != nil {
		return nil, err
	}
	if err = s.persistSharedInlineMessage(contextValue.Conversation.ID, message); err != nil {
		if s.publicHandoffs != nil {
			_ = s.publicHandoffs.CancelForSource(contextValue.Conversation.ID, messageID, "error")
		}
		return nil, err
	}
	s.broadcastSharedEventWithTimeout(
		ctx,
		sessionKey,
		contextValue.Room.ID,
		roomdomain.WrapMessageEvent(contextValue.Room.ID, contextValue.Conversation.ID, message, roundID),
	)
	s.loggerFor(ctx).Info("Room public message 已发布",
		"room_id", contextValue.Room.ID,
		"conversation_id", contextValue.Conversation.ID,
		"message_id", messageID,
		"source_agent_id", sourceAgentID,
		"content_chars", utf8.RuneCountInString(content),
	)
	if err = s.startPublicMessageMentionWakes(ctx, contextValue, sourceAgentID, messageID, content, rootRoundID, hopIndex); err != nil {
		return nil, err
	}
	return message, nil
}

// MarkPublicMessagePublished 将主动广播写入当前 slot 的运行时状态。
// 后续 assistant/result 事件仍可被 SDK 发送，但不会再次投影到公区。
func (s *RealtimeService) MarkPublicMessagePublished(
	_ context.Context,
	sessionKey string,
	roundID string,
	agentID string,
) error {
	roundValue := s.findActiveRoundByRoundID(strings.TrimSpace(sessionKey), strings.TrimSpace(roundID))
	if roundValue == nil {
		return errors.New("active Room round not found")
	}
	agentID = strings.TrimSpace(agentID)
	for _, slot := range roundValue.Slots {
		if slot != nil && strings.TrimSpace(slot.AgentID) == agentID {
			slot.markPublicMessagePublished()
			return nil
		}
	}
	return errors.New("active Room slot not found")
}

func (s *RealtimeService) startPublicMessageMentionWakes(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	content string,
	rootRoundID string,
	hopIndex int,
) error {
	targetAgentIDs := roomdomain.ResolveMentionAgentIDs(content, roomdomain.BuildMentionAliases(contextValue))
	if len(targetAgentIDs) == 0 {
		return nil
	}
	if err := s.detectPublicMessageHandoffs(contextValue, sourceAgentID, messageID, content, rootRoundID, hopIndex); err != nil {
		return err
	}
	parentRound := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID),
		RoomID:         contextValue.Room.ID,
		ConversationID: contextValue.Conversation.ID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        messageID,
		RootRoundID:    rootRoundID,
		HopIndex:       hopIndex,
		OwnerUserID:    authctx.OwnerUserID(ctx),
	}
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" || targetAgentID == sourceAgentID {
			continue
		}
		if !roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
			continue
		}
		handoffID := roomPublicHandoffID(contextValue.Conversation.ID, messageID, targetAgentID)
		if s.publicHandoffs != nil {
			if err := s.publicHandoffs.MarkSourceFinished(contextValue.Conversation.ID, handoffID); err != nil {
				return err
			}
		}
		wakes = append(wakes, publicMentionWake{
			HandoffID:     handoffID,
			SourceAgentID: strings.TrimSpace(sourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       strings.TrimSpace(content),
			MessageID:     strings.TrimSpace(messageID),
		})
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}

func (s *RealtimeService) detectPublicMessageHandoffs(
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	content string,
	rootRoundID string,
	hopIndex int,
) error {
	if s.publicHandoffs == nil || contextValue == nil {
		return nil
	}
	for _, targetAgentID := range roomdomain.ResolveMentionAgentIDs(content, roomdomain.BuildMentionAliases(contextValue)) {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" || targetAgentID == strings.TrimSpace(sourceAgentID) ||
			!roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
			continue
		}
		handoffID := roomPublicHandoffID(contextValue.Conversation.ID, messageID, targetAgentID)
		if _, _, err := s.publicHandoffs.Detect(workspacestore.RoomPublicHandoff{
			HandoffID:          handoffID,
			ConversationID:     contextValue.Conversation.ID,
			RoomID:             contextValue.Room.ID,
			RootRoundID:        rootRoundID,
			SourceAgentRoundID: messageID,
			SourceMessageID:    messageID,
			SourceAgentID:      strings.TrimSpace(sourceAgentID),
			TargetAgentID:      targetAgentID,
			Content:            strings.TrimSpace(content),
			HopIndex:           hopIndex,
		}); err != nil {
			return err
		}
	}
	return nil
}

func buildPublicMessageMentionAnnotations(
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	content string,
) []protocol.AgentMention {
	if contextValue == nil {
		return nil
	}
	result := make([]protocol.AgentMention, 0)
	for _, match := range roomdomain.ResolveMentionMatches(content, roomdomain.BuildMentionAliases(contextValue)) {
		targetAgentID := strings.TrimSpace(match.AgentID)
		if targetAgentID == "" || targetAgentID == strings.TrimSpace(sourceAgentID) ||
			!roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
			continue
		}
		result = append(result, protocol.AgentMention{
			AgentID:           targetAgentID,
			Label:             strings.TrimSpace(match.Label),
			ContentBlockIndex: 0,
			StartRune:         match.StartRune,
			EndRune:           match.EndRune,
			HandoffID:         roomPublicHandoffID(contextValue.Conversation.ID, messageID, targetAgentID),
		})
	}
	return result
}
