// INPUT: Group Room 成员、目标 conversation 与公共正文。
// OUTPUT: 公区持久消息、mention 注解及明确 @ 目标的唤醒。
// POS: Agent 主动跨上下文群发时复用的 Room public feed 边界。
package realtime

import (
	"context"
	"errors"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// HandlePublicMessage 处理 Room 成员通过受控工具主动发布的公区消息。
func (s *Service) HandlePublicMessage(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	contextValue, err := s.resolveDirectedMessageContext(ctx, roomID, conversationID)
	if err != nil {
		return nil, err
	}
	return s.handlePublicMessage(ctx, contextValue, request, "nexus_room.publish_public_message")
}

// HandlePlatformPublicMessage 处理平台通讯能力发起的群消息，不依赖 Room 私域开关。
func (s *Service) HandlePlatformPublicMessage(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	contextValue, err := s.resolveGroupMessageContext(ctx, roomID, conversationID)
	if err != nil {
		return nil, err
	}
	return s.handlePublicMessage(ctx, contextValue, request, "nexus_comms.send_message")
}

func (s *Service) handlePublicMessage(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	request protocol.CreateRoomPublicMessageRequest,
	messageSource string,
) (protocol.Message, error) {
	var err error
	content := roomdomain.StripFanoutMarkerText(request.Content)
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

	messageID := newRealtimeID()
	roundID := protocol.NewRoundID()
	rootRoundID, causedByRoundID, hopIndex := s.resolveRoomMessageCausality(
		contextValue.Conversation.ID,
		sourceAgentID,
		request.RootRoundID,
	)
	goalCollaborationBinding := s.goalCollaborationBindingForActiveRound(
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
		"room_message_source":   messageSource,
		"room_message_protocol": "public_feed",
		"root_round_id":         rootRoundID,
		"caused_by_round_id":    causedByRoundID,
		"hop_index":             hopIndex,
		"timestamp":             time.Now().UnixMilli(),
	}
	mentions := buildPublicMessageMentionAnnotations(contextValue, sourceAgentID, messageID, content)
	targetAgentIDs := handoffTargetAgentIDs(mentions)
	if len(mentions) > 0 {
		message["agent_mentions"] = mentions
	}
	if correlationID := strings.TrimSpace(request.CorrelationID); correlationID != "" {
		message["correlation_id"] = correlationID
	}
	if err = s.detectPublicMessageHandoffs(contextValue, sourceAgentID, messageID, content, rootRoundID, hopIndex, targetAgentIDs, goalCollaborationBinding); err != nil {
		return nil, err
	}
	if err = s.persistSharedInlineMessage(
		contextValue.Room.OwnerUserID,
		contextValue.Conversation.ID,
		message,
	); err != nil {
		if s.publicHandoffs != nil {
			_ = s.publicHandoffs.CancelForSource(
				contextValue.Room.OwnerUserID,
				contextValue.Conversation.ID,
				messageID,
				"error",
			)
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
	if err = s.startPublicMessageMentionWakes(ctx, contextValue, sourceAgentID, messageID, content, rootRoundID, hopIndex, targetAgentIDs, goalCollaborationBinding); err != nil {
		return nil, err
	}
	return message, nil
}

// MarkPublicMessagePublished 将主动广播写入当前 slot 的运行时状态。
// 后续 assistant/result 事件仍可被 SDK 发送，但不会再次投影到公区。
func (s *Service) MarkPublicMessagePublished(
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
			if slot.goalMutationAuthority().valid() &&
				s.publicMessageHasGoalCollaboration(
					roundValue.OwnerUserID,
					roundValue.ConversationID,
					roomRootRoundID(roundValue),
					slot,
				) {
				slot.markPendingGoalCollaboration()
			}
			slot.markPublicMessagePublished()
			return nil
		}
	}
	return errors.New("active Room slot not found")
}

func (s *Service) publicMessageHasGoalCollaboration(
	ownerUserID string,
	conversationID string,
	rootRoundID string,
	slot *activeRoomSlot,
) bool {
	if s == nil || s.publicHandoffs == nil || slot == nil {
		return false
	}
	binding := goalCollaborationBindingFromAuthority(slot.goalMutationAuthority())
	if binding == nil {
		return false
	}
	handoffs, err := s.publicHandoffs.ListRoot(
		ownerUserID,
		conversationID,
		strings.TrimSpace(rootRoundID),
	)
	if err != nil {
		return false
	}
	for _, handoff := range handoffs {
		candidate := protocol.NormalizeGoalCollaborationBinding(
			handoff.GoalCollaborationBinding,
		)
		if candidate != nil && *candidate == *binding {
			return true
		}
	}
	return false
}

func (s *Service) startPublicMessageMentionWakes(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	content string,
	rootRoundID string,
	hopIndex int,
	targetAgentIDs []string,
	goalCollaborationBinding *protocol.GoalCollaborationBinding,
) error {
	if len(targetAgentIDs) == 0 {
		return nil
	}
	// HandlePublicMessage 已在 source 持久化前完成 Detect；这里仅负责把
	// 已记录的 handoff 送入统一 admission/queue/slot 路径，避免重复重放 ledger。
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
			if err := s.publicHandoffs.MarkSourceFinished(
				contextValue.Room.OwnerUserID,
				contextValue.Conversation.ID,
				handoffID,
			); err != nil {
				return err
			}
		}
		wakes = append(wakes, publicMentionWake{
			HandoffID:     handoffID,
			SourceAgentID: strings.TrimSpace(sourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       strings.TrimSpace(content),
			MessageID:     strings.TrimSpace(messageID),
			GoalCollaborationBinding: cloneGoalCollaborationBinding(
				goalCollaborationBinding,
			),
		})
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}

func (s *Service) detectPublicMessageHandoffs(
	contextValue *protocol.ConversationContextAggregate,
	sourceAgentID string,
	messageID string,
	content string,
	rootRoundID string,
	hopIndex int,
	targetAgentIDs []string,
	goalCollaborationBinding *protocol.GoalCollaborationBinding,
) error {
	if s.publicHandoffs == nil || contextValue == nil {
		return nil
	}
	for _, targetAgentID := range targetAgentIDs {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" || targetAgentID == strings.TrimSpace(sourceAgentID) ||
			!roomdomain.IsMemberAgent(contextValue.Members, targetAgentID) {
			continue
		}
		handoffID := roomPublicHandoffID(contextValue.Conversation.ID, messageID, targetAgentID)
		if _, _, err := s.publicHandoffs.Detect(contextValue.Room.OwnerUserID, workspacestore.RoomPublicHandoff{
			HandoffID:          handoffID,
			ConversationID:     contextValue.Conversation.ID,
			RoomID:             contextValue.Room.ID,
			RootRoundID:        rootRoundID,
			SourceAgentRoundID: messageID,
			SourceMessageID:    messageID,
			SourceAgentID:      strings.TrimSpace(sourceAgentID),
			TargetAgentID:      targetAgentID,
			Content:            strings.TrimSpace(content),
			QueueSource:        protocol.InputQueueSourceAgentPublicMention,
			GoalCollaborationBinding: cloneGoalCollaborationBinding(
				goalCollaborationBinding,
			),
			HopIndex: hopIndex,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) goalCollaborationBindingForActiveRound(
	conversationID string,
	sourceAgentID string,
	rootRoundID string,
) *protocol.GoalCollaborationBinding {
	conversationID = strings.TrimSpace(conversationID)
	sourceAgentID = strings.TrimSpace(sourceAgentID)
	rootRoundID = strings.TrimSpace(rootRoundID)
	for _, roundValue := range s.rounds.snapshotConversation(conversationID) {
		if roundValue == nil || strings.TrimSpace(roundValue.ConversationID) != conversationID {
			continue
		}
		if rootRoundID != "" && roomRootRoundID(roundValue) != rootRoundID {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || strings.TrimSpace(slot.AgentID) != sourceAgentID {
				continue
			}
			return goalCollaborationBindingForSlot(roundValue, slot)
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
	return buildRoomMentionAnnotations(
		contextValue,
		sourceAgentID,
		messageID,
		[]roomMentionTextBlock{{index: 0, text: content}},
	)
}

func handoffTargetAgentIDs(mentions []protocol.AgentMention) []string {
	result := make([]string, 0, len(mentions))
	seen := make(map[string]struct{}, len(mentions))
	for _, mention := range mentions {
		if strings.TrimSpace(mention.HandoffID) == "" {
			continue
		}
		agentID := strings.TrimSpace(mention.AgentID)
		if agentID == "" {
			continue
		}
		if _, ok := seen[agentID]; ok {
			continue
		}
		seen[agentID] = struct{}{}
		result = append(result, agentID)
	}
	return result
}

// INPUT: Room runtime 注入的 root round 与当前 Agent。
// OUTPUT: 可持久化到消息的 root / cause / hop 因果信息。
// POS: Room 工具消息与后续唤醒之间的因果链连接点。
func (s *Service) resolveRoomMessageCausality(
	conversationID string,
	sourceAgentID string,
	rootRoundID string,
) (string, string, int) {
	normalizedConversationID := strings.TrimSpace(conversationID)
	normalizedSourceAgentID := strings.TrimSpace(sourceAgentID)
	normalizedRootRoundID := strings.TrimSpace(rootRoundID)

	for _, roundValue := range s.rounds.snapshotConversation(normalizedConversationID) {
		if roundValue == nil || strings.TrimSpace(roundValue.ConversationID) != normalizedConversationID {
			continue
		}
		if normalizedRootRoundID != "" && roomRootRoundID(roundValue) != normalizedRootRoundID {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || strings.TrimSpace(slot.AgentID) != normalizedSourceAgentID {
				continue
			}
			return roomRootRoundID(roundValue), strings.TrimSpace(roundValue.RoundID), roundValue.HopIndex
		}
	}
	return normalizedRootRoundID, normalizedRootRoundID, 0
}
