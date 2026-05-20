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
	agentValue *protocol.Agent,
) (string, error) {
	batch, err := s.publicInputBatchForSlot(ctx, roundValue, slot, publicHistory, agentNameByID, roomdomain.PublicCursor{})
	if err != nil {
		return "", err
	}
	runtimeMessages, err := s.renderRuntimeAttachmentMessages(ctx, batch.Messages)
	if err != nil {
		return "", err
	}
	slot.PublicCursorID = batch.LastMessageID
	slot.PublicCursorTS = batch.LastTimestamp
	actions, err := s.roomActionsForSlot(roundValue, slot)
	if err != nil {
		return "", err
	}
	base := roomdomain.BuildVisibleContext(roomdomain.VisibleContextInput{
		PublicMessages: runtimeMessages,
		RoomActions:    actions,
		LatestTrigger:  slot.Trigger,
		AgentNameByID:  agentNameByID,
		TargetAgentID:  slot.AgentID,
	})
	return s.prependRoomMemoryContext(ctx, roundValue, slot, agentValue, base), nil
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
	batch, err := s.publicInputBatchForSlot(ctx, roundValue, slot, publicHistory, agentNameByID, baseCursor)
	if err != nil {
		return "", err
	}
	runtimeMessages, err := s.renderRuntimeAttachmentMessages(ctx, batch.Messages)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(batch.LastMessageID) != "" || batch.LastTimestamp > 0 {
		slot.PublicCursorID = batch.LastMessageID
		slot.PublicCursorTS = batch.LastTimestamp
		if err = s.recordRoomPublicCursor(slot, roundValue, batch.LastMessageID, batch.LastTimestamp); err != nil {
			return "", err
		}
	}
	return roomdomain.BuildGuidedPublicInputContext(roomdomain.VisibleContextInput{
		PublicMessages: runtimeMessages,
		LatestTrigger:  trigger,
		AgentNameByID:  agentNameByID,
		TargetAgentID:  slot.AgentID,
	}), nil
}

func (s *RealtimeService) publicInputBatchForSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	publicHistory []protocol.Message,
	agentNameByID map[string]string,
	overrideCursor roomdomain.PublicCursor,
) (roomdomain.PublicInputBatch, error) {
	cursor := overrideCursor
	if strings.TrimSpace(cursor.LastMessageID) == "" && cursor.LastTimestamp == 0 && s.history != nil {
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
		}
	}
	return roomdomain.BuildPublicInputBatch(roomdomain.PublicInputBatchInput{
		PublicHistory: publicHistory,
		Cursor:        cursor,
		AgentNameByID: agentNameByID,
		TargetAgentID: slot.AgentID,
	}), nil
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

func (s *RealtimeService) recordRoomActionCursor(
	slot *activeRoomSlot,
	roundValue *activeRoomRound,
) (workspacestore.RoomActionCursor, bool, error) {
	if s.actions == nil || slot == nil || roundValue == nil {
		return workspacestore.RoomActionCursor{}, false, nil
	}
	actionID := strings.TrimSpace(slot.ActionCursorID)
	if actionID == "" && slot.ActionCursorTS == 0 {
		return workspacestore.RoomActionCursor{}, false, nil
	}
	cursor := workspacestore.RoomActionCursor{
		RoomID:              roundValue.RoomID,
		ConversationID:      roundValue.ConversationID,
		AgentID:             slot.AgentID,
		RoundID:             slot.AgentRoundID,
		LastActionID:        actionID,
		LastActionTimestamp: slot.ActionCursorTS,
		Timestamp:           time.Now().UnixMilli(),
	}
	if err := s.actions.AppendActionCursor(cursor); err != nil {
		return workspacestore.RoomActionCursor{}, false, err
	}
	return cursor, true, nil
}

func (s *RealtimeService) roomActionsForSlot(
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
) ([]protocol.RoomActionRecord, error) {
	if s.actions == nil || roundValue == nil || slot == nil {
		return nil, nil
	}
	cursor, _, err := s.actions.ReadActionCursor(roundValue.ConversationID, slot.AgentID)
	if err != nil {
		return nil, err
	}
	actions, err := s.actions.ReadContextActionsAfterCursor(roundValue.ConversationID, slot.AgentID, cursor)
	if err != nil {
		return nil, err
	}
	if len(actions) > 0 {
		lastAction := actions[len(actions)-1]
		slot.ActionCursorID = lastAction.ActionID
		slot.ActionCursorTS = lastAction.Timestamp
	}
	return actions, nil
}

func newRoomActionConsumedEvent(cursor workspacestore.RoomActionCursor) protocol.EventMessage {
	data := map[string]any{
		"room_id":               cursor.RoomID,
		"conversation_id":       cursor.ConversationID,
		"agent_id":              cursor.AgentID,
		"round_id":              cursor.RoundID,
		"last_action_id":        cursor.LastActionID,
		"last_action_timestamp": cursor.LastActionTimestamp,
	}
	event := protocol.NewEvent(protocol.EventTypeRoomActionConsumed, data)
	event.SessionKey = protocol.BuildRoomSharedSessionKey(cursor.ConversationID)
	event.RoomID = cursor.RoomID
	event.ConversationID = cursor.ConversationID
	event.AgentID = cursor.AgentID
	event.CausedBy = cursor.RoundID
	return event
}
