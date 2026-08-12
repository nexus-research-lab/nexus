// INPUT: Room 队列项、目标 Agent 运行态与 conversation 上下文。
// OUTPUT: 串行队列接力，或把未消费 guide 恢复为下一轮输入。
// POS: Room 用户输入队列的数据面与 round 交接点。
package realtime

import (
	"cmp"
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) pruneStaleGoalCollaborationQueueEntries(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	entries []roomInputQueueEntry,
	currentGoal *protocol.Goal,
) ([]roomInputQueueEntry, error) {
	if s == nil || s.inputQueue == nil || contextValue == nil || len(entries) == 0 {
		return entries, nil
	}
	kept := make([]roomInputQueueEntry, 0, len(entries))
	changed := false
	for _, entry := range entries {
		binding := protocol.NormalizeGoalCollaborationBinding(
			entry.Item.GoalCollaborationBinding,
		)
		if binding == nil || roomGoalCollaborationBindingMatchesGoal(currentGoal, binding) {
			kept = append(kept, entry)
			continue
		}
		if err := s.markRoomQueueHandoffTerminalStatus(
			contextValue.Conversation.ID,
			entry.Item,
			"interrupted",
		); err != nil {
			return nil, err
		}
		if _, err := s.inputQueue.Delete(entry.Location, entry.Item.ID); err != nil {
			return nil, err
		}
		changed = true
		s.loggerFor(ctx).Info(
			"清理过期的 Room Goal collaboration queue",
			"goal_id", binding.GoalID,
			"objective_revision", binding.ObjectiveRevision,
			"item_id", entry.Item.ID,
			"handoff_id", entry.Item.HandoffID,
		)
	}
	if changed {
		if err := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			s.loggerFor(ctx).Warn("广播 Room Goal queue 清理快照失败", "session_key", sessionKey, "err", err)
		}
	}
	return kept, nil
}

func (s *Service) dispatchNextInputQueueItem(ctx context.Context, sessionKey string, roomID string, conversationID string) {
	if strings.TrimSpace(sessionKey) == "" {
		return
	}
	lease := s.lockRoomDispatch(sessionKey, conversationID)
	defer lease.Unlock()
	s.dispatchNextInputQueueItemLocked(ctx, sessionKey, roomID, conversationID)
}

// dispatchNextInputQueueItemLocked 在 conversation 派发闸门内接力下一条队列项。
func (s *Service) dispatchNextInputQueueItemLocked(ctx context.Context, sessionKey string, roomID string, conversationID string) {
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil || contextValue == nil {
		if err != nil {
			s.loggerFor(ctx).Error("读取 Room 待发送队列上下文失败", "session_key", sessionKey, "err", err)
		}
		return
	}
	if requireGroupRoomContext(contextValue) != nil {
		return
	}
	s.releaseUndeliveredRoomGuidanceLocked(ctx, sessionKey, contextValue)
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Error("读取 Room 待发送队列失败", "session_key", sessionKey, "err", err)
		return
	}
	entry, ok := s.findDispatchableInputQueueEntry(
		sessionKey,
		conversationID,
		contextValue.Members,
		entries,
	)
	if len(entries) == 0 || !ok {
		return
	}
	batch := isolatedRoomInputQueueDispatch(entry)
	itemIDs := make([]string, 0, len(batch))
	for _, candidate := range batch {
		itemIDs = append(itemIDs, candidate.Item.ID)
	}
	if _, err = s.inputQueue.DispatchMany(entry.Location, itemIDs); err != nil {
		s.loggerFor(ctx).Error("弹出 Room 待发送队列失败", "session_key", sessionKey, "err", err)
		return
	}
	dispatchedItem := entry.Item
	if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
		s.loggerFor(ctx).Warn("广播 Room 待发送队列快照失败", "session_key", sessionKey, "err", err)
	}
	err = s.dispatchInputQueueItemLocked(
		ctx,
		sessionKey,
		roomID,
		conversationID,
		entry.Location,
		dispatchedItem,
	)
	if err == nil {
		if s.canDispatchMoreInputQueueItems(ctx, sessionKey, conversationID) {
			s.startSessionBackgroundTask(
				sessionKey,
				contextValue.Room.OwnerUserID,
				func(taskCtx context.Context) {
					s.dispatchNextInputQueueItem(taskCtx, sessionKey, roomID, conversationID)
				},
			)
		}
		return
	}
	s.loggerFor(ctx).Error("派发 Room 待发送队列失败",
		"session_key", sessionKey,
		"room_id", roomID,
		"conversation_id", conversationID,
		"item_id", dispatchedItem.ID,
		"err", err,
	)
	invalidCapabilityEnvelope := errors.Is(err, protocol.ErrInvalidInputQueueCapabilityEnvelope)
	if !invalidCapabilityEnvelope {
		for _, candidate := range batch {
			if _, restoreErr := s.inputQueue.Enqueue(candidate.Location, candidate.Item); restoreErr != nil {
				s.loggerFor(ctx).Error("恢复 Room 待发送队列项失败",
					"session_key", sessionKey,
					"item_id", candidate.Item.ID,
					"err", restoreErr,
				)
			}
		}
	}
	if snapshotErr := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); snapshotErr != nil {
		s.loggerFor(ctx).Warn("广播恢复后的 Room 待发送队列快照失败", "session_key", sessionKey, "err", snapshotErr)
	}
	message := "待发送消息派发失败"
	if clientMessage, ok := protocol.ClientErrorMessage(err); ok {
		message = clientMessage
	}
	s.broadcastSharedEvent(ctx, sessionKey, roomID, roomdomain.NewErrorEvent(sessionKey, roomID, conversationID, "input_queue_error", message, dispatchedItem.ID))
	if invalidCapabilityEnvelope && s.canDispatchMoreInputQueueItems(ctx, sessionKey, conversationID) {
		s.startSessionBackgroundTask(
			sessionKey,
			contextValue.Room.OwnerUserID,
			func(taskCtx context.Context) {
				s.dispatchNextInputQueueItem(taskCtx, sessionKey, roomID, conversationID)
			},
		)
	}
}

func isolatedRoomInputQueueDispatch(
	selected roomInputQueueEntry,
) []roomInputQueueEntry {
	// Content, source message, handoff and capability envelope form one durable
	// identity. Combining adjacent directed messages would either drop content
	// or make one round ambiguously acknowledge several independent handoffs.
	return []roomInputQueueEntry{selected}
}

// releaseUndeliveredRoomGuidance 把错过最后一个 PostToolUse 的引导恢复成普通队列输入。
func (s *Service) releaseUndeliveredRoomGuidance(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) {
	if contextValue == nil {
		return
	}
	lease := s.lockRoomDispatch(sessionKey, contextValue.Conversation.ID)
	defer lease.Unlock()
	s.releaseUndeliveredRoomGuidanceLocked(ctx, sessionKey, contextValue)
}

// releaseUndeliveredRoomGuidanceLocked 统一恢复已失去目标 slot 的持久化引导。
// 调用方必须持有 conversation 派发闸门，避免恢复与新 round 启动交错。
func (s *Service) releaseUndeliveredRoomGuidanceLocked(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) {
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Error("读取 Room 未消费引导失败", "session_key", sessionKey, "err", err)
		return
	}
	changed := false
	for _, entry := range entries {
		if !protocol.ShouldGuideRunningRound(entry.Item.DeliveryPolicy) {
			continue
		}
		activeSlot := s.inputQueueGuidanceTargetSlot(sessionKey, contextValue.Conversation.ID, entry)
		boundRoundID := strings.TrimSpace(entry.Item.RootRoundID)
		if activeSlot != nil && (boundRoundID == "" || boundRoundID == strings.TrimSpace(activeSlot.AgentRoundID)) {
			continue
		}
		if _, err = s.inputQueue.UpdateDeliveryPolicy(entry.Location, entry.Item.ID, protocol.ChatDeliveryPolicyQueue); err != nil {
			s.loggerFor(ctx).Error("恢复 Room 未消费引导失败", "session_key", sessionKey, "item_id", entry.Item.ID, "err", err)
			continue
		}
		entry.Item.DeliveryPolicy = protocol.ChatDeliveryPolicyQueue
		if syncErr := s.syncQueuedPublicUserMessage(ctx, sessionKey, contextValue, entry.Item, "", false); syncErr != nil {
			s.loggerFor(ctx).Error("同步 Room 未消费引导展示状态失败",
				"session_key", sessionKey,
				"item_id", entry.Item.ID,
				"err", syncErr,
			)
		}
		changed = true
	}
	if changed {
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			s.loggerFor(ctx).Warn("广播 Room 未消费引导恢复快照失败", "session_key", sessionKey, "err", err)
		}
	}
}

// dispatchInputQueueItemLocked 在 conversation 派发闸门内消费已 claim 的队列项。
func (s *Service) dispatchInputQueueItemLocked(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	location workspacestore.InputQueueLocation,
	item protocol.InputQueueItem,
) error {
	if err := protocol.ValidateInputQueueCapabilityEnvelope(item); err != nil {
		return err
	}
	if binding := protocol.NormalizeGoalCollaborationBinding(
		item.GoalCollaborationBinding,
	); binding != nil {
		current, err := s.roomGoalCollaborationBindingIsCurrent(
			ctx,
			conversationID,
			binding,
		)
		if err != nil {
			return err
		}
		if !current {
			return s.markRoomQueueHandoffTerminalStatus(
				conversationID,
				item,
				"interrupted",
			)
		}
	}
	dispatchCtx := contextWithExactQueueOwner(ctx, item.OwnerUserID)
	claim, trustedQueue, err := s.claimTrustedRoomQueueAdmission(
		dispatchCtx,
		sessionKey,
		location,
		item,
	)
	if err != nil {
		return err
	}
	if trustedQueue {
		dispatchCtx = authctx.WithQueuedHumanPrincipalBinding(
			dispatchCtx,
			authctx.QueuedHumanPrincipalBinding{
				UserID:     claim.Principal.UserID,
				AuthMethod: claim.Principal.AuthMethod,
				SessionID:  claim.Principal.SessionID,
			},
		)
	}
	err = s.dispatchClaimedInputQueueItemLocked(
		dispatchCtx,
		sessionKey,
		roomID,
		conversationID,
		item,
		trustedQueue,
	)
	if err != nil {
		if trustedQueue {
			if releaseErr := s.queueTrust.Release(dispatchCtx, claim); releaseErr != nil {
				s.loggerFor(ctx).Error("释放 Room queue configuration admission 失败",
					"session_key", sessionKey,
					"item_id", item.ID,
					"err", releaseErr,
				)
			}
		}
		return err
	}
	if trustedQueue {
		if consumeErr := s.queueTrust.Consume(dispatchCtx, claim); consumeErr != nil {
			s.loggerFor(ctx).Error("收口 Room queue configuration admission 失败",
				"session_key", sessionKey,
				"item_id", item.ID,
				"err", consumeErr,
			)
		}
	}
	return nil
}

func (s *Service) dispatchClaimedInputQueueItemLocked(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
	trustedQueue bool,
) error {
	if item.Source == protocol.InputQueueSourceAgentPublicMention ||
		item.Source == protocol.InputQueueSourceAgentRoomMessage {
		return s.dispatchAgentWakeQueueItem(
			ctx,
			sessionKey,
			roomID,
			conversationID,
			item,
			protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		)
	}
	if strings.TrimSpace(item.SourceMessageID) != "" && len(inputQueueTargetAgentIDs(item)) > 0 {
		return s.dispatchRoomPublicTriggerQueueItem(
			ctx,
			sessionKey,
			roomID,
			conversationID,
			item,
			trustedQueue,
		)
	}
	// dispatchNextInputQueueItemLocked 已持有 conversation 派发闸门。
	return s.handleChatLocked(ctx, ChatRequest{
		SessionKey:                        sessionKey,
		RoomID:                            roomID,
		ConversationID:                    conversationID,
		Content:                           item.Content,
		Attachments:                       item.Attachments,
		TargetAgentIDs:                    inputQueueTargetAgentIDs(item),
		RoundID:                           "queue_" + item.ID,
		DeliveryPolicy:                    protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		ExecutionOrigin:                   "queue",
		trustedQueuedConfigurationContext: trustedQueue,
	})
}

func (s *Service) dispatchRoomPublicTriggerQueueItem(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
	trustedQueue bool,
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
	if err = s.syncQueuedPublicUserMessage(ctx, sessionKey, contextValue, item, "", true); err != nil {
		return err
	}
	sourceRoundID := roomInputQueueSourceRoundID(item)
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		wakes = append(wakes, publicMentionWake{
			HandoffID:     strings.TrimSpace(item.HandoffID),
			SourceAgentID: strings.TrimSpace(item.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       content,
			MessageID:     strings.TrimSpace(item.SourceMessageID),
			GoalCollaborationBinding: cloneGoalCollaborationBinding(
				item.GoalCollaborationBinding,
			),
		})
	}
	parentRound := &activeRoomRound{
		SessionKey:                  sessionKey,
		RoomID:                      cmp.Or(strings.TrimSpace(roomID), contextValue.Room.ID),
		ConversationID:              conversationID,
		RoomType:                    contextValue.Room.RoomType,
		Context:                     contextValue,
		RoundID:                     sourceRoundID,
		RootRoundID:                 cmp.Or(strings.TrimSpace(item.RootRoundID), sourceRoundID),
		HopIndex:                    item.HopIndex,
		OwnerUserID:                 strings.TrimSpace(item.OwnerUserID),
		pendingTrustedQueueDispatch: trustedQueue,
	}
	return s.startPublicMentionRoundLocked(ctx, parentRound, wakes)
}

func (s *Service) dispatchAgentWakeQueueItem(
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
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	// Goal collaboration must remain an independently attributable target
	// round. Folding it into an unrelated active slot would lose its single
	// handoff identity and can terminalize the ledger before Goal handback.
	if protocol.NormalizeGoalCollaborationBinding(item.GoalCollaborationBinding) == nil &&
		protocol.ShouldGuideRunningRound(deliveryPolicy) {
		guidedItem := item
		guidedItem.ID = "queue_" + item.ID
		guidedAgentIDs, err := s.guideActiveAgentSlots(
			ctx,
			sessionKey,
			roomID,
			conversationID,
			targetAgentIDs,
			guidedItem,
		)
		if err != nil {
			return err
		}
		if len(guidedAgentIDs) > 0 {
			if err = s.markRoomQueueHandoffTerminal(conversationID, item); err != nil {
				return err
			}
			if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
				return err
			}
			s.broadcastSessionStatus(ctx, sessionKey)
			return nil
		}
	}
	rootRoundID := s.logicalPublicHandoffRootRoundID(conversationID, item, item.RootRoundID)
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		wakes = append(wakes, publicMentionWake{
			HandoffID:     strings.TrimSpace(item.HandoffID),
			TriggerType:   inputQueueWakeTriggerType(item),
			QueueSource:   protocol.NormalizeInputQueueSource(string(item.Source)),
			SourceAgentID: strings.TrimSpace(item.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       content,
			MessageID:     cmp.Or(strings.TrimSpace(item.SourceMessageID), "queue_"+item.ID),
			ReplyRoute:    item.ReplyRoute,
			GoalCollaborationBinding: cloneGoalCollaborationBinding(
				item.GoalCollaborationBinding,
			),
			WorkBinding:   cloneExecutionWorkBinding(item.WorkBinding),
			ReviewBinding: cloneExecutionReviewBinding(item.ReviewBinding),
		})
	}
	coordinatorAgentID := roomCoordinatorAgentID(item.SourceAgentID, contextValue)
	if item.ReviewBinding != nil {
		coordinatorAgentID = strings.TrimSpace(item.ReviewBinding.TargetAgentID)
	}
	parentRound := &activeRoomRound{
		SessionKey:         sessionKey,
		RoomID:             cmp.Or(strings.TrimSpace(roomID), contextValue.Room.ID),
		ConversationID:     conversationID,
		CoordinatorAgentID: coordinatorAgentID,
		RoomType:           contextValue.Room.RoomType,
		Context:            contextValue,
		RoundID:            cmp.Or(strings.TrimSpace(item.SourceMessageID), "queue_"+item.ID),
		RootRoundID:        cmp.Or(rootRoundID, strings.TrimSpace(item.SourceMessageID), "queue_"+item.ID),
		HopIndex:           item.HopIndex,
		OwnerUserID:        strings.TrimSpace(item.OwnerUserID),
	}
	return s.startPublicMentionRoundLocked(ctx, parentRound, wakes)
}

// logicalPublicHandoffRootRoundID 从 ledger 取回稳定 root；InputQueue 的
// RootRoundID 在 guide 期间可能暂时被改成目标 busy slot 的绑定 round。
func (s *Service) logicalPublicHandoffRootRoundID(
	conversationID string,
	item protocol.InputQueueItem,
	fallback string,
) string {
	rootRoundID := strings.TrimSpace(fallback)
	if (item.Source != protocol.InputQueueSourceAgentPublicMention &&
		!(item.Source == protocol.InputQueueSourceAgentRoomMessage &&
			protocol.NormalizeGoalCollaborationBinding(item.GoalCollaborationBinding) != nil)) ||
		s == nil || s.publicHandoffs == nil || strings.TrimSpace(item.HandoffID) == "" {
		return rootRoundID
	}
	handoff, ok, err := s.publicHandoffs.Get(item.OwnerUserID, conversationID, item.HandoffID)
	if err == nil && ok && strings.TrimSpace(handoff.RootRoundID) != "" {
		return strings.TrimSpace(handoff.RootRoundID)
	}
	return rootRoundID
}

func inputQueueWakeTriggerType(item protocol.InputQueueItem) string {
	if item.ReviewBinding != nil {
		return "execution_review_return"
	}
	if item.WorkBinding != nil {
		return "execution_dispatch"
	}
	if item.Source == protocol.InputQueueSourceAgentRoomMessage {
		return "room_directed_message"
	}
	return "public_mention"
}

func (s *Service) canDispatchInputQueueItem(
	sessionKey string,
	conversationID string,
	members []protocol.MemberRecord,
	item protocol.InputQueueItem,
) bool {
	if protocol.ShouldGuideRunningRound(item.DeliveryPolicy) {
		return false
	}
	targetAgentIDs := inputQueueTargetAgentIDs(item)
	if len(targetAgentIDs) > 0 {
		participatingAgentIDs, pausedAgentIDs := partitionRoomParticipationTargets(
			members,
			targetAgentIDs,
		)
		if len(pausedAgentIDs) > 0 || len(participatingAgentIDs) == 0 {
			return false
		}
		return len(s.findActiveDeliverySlotsByAgent(sessionKey, conversationID, participatingAgentIDs)) == 0
	}
	return len(s.runtime.GetRunningRoundIDs(sessionKey)) == 0
}

func (s *Service) findDispatchableInputQueueEntry(
	sessionKey string,
	conversationID string,
	members []protocol.MemberRecord,
	entries []roomInputQueueEntry,
) (roomInputQueueEntry, bool) {
	for _, entry := range entries {
		if protocol.ShouldGuideRunningRound(entry.Item.DeliveryPolicy) {
			continue
		}
		if s.canDispatchInputQueueItem(
			sessionKey,
			conversationID,
			members,
			entry.Item,
		) {
			return entry, true
		}
	}
	return roomInputQueueEntry{}, false
}

func (s *Service) canDispatchMoreInputQueueItems(ctx context.Context, sessionKey string, conversationID string) bool {
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil || contextValue == nil {
		return false
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil || len(entries) == 0 {
		return false
	}
	_, ok := s.findDispatchableInputQueueEntry(
		sessionKey,
		conversationID,
		contextValue.Members,
		entries,
	)
	return ok
}

func (s *Service) inputQueueGuidanceTargetSlot(
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
