// INPUT: Room 输入队列控制请求与持久化队列快照。
// OUTPUT: 队列变更、guide 消费轮身份同步和共享快照事件。
// POS: Room 用户输入队列的控制面。
package room

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// InputQueueRequest 表示 Room 待发送队列控制请求。
type InputQueueRequest struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	Action         string
	ItemID         string
	Content        string
	Attachments    []protocol.ChatAttachment
	TargetAgentIDs []string
	OrderedIDs     []string
	DeliveryPolicy protocol.ChatDeliveryPolicy
}

type roomInputQueueLocation struct {
	AgentID  string
	Location workspacestore.InputQueueLocation
}

type roomInputQueueEntry struct {
	Item     protocol.InputQueueItem
	Location workspacestore.InputQueueLocation
}

func inputQueueTargetAgentIDs(item protocol.InputQueueItem) []string {
	targets := make([]string, 0, len(item.TargetAgentIDs)+1)
	seen := make(map[string]struct{}, len(item.TargetAgentIDs)+1)
	appendTarget := func(agentID string) {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			return
		}
		if _, exists := seen[agentID]; exists {
			return
		}
		seen[agentID] = struct{}{}
		targets = append(targets, agentID)
	}
	appendTarget(item.AgentID)
	for _, agentID := range item.TargetAgentIDs {
		appendTarget(agentID)
	}
	return targets
}

func inputQueueLocationAgentID(location workspacestore.InputQueueLocation) string {
	return strings.TrimSpace(protocol.ParseSessionKey(location.SessionKey).AgentID)
}

func inputQueueLocationKey(location workspacestore.InputQueueLocation) string {
	return strings.TrimSpace(location.WorkspacePath) + "::" + strings.TrimSpace(location.SessionKey)
}

func contextWithQueueOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return ctx
	}
	if _, ok := authctx.CurrentUserID(ctx); ok {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	})
}

func (s *RealtimeService) broadcastRoomInputQueueSnapshot(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) error {
	items, err := s.roomInputQueueItems(ctx, contextValue)
	if err != nil {
		return err
	}
	s.broadcastInputQueueItems(ctx, sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, items)
	return nil
}

func (s *RealtimeService) broadcastInputQueueItems(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	items []protocol.InputQueueItem,
) {
	s.broadcastSharedEvent(ctx, sessionKey, roomID, newRoomInputQueueEvent(sessionKey, roomID, conversationID, items))
}

func newRoomInputQueueEvent(sessionKey string, roomID string, conversationID string, items []protocol.InputQueueItem) protocol.EventMessage {
	event := protocol.NewInputQueueEvent(sessionKey, items)
	event.Data["scope"] = string(protocol.InputQueueScopeRoom)
	event.RoomID = strings.TrimSpace(roomID)
	event.ConversationID = strings.TrimSpace(conversationID)
	return event
}

// HandleInputQueue 处理 Room 待发送队列控制消息。
func (s *RealtimeService) HandleInputQueue(ctx context.Context, request InputQueueRequest) error {
	sessionKey, contextValue, err := s.resolveInputQueueContext(ctx, request)
	if err != nil {
		return err
	}

	action := strings.TrimSpace(request.Action)
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()
	switch action {
	case "enqueue", "":
		content := strings.TrimSpace(request.Content)
		attachments := s.normalizeChatAttachments(request.Attachments, "", contextValue.Room.ID, contextValue.Conversation.ID)
		if !protocol.HasChatInput(content, attachments) {
			return errors.New("content is required")
		}
		location, targetAgentIDs, err := s.resolveRoomInputQueuePrimaryLocation(
			ctx,
			contextValue,
			content,
			request.TargetAgentIDs,
		)
		if err != nil {
			return err
		}
		ownerUserID := authctx.OwnerUserID(ctx)
		if _, err = s.inputQueue.Enqueue(location, protocol.InputQueueItem{
			Scope:          protocol.InputQueueScopeRoom,
			SessionKey:     location.SessionKey,
			RoomID:         contextValue.Room.ID,
			ConversationID: contextValue.Conversation.ID,
			AgentID:        inputQueueLocationAgentID(location),
			TargetAgentIDs: targetAgentIDs,
			Source:         protocol.InputQueueSourceUser,
			Content:        content,
			Attachments:    attachments,
			DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy)),
			OwnerUserID:    ownerUserID,
		}); err != nil {
			return err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return err
		}
		go s.dispatchNextInputQueueItem(contextWithQueueOwner(context.Background(), ownerUserID), sessionKey, contextValue.Room.ID, contextValue.Conversation.ID)
		return nil
	case "delete":
		if s.hasInFlightRoomGuidance(request.ItemID) {
			return errors.New("该引导已发送给智能体，不能再删除")
		}
		if err = s.deleteRoomInputQueueItem(ctx, contextValue, request.ItemID); err != nil {
			return err
		}
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	case "reorder":
		for _, itemID := range request.OrderedIDs {
			if s.hasInFlightRoomGuidance(itemID) {
				return errors.New("已发送给智能体的引导不能重排")
			}
		}
		if err = s.reorderRoomInputQueueItems(ctx, contextValue, request.OrderedIDs); err != nil {
			return err
		}
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	case "guide":
		if s.hasInFlightRoomGuidance(request.ItemID) {
			return errors.New("该引导正在等待智能体确认，不能更改投递方式")
		}
		return s.guideInputQueueItem(ctx, sessionKey, contextValue, request.ItemID)
	default:
		return errors.New("unsupported input_queue action")
	}
}

// InputQueueSnapshotEvent 构造 Room 队列快照事件，供新订阅连接恢复状态。
func (s *RealtimeService) InputQueueSnapshotEvent(
	ctx context.Context,
	roomID string,
	conversationID string,
) (protocol.EventMessage, error) {
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return protocol.EventMessage{}, err
	}
	if contextValue == nil {
		return protocol.EventMessage{}, errors.New("room conversation not found")
	}
	s.inputQueueDispatchMu.Lock()
	s.releaseUndeliveredRoomGuidanceLocked(ctx, sessionKey, contextValue)
	items, err := s.roomInputQueueItems(ctx, contextValue)
	s.inputQueueDispatchMu.Unlock()
	if err != nil {
		return protocol.EventMessage{}, err
	}
	event := newRoomInputQueueEvent(sessionKey, strings.TrimSpace(roomID), strings.TrimSpace(conversationID), items)
	go s.dispatchNextInputQueueItem(ctx, sessionKey, roomID, conversationID)
	return event, nil
}

func (s *RealtimeService) guideInputQueueItem(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	itemID string,
) error {
	entry, ok, err := s.findRoomInputQueueEntry(ctx, contextValue, itemID)
	if err != nil {
		return err
	}
	if !ok {
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	}
	if protocol.ShouldGuideRunningRound(entry.Item.DeliveryPolicy) {
		if _, err = s.inputQueue.UpdateDeliveryPolicy(entry.Location, entry.Item.ID, protocol.ChatDeliveryPolicyQueue); err != nil {
			return err
		}
		entry.Item.DeliveryPolicy = protocol.ChatDeliveryPolicyQueue
		if err = s.syncQueuedPublicUserMessage(ctx, sessionKey, contextValue, entry.Item, "", false); err != nil {
			return err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return err
		}
		go s.dispatchNextInputQueueItem(
			contextWithQueueOwner(context.Background(), entry.Item.OwnerUserID),
			sessionKey,
			contextValue.Room.ID,
			contextValue.Conversation.ID,
		)
		return nil
	}
	activeSlot := s.inputQueueGuidanceTargetSlot(sessionKey, contextValue.Conversation.ID, entry)
	if activeSlot == nil {
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	}
	if _, err = s.inputQueue.UpdateDeliveryPolicy(
		entry.Location,
		entry.Item.ID,
		protocol.ChatDeliveryPolicyGuide,
		activeSlot.AgentRoundID,
	); err != nil {
		return err
	}
	entry.Item.DeliveryPolicy = protocol.ChatDeliveryPolicyGuide
	entry.Item.RootRoundID = activeSlot.AgentRoundID
	if err = s.syncQueuedPublicUserMessage(ctx, sessionKey, contextValue, entry.Item, "", false); err != nil {
		return err
	}
	return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
}

func (s *RealtimeService) syncQueuedPublicUserMessage(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	item protocol.InputQueueItem,
	rootRoundID string,
	materialize bool,
) error {
	if contextValue == nil || s.roomHistory == nil {
		return nil
	}
	sourceRoundID := roomInputQueueSourceRoundID(item)
	rootRoundID = strings.TrimSpace(rootRoundID)
	targetAgentIDs := inputQueueTargetAgentIDs(item)
	consumingAgentRoundID := ""
	if materialize && protocol.ShouldGuideRunningRound(item.DeliveryPolicy) && len(targetAgentIDs) == 1 {
		consumingAgentRoundID = strings.TrimSpace(item.RootRoundID)
	}
	userMessageID := strings.TrimSpace(item.SourceMessageID)
	if userMessageID == "" {
		userMessageID = "msg_user_" + sourceRoundID
	}
	messages, err := s.roomHistory.ReadMessages(contextValue.Conversation.ID, nil)
	if err != nil {
		return err
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		messageRoundID := protocol.MessageRoundID(message)
		messageSourceRoundID, _ := message["source_round_id"].(string)
		if protocol.MessageRole(message) != "user" ||
			(message["message_id"] != userMessageID && messageRoundID != sourceRoundID &&
				strings.TrimSpace(messageSourceRoundID) != sourceRoundID) {
			continue
		}
		updated := protocol.Clone(message)
		updated["delivery_policy"] = string(item.DeliveryPolicy)
		messageTargets := roomMessageTargetAgentIDs(message["target_agent_ids"])
		updatedTargets := mergeRoomMessageTargetAgentIDs(messageTargets, targetAgentIDs)
		if len(updatedTargets) > 0 {
			updated["target_agent_ids"] = updatedTargets
		}
		messageAgentRoundID, _ := message["agent_round_id"].(string)
		messageAgentRoundID = strings.TrimSpace(messageAgentRoundID)
		if len(updatedTargets) > 1 {
			delete(updated, "agent_round_id")
		} else if consumingAgentRoundID != "" && messageAgentRoundID == "" {
			updated["agent_round_id"] = consumingAgentRoundID
		}
		annotateRoomUserMessage(contextValue, updated)
		// 第一位消费者确定公开用户消息的归组；其他 root 只聚合消费目标，
		// 不能让同一条消息在时间线中随最后完成的 Agent 来回移动。
		if rootRoundID != "" && rootRoundID != sourceRoundID &&
			strings.TrimSpace(messageSourceRoundID) == "" &&
			(messageRoundID == "" || messageRoundID == sourceRoundID) {
			updated["source_round_id"] = sourceRoundID
			updated["round_id"] = rootRoundID
		}
		messagePolicy, _ := message["delivery_policy"].(string)
		updatedPolicy, _ := updated["delivery_policy"].(string)
		updatedSourceRoundID, _ := updated["source_round_id"].(string)
		updatedAgentRoundID, _ := updated["agent_round_id"].(string)
		updatedAgentRoundID = strings.TrimSpace(updatedAgentRoundID)
		if protocol.MessageRoundID(updated) == messageRoundID &&
			strings.TrimSpace(messagePolicy) == strings.TrimSpace(updatedPolicy) &&
			strings.TrimSpace(messageSourceRoundID) == strings.TrimSpace(updatedSourceRoundID) &&
			messageAgentRoundID == updatedAgentRoundID &&
			slices.Equal(messageTargets, updatedTargets) {
			return nil
		}
		if err = s.persistSharedInlineMessage(contextValue.Conversation.ID, updated); err != nil {
			return err
		}
		s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapMessageEvent(
			contextValue.Room.ID,
			contextValue.Conversation.ID,
			updated,
			protocol.MessageRoundID(updated),
		))
		return nil
	}
	if !materialize || item.Source != protocol.InputQueueSourceUser || sourceRoundID == "" {
		return nil
	}
	messageRoundID := sourceRoundID
	messageValue := protocol.Message{
		"message_id":      userMessageID,
		"session_key":     strings.TrimSpace(sessionKey),
		"room_id":         contextValue.Room.ID,
		"conversation_id": contextValue.Conversation.ID,
		"agent_id":        "",
		"round_id":        sourceRoundID,
		"role":            "user",
		"content":         strings.TrimSpace(item.Content),
		"timestamp":       time.Now().UnixMilli(),
		"delivery_policy": string(item.DeliveryPolicy),
	}
	if rootRoundID != "" && rootRoundID != sourceRoundID {
		messageRoundID = rootRoundID
		messageValue["source_round_id"] = sourceRoundID
		messageValue["round_id"] = rootRoundID
	}
	if consumingAgentRoundID != "" {
		messageValue["agent_round_id"] = consumingAgentRoundID
	}
	if len(targetAgentIDs) > 0 {
		messageValue["target_agent_ids"] = targetAgentIDs
	}
	annotateRoomUserMessage(contextValue, messageValue)
	if attachments := protocol.NormalizeChatAttachments(item.Attachments, ""); len(attachments) > 0 {
		messageValue["attachments"] = attachments
	}
	if err = s.persistSharedInlineMessage(contextValue.Conversation.ID, messageValue); err != nil {
		return err
	}
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapMessageEvent(
		contextValue.Room.ID,
		contextValue.Conversation.ID,
		messageValue,
		messageRoundID,
	))
	return nil
}

func roomInputQueueSourceRoundID(item protocol.InputQueueItem) string {
	if itemID := strings.TrimSpace(item.ID); itemID != "" {
		if strings.TrimSpace(item.SourceMessageID) != "" {
			return itemID
		}
		return "queue_" + itemID
	}
	return strings.TrimSpace(item.SourceMessageID)
}

func roomMessageTargetAgentIDs(value any) []string {
	result := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		result = append(result, typed...)
	case []any:
		for _, item := range typed {
			if agentID, ok := item.(string); ok {
				result = append(result, agentID)
			}
		}
	}
	return mergeRoomMessageTargetAgentIDs(nil, result)
}

func mergeRoomMessageTargetAgentIDs(current []string, incoming []string) []string {
	result := make([]string, 0, len(current)+len(incoming))
	seen := make(map[string]struct{}, len(current)+len(incoming))
	for _, values := range [][]string{current, incoming} {
		for _, agentID := range values {
			agentID = strings.TrimSpace(agentID)
			if agentID == "" {
				continue
			}
			if _, ok := seen[agentID]; ok {
				continue
			}
			seen[agentID] = struct{}{}
			result = append(result, agentID)
		}
	}
	return result
}
