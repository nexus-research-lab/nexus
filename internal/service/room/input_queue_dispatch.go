// INPUT: Room 队列项、目标 Agent 运行态与 conversation 上下文。
// OUTPUT: 串行队列接力，或把未消费 guide 恢复为下一轮输入。
// POS: Room 用户输入队列的数据面与 round 交接点。
package room

import (
	"cmp"
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) dispatchNextInputQueueItem(ctx context.Context, sessionKey string, roomID string, conversationID string) {
	if strings.TrimSpace(sessionKey) == "" {
		return
	}
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()

	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil || contextValue == nil {
		if err != nil {
			s.loggerFor(ctx).Error("读取 Room 待发送队列上下文失败", "session_key", sessionKey, "err", err)
		}
		return
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Error("读取 Room 待发送队列失败", "session_key", sessionKey, "err", err)
		return
	}
	entry, ok := s.findDispatchableInputQueueEntry(sessionKey, conversationID, entries)
	if len(entries) == 0 || !ok {
		return
	}
	if _, err = s.inputQueue.Dispatch(entry.Location, entry.Item.ID); err != nil {
		s.loggerFor(ctx).Error("弹出 Room 待发送队列失败", "session_key", sessionKey, "err", err)
		return
	}
	if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
		s.loggerFor(ctx).Warn("广播 Room 待发送队列快照失败", "session_key", sessionKey, "err", err)
	}
	err = s.dispatchInputQueueItem(ctx, sessionKey, roomID, conversationID, entry.Item)
	if err == nil {
		if s.canDispatchMoreInputQueueItems(ctx, sessionKey, conversationID) {
			go s.dispatchNextInputQueueItem(ctx, sessionKey, roomID, conversationID)
		}
		return
	}
	s.loggerFor(ctx).Error("派发 Room 待发送队列失败",
		"session_key", sessionKey,
		"room_id", roomID,
		"conversation_id", conversationID,
		"item_id", entry.Item.ID,
		"err", err,
	)
	if _, restoreErr := s.inputQueue.Enqueue(entry.Location, entry.Item); restoreErr != nil {
		s.loggerFor(ctx).Error("恢复 Room 待发送队列项失败",
			"session_key", sessionKey,
			"item_id", entry.Item.ID,
			"err", restoreErr,
		)
	} else if snapshotErr := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); snapshotErr != nil {
		s.loggerFor(ctx).Warn("广播恢复后的 Room 待发送队列快照失败", "session_key", sessionKey, "err", snapshotErr)
	}
	s.broadcastSharedEvent(ctx, sessionKey, roomID, roomdomain.NewErrorEvent(sessionKey, roomID, conversationID, "input_queue_error", "待发送消息派发失败", entry.Item.ID))
}

// releaseUndeliveredRoomGuidance 把错过最后一个 PostToolUse 的引导恢复成普通队列输入。
func (s *RealtimeService) releaseUndeliveredRoomGuidance(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) {
	if contextValue == nil {
		return
	}
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()

	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Error("读取 Room 未消费引导失败", "session_key", sessionKey, "err", err)
		return
	}
	changed := false
	for _, entry := range entries {
		if !protocol.ShouldGuideRunningRound(entry.Item.DeliveryPolicy) ||
			len(s.findActiveDeliverySlots(sessionKey, contextValue.Conversation.ID, inputQueueTargetAgentIDs(entry.Item))) > 0 {
			continue
		}
		if _, err = s.inputQueue.UpdateDeliveryPolicy(entry.Location, entry.Item.ID, protocol.ChatDeliveryPolicyQueue); err != nil {
			s.loggerFor(ctx).Error("恢复 Room 未消费引导失败", "session_key", sessionKey, "item_id", entry.Item.ID, "err", err)
			continue
		}
		entry.Item.DeliveryPolicy = protocol.ChatDeliveryPolicyQueue
		s.syncQueuedPublicMessageDeliveryPolicy(ctx, sessionKey, contextValue, entry.Item)
		changed = true
	}
	if changed {
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			s.loggerFor(ctx).Warn("广播 Room 未消费引导恢复快照失败", "session_key", sessionKey, "err", err)
		}
	}
}

func (s *RealtimeService) dispatchInputQueueItem(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
) error {
	if item.Source == protocol.InputQueueSourceAgentPublicMention ||
		item.Source == protocol.InputQueueSourceAgentRoomMessage {
		return s.dispatchAgentWakeQueueItem(
			contextWithQueueOwner(ctx, item.OwnerUserID),
			sessionKey,
			roomID,
			conversationID,
			item,
			protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		)
	}
	if strings.TrimSpace(item.SourceMessageID) != "" && len(inputQueueTargetAgentIDs(item)) > 0 {
		return s.dispatchRoomPublicTriggerQueueItem(
			contextWithQueueOwner(ctx, item.OwnerUserID),
			sessionKey,
			roomID,
			conversationID,
			item,
		)
	}
	return s.HandleChat(contextWithQueueOwner(ctx, item.OwnerUserID), ChatRequest{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		Content:        item.Content,
		Attachments:    item.Attachments,
		RoundID:        "queue_" + item.ID,
		DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
	})
}

func (s *RealtimeService) dispatchRoomPublicTriggerQueueItem(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
) error {
	targetAgentIDs := inputQueueTargetAgentIDs(item)
	if len(targetAgentIDs) == 0 {
		return errors.New("target_agent_ids is required")
	}
	content := strings.TrimSpace(item.Content)
	if content == "" {
		return errors.New("content is required")
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		wakes = append(wakes, publicMentionWake{
			SourceAgentID: strings.TrimSpace(item.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       content,
			MessageID:     strings.TrimSpace(item.SourceMessageID),
		})
	}
	parentRound := &activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         cmp.Or(strings.TrimSpace(roomID), contextValue.Room.ID),
		ConversationID: conversationID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        strings.TrimSpace(item.SourceMessageID),
		RootRoundID:    cmp.Or(strings.TrimSpace(item.RootRoundID), strings.TrimSpace(item.SourceMessageID)),
		HopIndex:       item.HopIndex,
		OwnerUserID:    strings.TrimSpace(item.OwnerUserID),
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}

func (s *RealtimeService) dispatchAgentWakeQueueItem(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
	deliveryPolicy protocol.ChatDeliveryPolicy,
) error {
	targetAgentIDs := inputQueueTargetAgentIDs(item)
	if len(targetAgentIDs) == 0 {
		return errors.New("target_agent_ids is required")
	}
	content := strings.TrimSpace(item.Content)
	if content == "" {
		return errors.New("content is required")
	}
	if protocol.ShouldGuideRunningRound(deliveryPolicy) {
		runtimeContent, renderErr := s.renderRuntimeContentWithAttachments(ctx, content, item.Attachments)
		if renderErr != nil {
			return renderErr
		}
		guidedAgentIDs, err := s.guideActiveAgentSlots(ctx, sessionKey, roomID, conversationID, targetAgentIDs, content, runtimeContent.PlainText(), "queue_"+item.ID)
		if err != nil {
			return err
		}
		if len(guidedAgentIDs) > 0 {
			s.broadcastSessionStatus(ctx, sessionKey)
			return nil
		}
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		wakes = append(wakes, publicMentionWake{
			TriggerType:   inputQueueWakeTriggerType(item),
			QueueSource:   protocol.NormalizeInputQueueSource(string(item.Source)),
			SourceAgentID: strings.TrimSpace(item.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       content,
			MessageID:     cmp.Or(strings.TrimSpace(item.SourceMessageID), "queue_"+item.ID),
			ReplyRoute:    item.ReplyRoute,
		})
	}
	parentRound := &activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         cmp.Or(strings.TrimSpace(roomID), contextValue.Room.ID),
		ConversationID: conversationID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        cmp.Or(strings.TrimSpace(item.SourceMessageID), "queue_"+item.ID),
		RootRoundID:    strings.TrimSpace(item.RootRoundID),
		HopIndex:       item.HopIndex,
		OwnerUserID:    strings.TrimSpace(item.OwnerUserID),
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}

func inputQueueWakeTriggerType(item protocol.InputQueueItem) string {
	if item.Source == protocol.InputQueueSourceAgentRoomMessage {
		return "room_directed_message"
	}
	return "public_mention"
}

func (s *RealtimeService) canDispatchInputQueueItem(sessionKey string, conversationID string, item protocol.InputQueueItem) bool {
	if protocol.ShouldGuideRunningRound(item.DeliveryPolicy) {
		return false
	}
	targetAgentIDs := inputQueueTargetAgentIDs(item)
	if len(targetAgentIDs) > 0 {
		return len(s.findActiveDeliverySlots(sessionKey, conversationID, targetAgentIDs)) == 0
	}
	return len(s.runtime.GetRunningRoundIDs(sessionKey)) == 0
}

func (s *RealtimeService) findDispatchableInputQueueEntry(
	sessionKey string,
	conversationID string,
	entries []roomInputQueueEntry,
) (roomInputQueueEntry, bool) {
	for _, entry := range entries {
		if protocol.ShouldGuideRunningRound(entry.Item.DeliveryPolicy) {
			continue
		}
		if s.canDispatchInputQueueItem(sessionKey, conversationID, entry.Item) {
			return entry, true
		}
	}
	return roomInputQueueEntry{}, false
}

func (s *RealtimeService) canDispatchMoreInputQueueItems(ctx context.Context, sessionKey string, conversationID string) bool {
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil || contextValue == nil {
		return false
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil || len(entries) == 0 {
		return false
	}
	_, ok := s.findDispatchableInputQueueEntry(sessionKey, conversationID, entries)
	return ok
}

func (s *RealtimeService) inputQueueGuidanceTargetSlot(
	sessionKey string,
	conversationID string,
	entry roomInputQueueEntry,
) *activeRoomSlot {
	slotsByAgentID := s.findActiveDeliverySlots(sessionKey, conversationID, inputQueueTargetAgentIDs(entry.Item))
	if len(slotsByAgentID) == 0 {
		return nil
	}
	if slot := slotsByAgentID[inputQueueLocationAgentID(entry.Location)]; slot != nil {
		return slot
	}
	for _, slot := range slotsByAgentID {
		if slot != nil {
			return slot
		}
	}
	return nil
}
