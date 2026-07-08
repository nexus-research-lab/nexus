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
		"timestamp":             time.Now().UnixMilli(),
	}
	if correlationID := strings.TrimSpace(request.CorrelationID); correlationID != "" {
		message["correlation_id"] = correlationID
	}
	if err = s.persistSharedInlineMessage(contextValue.Conversation.ID, message); err != nil {
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
	if err = s.startPublicMessageMentionWakes(ctx, contextValue, sourceAgentID, messageID, content); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *RealtimeService) startPublicMessageMentionWakes(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	content string,
) error {
	targetAgentIDs := roomdomain.ResolveMentionAgentIDs(content, roomdomain.BuildMentionAliases(contextValue))
	if len(targetAgentIDs) == 0 {
		return nil
	}
	parentRound := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID),
		RoomID:         contextValue.Room.ID,
		ConversationID: contextValue.Conversation.ID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        messageID,
		RootRoundID:    messageID,
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
		wakes = append(wakes, publicMentionWake{
			SourceAgentID: strings.TrimSpace(sourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       strings.TrimSpace(content),
			MessageID:     strings.TrimSpace(messageID),
		})
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}
