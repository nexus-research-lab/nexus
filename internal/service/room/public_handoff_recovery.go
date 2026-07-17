// INPUT: 进程启动时 handoff ledger 中尚未完成的 source_finished 记录。
// OUTPUT: 重新进入统一 busy/idle 派发路径的 target wake。
// POS: Room 公区协作的 durable recovery 边界。
package room

import (
	"context"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// StartPublicHandoffReconciler 恢复进程退出前已确认 source 成功但尚未启动的 handoff。
func (s *RealtimeService) StartPublicHandoffReconciler(ctx context.Context) (func(), error) {
	if s == nil || s.publicHandoffs == nil || s.rooms == nil {
		return nil, nil
	}
	pending, err := s.publicHandoffs.PendingAll()
	if err != nil {
		return nil, err
	}
	for _, handoff := range pending {
		if err := s.reconcilePublicHandoff(ctx, handoff); err != nil {
			s.loggerFor(ctx).Warn("恢复 Room 公区 handoff 失败",
				"conversation_id", handoff.ConversationID,
				"handoff_id", handoff.HandoffID,
				"err", err,
			)
		}
	}
	return nil, nil
}

func (s *RealtimeService) reconcilePublicHandoff(ctx context.Context, handoff workspacestore.RoomPublicHandoff) error {
	conversationID := strings.TrimSpace(handoff.ConversationID)
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	if contextValue == nil {
		return nil
	}
	if !roomdomain.IsMemberAgent(contextValue.Members, handoff.TargetAgentID) {
		return s.publicHandoffs.MarkTerminal(conversationID, handoff.HandoffID, "error")
	}
	if handoff.Status == "queued" {
		present, queueErr := s.publicHandoffQueueItemPresent(ctx, contextValue, handoff)
		if queueErr != nil {
			return queueErr
		}
		if present {
			// 队列项仍然是 durable 真相；让正常队列恢复负责出队，
			// 不在这里再创建一条 target round。
			if s.inputQueue != nil {
				go s.dispatchNextInputQueueItem(
					contextWithQueueOwner(context.Background(), ""),
					protocol.BuildRoomSharedSessionKey(conversationID),
					contextValue.Room.ID,
					conversationID,
				)
			}
			return nil
		}
		// 出队与 target 启动之间崩溃时，队列项已经不存在；
		// 将 handoff 重新暴露为可 claim 的 source_finished。
		if err := s.publicHandoffs.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	if handoff.Status == "detected" {
		if s.roomHistory == nil {
			return nil
		}
		messages, readErr := s.roomHistory.ReadMessages(conversationID, nil)
		if readErr != nil {
			return readErr
		}
		if !roomHistoryContainsMessage(messages, handoff.SourceMessageID) {
			// detected 可能落在 source transcript 写入前的崩溃窗口；
			// 保留 ledger，下一次启动或 source 收尾路径继续处理。
			return nil
		}
		if err := s.publicHandoffs.MarkSourceFinished(conversationID, handoff.HandoffID); err != nil {
			return err
		}
		handoff.Status = "source_finished"
	}
	parentRound := &activeRoomRound{
		SessionKey:     protocol.BuildRoomSharedSessionKey(conversationID),
		RoomID:         contextValue.Room.ID,
		ConversationID: conversationID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        strings.TrimSpace(handoff.SourceMessageID),
		RootRoundID:    strings.TrimSpace(handoff.RootRoundID),
		HopIndex:       handoff.HopIndex,
		Slots:          make(map[string]*activeRoomSlot),
	}
	wake := publicMentionWake{
		HandoffID:     handoff.HandoffID,
		TriggerType:   "public_mention",
		QueueSource:   protocol.InputQueueSourceAgentPublicMention,
		SourceAgentID: handoff.SourceAgentID,
		TargetAgentID: handoff.TargetAgentID,
		Content:       handoff.Content,
		MessageID:     handoff.SourceMessageID,
		ReplyRoute:    handoff.ReplyRoute,
	}
	return s.startPublicMentionRound(ctx, parentRound, []publicMentionWake{wake})
}

func (s *RealtimeService) publicHandoffQueueItemPresent(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	handoff workspacestore.RoomPublicHandoff,
) (bool, error) {
	if s.inputQueue == nil || contextValue == nil {
		return false, nil
	}
	locations, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return false, err
	}
	location, ok := locations[strings.TrimSpace(handoff.TargetAgentID)]
	if !ok {
		return false, nil
	}
	items, err := s.inputQueue.Snapshot(location.Location)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if strings.TrimSpace(item.HandoffID) == strings.TrimSpace(handoff.HandoffID) ||
			(strings.TrimSpace(handoff.QueueItemID) != "" && item.ID == handoff.QueueItemID) {
			return true, nil
		}
	}
	return false, nil
}

func roomHistoryContainsMessage(messages []protocol.Message, messageID string) bool {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return false
	}
	for _, message := range messages {
		if strings.TrimSpace(anyString(message["message_id"])) == messageID {
			return true
		}
	}
	return false
}
