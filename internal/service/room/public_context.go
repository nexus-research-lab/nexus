package room

import (
	"context"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) buildSlotVisibleContext(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	publicHistory []protocol.Message,
	agentNameByID map[string]string,
) (string, error) {
	batch, err := s.publicInputBatchForSlot(ctx, roundValue, slot, publicHistory, roomdomain.PublicCursor{}, false)
	if err != nil {
		return "", err
	}
	runtimeMessages, err := s.renderRuntimeAttachmentMessages(ctx, batch.Messages)
	if err != nil {
		return "", err
	}
	privateMessages, err := s.roomDirectedMessagesForSlot(roundValue, slot)
	if err != nil {
		return "", err
	}
	plan := roomdomain.BuildVisibleContextPlan(roomdomain.VisibleContextInput{
		PublicMessages:      runtimeMessages,
		RoomMessages:        privateMessages,
		LatestTrigger:       slot.Trigger,
		AgentNameByID:       agentNameByID,
		TargetAgentID:       slot.AgentID,
		ContextWindowTokens: slot.ContextWindow,
		ColdStart:           batch.ColdStart,
		PublicAnchor:        roomPublicAnchorMetadata(roundValue),
	})
	slot.PublicCursorID = plan.PublicBoundary.MessageID
	slot.PublicCursorTS = plan.PublicBoundary.Timestamp
	slot.MessageCursorID = plan.PrivateBoundary.MessageID
	slot.MessageCursorTS = plan.PrivateBoundary.Timestamp
	s.logRoomContextUsage(ctx, roundValue, slot, plan.Usage)
	return plan.Text, nil
}

func (s *RealtimeService) buildSlotGuidedPublicContext(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	publicHistory []protocol.Message,
	agentNameByID map[string]string,
	trigger roomTrigger,
) (string, error) {
	baseCursor := roomdomain.PublicCursor{
		LastMessageID: strings.TrimSpace(slot.PublicCursorID),
		LastTimestamp: slot.PublicCursorTS,
	}
	batch, err := s.publicInputBatchForSlot(ctx, roundValue, slot, publicHistory, baseCursor, true)
	if err != nil {
		return "", err
	}
	runtimeMessages, err := s.renderRuntimeAttachmentMessages(ctx, batch.Messages)
	if err != nil {
		return "", err
	}
	plan := roomdomain.BuildGuidedPublicInputContextPlan(roomdomain.VisibleContextInput{
		PublicMessages:      runtimeMessages,
		LatestTrigger:       trigger,
		AgentNameByID:       agentNameByID,
		TargetAgentID:       slot.AgentID,
		ContextWindowTokens: slot.ContextWindow,
		ColdStart:           batch.ColdStart,
		PublicAnchor:        roomPublicAnchorMetadata(roundValue),
	})
	if strings.TrimSpace(plan.PublicBoundary.MessageID) != "" || plan.PublicBoundary.Timestamp > 0 {
		slot.PublicCursorID = plan.PublicBoundary.MessageID
		slot.PublicCursorTS = plan.PublicBoundary.Timestamp
		if err = s.recordRoomPublicCursor(slot, roundValue, plan.PublicBoundary.MessageID, plan.PublicBoundary.Timestamp); err != nil {
			return "", err
		}
	}
	s.logRoomContextUsage(ctx, roundValue, slot, plan.Usage)
	return plan.Text, nil
}

func (s *RealtimeService) publicInputBatchForSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	publicHistory []protocol.Message,
	overrideCursor roomdomain.PublicCursor,
	overrideKnown bool,
) (roomdomain.PublicInputBatch, error) {
	cursor := overrideCursor
	cursorKnown := overrideKnown || (!slot.ContextColdStart &&
		(strings.TrimSpace(cursor.LastMessageID) != "" || cursor.LastTimestamp > 0))
	if !cursorKnown && !slot.ContextColdStart && s.history != nil {
		stored, ok, err := s.history.ReadRoomPublicCursor(
			slot.WorkspacePath,
			slot.RuntimeSessionKey,
			roundValue.ConversationID,
			slot.AgentID,
		)
		if err != nil {
			return roomdomain.PublicInputBatch{}, err
		}
		if ok {
			cursor = roomdomain.PublicCursor{
				LastMessageID: stored.LastPublicMessageID,
				LastTimestamp: stored.LastPublicTimestamp,
			}
			cursorKnown = true
		}
	}
	return roomdomain.BuildPublicInputBatch(roomdomain.PublicInputBatchInput{
		PublicHistory: publicHistory,
		Cursor:        cursor,
		CursorKnown:   cursorKnown,
	}), nil
}

func roomPublicAnchorMetadata(roundValue *activeRoomRound) roomdomain.PublicAnchorMetadata {
	if roundValue == nil || roundValue.Context == nil {
		return roomdomain.PublicAnchorMetadata{}
	}
	return roomdomain.PublicAnchorMetadata{
		RoomName:          roundValue.Context.Room.Name,
		RoomDescription:   roundValue.Context.Room.Description,
		ConversationTitle: roundValue.Context.Conversation.Title,
	}
}

func (s *RealtimeService) logRoomContextUsage(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	usage roomdomain.RoomContextUsage,
) {
	if roundValue == nil || slot == nil {
		return
	}
	s.loggerFor(ctx).Debug("Room 可见上下文预算已应用",
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"agent_round_id", slot.AgentRoundID,
		"context_window_tokens", usage.ContextWindowTokens,
		"budget_tokens", usage.BudgetTokens,
		"used_tokens", usage.UsedTokens,
		"current_message_tokens", usage.CurrentMessageTokens,
		"trigger_tokens", usage.TriggerTokens,
		"public_delta_tokens", usage.PublicDeltaTokens,
		"private_delta_tokens", usage.PrivateDeltaTokens,
		"public_anchor_tokens", usage.PublicAnchorTokens,
		"cold_start", usage.ColdStart,
	)
}

func (s *RealtimeService) recordRoomPublicCursor(slot *activeRoomSlot, roundValue *activeRoomRound, messageID string, timestamp int64) error {
	if s.history == nil || slot == nil || roundValue == nil {
		return nil
	}
	messageID = strings.TrimSpace(messageID)
	if messageID == "" && timestamp == 0 {
		return nil
	}
	return s.history.AppendRoomPublicCursor(slot.WorkspacePath, slot.RuntimeSessionKey, workspacestore.RoomPublicCursor{
		RoomID:              roundValue.RoomID,
		ConversationID:      roundValue.ConversationID,
		AgentID:             slot.AgentID,
		RoundID:             slot.AgentRoundID,
		LastPublicMessageID: messageID,
		LastPublicTimestamp: timestamp,
		Timestamp:           time.Now().UnixMilli(),
	})
}

func (s *RealtimeService) recordRoomDirectedMessageCursor(
	slot *activeRoomSlot,
	roundValue *activeRoomRound,
) (workspacestore.RoomDirectedMessageCursor, bool, error) {
	if s.directedMessages == nil || slot == nil || roundValue == nil {
		return workspacestore.RoomDirectedMessageCursor{}, false, nil
	}
	messageID := strings.TrimSpace(slot.MessageCursorID)
	if messageID == "" && slot.MessageCursorTS == 0 {
		return workspacestore.RoomDirectedMessageCursor{}, false, nil
	}
	cursor := workspacestore.RoomDirectedMessageCursor{
		RoomID:               roundValue.RoomID,
		ConversationID:       roundValue.ConversationID,
		AgentID:              slot.AgentID,
		RoundID:              slot.AgentRoundID,
		LastMessageID:        messageID,
		LastMessageTimestamp: slot.MessageCursorTS,
		Timestamp:            time.Now().UnixMilli(),
	}
	if err := s.directedMessages.AppendMessageCursor(cursor); err != nil {
		return workspacestore.RoomDirectedMessageCursor{}, false, err
	}
	return cursor, true, nil
}

func (s *RealtimeService) roomDirectedMessagesForSlot(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) ([]protocol.RoomDirectedMessageRecord, error) {
	if s.directedMessages == nil || roundValue == nil || slot == nil {
		return nil, nil
	}
	cursor, _, err := s.directedMessages.ReadMessageCursor(roundValue.ConversationID, slot.AgentID)
	if err != nil {
		return nil, err
	}
	if slot.ContextColdStart {
		cursor = workspacestore.RoomDirectedMessageCursor{}
	}
	var messages []protocol.RoomDirectedMessageRecord
	if slot.Trigger.TriggerType == roomDirectedMessageTriggerType {
		messages, err = s.directedMessages.ReadContextMessagesThrough(
			roundValue.ConversationID,
			slot.AgentID,
			cursor,
			slot.ReplySourceMessage,
		)
	} else {
		messages, err = s.directedMessages.ReadContextMessagesAfterCursor(roundValue.ConversationID, slot.AgentID, cursor)
	}
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func newRoomDirectedMessageConsumedEvent(cursor workspacestore.RoomDirectedMessageCursor) protocol.EventMessage {
	data := map[string]any{
		"room_id":                cursor.RoomID,
		"conversation_id":        cursor.ConversationID,
		"agent_id":               cursor.AgentID,
		"round_id":               cursor.RoundID,
		"last_message_id":        cursor.LastMessageID,
		"last_message_timestamp": cursor.LastMessageTimestamp,
	}
	event := protocol.NewEvent(protocol.EventTypeRoomDirectedMessageConsumed, data)
	event.SessionKey = protocol.BuildRoomSharedSessionKey(cursor.ConversationID)
	event.RoomID = cursor.RoomID
	event.ConversationID = cursor.ConversationID
	event.AgentID = cursor.AgentID
	event.RoundID = cursor.RoundID
	return event
}
