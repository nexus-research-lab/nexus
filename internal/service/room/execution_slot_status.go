package room

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (s *RealtimeService) syncSlotSDKSessionID(ctx context.Context, slot *activeRoomSlot, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || sessionID == slot.getSDKSessionID() {
		return nil
	}
	slot.setSDKSessionID(sessionID)
	if s.rooms == nil {
		return nil
	}
	return s.rooms.UpdateSessionSDKSessionID(ctx, slot.RoomSessionID, sessionID)
}

func (s *RealtimeService) handleSlotFailure(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper, err error) {
	fields := []any{
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
		"err", err,
	}
	fields = append(fields, roomSlotFailureDiagnostics(err, slot, mapper)...)
	s.loggerFor(ctx).Error("Room slot 执行失败", fields...)
	slot.setStatus("error")
	resultMessage := protocol.Message{
		"message_id":      "result_" + slot.AgentRoundID,
		"session_key":     roundValue.SessionKey,
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"agent_id":        slot.AgentID,
		"round_id":        slot.AgentRoundID,
		"parent_id":       slot.MsgID,
		"role":            "result",
		"subtype":         "error",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"result":          err.Error(),
		"is_error":        true,
		"timestamp":       time.Now().UnixMilli(),
	}
	_ = s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey))
	if roomSlotPublishesPublicOutput(slot) {
		_ = s.persistSharedInlineMessage(roundValue.ConversationID, resultMessage)
		projectedMessage := message.ProjectResultMessage(nil, resultMessage)
		if mapper != nil {
			projectedMessage = mapper.ProjectResultMessage(resultMessage)
		}
		s.broadcastSharedEventWithTimeout(
			ctx,
			roundValue.SessionKey,
			roundValue.RoomID,
			roomdomain.WrapMessageEvent(
				roundValue.RoomID,
				roundValue.ConversationID,
				projectedMessage,
				slot.AgentRoundID,
			),
		)
		s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.NewErrorEvent(roundValue.SessionKey, roundValue.RoomID, roundValue.ConversationID, "room_error", err.Error(), slot.AgentRoundID))
	}
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamEnd,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func roomSlotFailureDiagnostics(err error, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper) []any {
	fields := make([]any, 0, 16)
	var streamClosed *runtimectx.RoundStreamClosedError
	if errors.As(err, &streamClosed) {
		fields = append(fields,
			"stream_messages_seen", streamClosed.MessagesSeen,
			"stream_last_type", streamClosed.LastMessageType,
			"stream_last_session_id", streamClosed.LastSessionID,
			"stream_last_message_id", streamClosed.LastMessageID,
			"stream_read_error", streamClosed.ReadError,
			"stream_wait_error", streamClosed.WaitError,
		)
	}
	if mapper != nil {
		lastAssistant := mapper.LastAssistantMessage()
		fields = append(fields,
			"sdk_session_id", mapper.SessionID(),
			"current_message_id", mapper.CurrentMessageID(),
			"last_assistant_message_id", anyString(lastAssistant["message_id"]),
			"last_assistant_complete", lastAssistant["is_complete"],
			"last_assistant_chars", utf8.RuneCountInString(strings.TrimSpace(roomdomain.ExtractHistoryText(lastAssistant))),
		)
	}
	if client := slot.getClient(); client != nil {
		fields = append(fields, "client_session_id", client.SessionID())
	}
	return fields
}

func (s *RealtimeService) handleSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper) {
	if !s.markSlotCancelled(slot) {
		return
	}
	s.loggerFor(ctx).Warn("Room slot 已取消",
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"agent_id", slot.AgentID,
		"round_id", slot.AgentRoundID,
		"msg_id", slot.MsgID,
		"reason", roomSlotInterruptReason(slot),
	)
	if mapper != nil {
		s.recordGoalUsageForSlot(ctx, slot, runtimectx.RoundExecutionResult{}, slot.lastGoalAssistantMessage())
	}
	s.emitInterruptedSlotResult(roundValue, slot, mapper, "")
	s.broadcastSlotCancelled(ctx, roundValue, slot)
}

func (s *RealtimeService) markSlotCancelled(slot *activeRoomSlot) bool {
	if slot == nil {
		return false
	}
	return slot.markCancelled()
}

func (s *RealtimeService) broadcastSlotCancelled(ctx context.Context, roundValue *activeRoomRound, slot *activeRoomSlot) {
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapLifecycleEvent(
		protocol.EventTypeStreamCancelled,
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		slot.AgentID,
		slot.MsgID,
		slot.AgentRoundID,
	))
}

func (s *RealtimeService) emitInterruptedSlotResult(roundValue *activeRoomRound, slot *activeRoomSlot, mapper *roomdomain.SlotMessageMapper, resultText string) {
	if roundValue == nil || slot == nil {
		return
	}
	resultMessage := protocol.Message{
		"message_id":      "result_" + slot.AgentRoundID,
		"session_key":     roundValue.SessionKey,
		"room_id":         roundValue.RoomID,
		"conversation_id": roundValue.ConversationID,
		"agent_id":        slot.AgentID,
		"round_id":        slot.AgentRoundID,
		"parent_id":       slot.MsgID,
		"role":            "result",
		"subtype":         "interrupted",
		"duration_ms":     0,
		"duration_api_ms": 0,
		"num_turns":       0,
		"is_error":        false,
		"timestamp":       time.Now().UnixMilli(),
	}
	if trimmedResult := strings.TrimSpace(resultText); trimmedResult != "" {
		resultMessage["result"] = trimmedResult
	}
	if client := slot.getClient(); client != nil {
		if sessionID := strings.TrimSpace(client.SessionID()); sessionID != "" {
			resultMessage["session_id"] = sessionID
		}
	}
	if roomSlotPublishesPublicOutput(slot) {
		if err := s.persistSharedInlineMessage(roundValue.ConversationID, resultMessage); err != nil {
			s.loggerFor(context.Background()).Error("Room interrupted 共享结果持久化失败",
				"s", roundValue.SessionKey,
				"r", roundValue.RoomID,
				"c", roundValue.ConversationID,
				"err", err,
			)
		} else {
			projectedMessage := message.ProjectResultMessage(nil, resultMessage)
			if mapper != nil {
				projectedMessage = mapper.ProjectResultMessage(resultMessage)
			}
			s.broadcastSharedEvent(
				context.Background(),
				roundValue.SessionKey,
				roundValue.RoomID,
				roomdomain.WrapMessageEvent(
					roundValue.RoomID,
					roundValue.ConversationID,
					projectedMessage,
					slot.AgentRoundID,
				),
			)
		}
	}
	if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(resultMessage, slot.RuntimeSessionKey)); err != nil {
		s.loggerFor(context.Background()).Error("Room interrupted 私有结果持久化失败",
			"s", roundValue.SessionKey,
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"err", err,
		)
	}
}
