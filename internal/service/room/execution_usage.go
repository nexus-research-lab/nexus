package room

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

func (s *RealtimeService) recordUsage(roundValue *activeRoomRound, slot *activeRoomSlot, message protocol.Message) {
	if s.usage == nil || roundValue == nil || slot == nil || protocol.MessageRole(message) != "result" {
		return
	}
	if !usagesvc.MessageHasUsage(message) {
		return
	}
	if s.writeUsage(roundValue, message) {
		slot.resultUsageWritten = true
	}
}

func (s *RealtimeService) recordTerminalAssistantUsage(roundValue *activeRoomRound, slot *activeRoomSlot, message protocol.Message) {
	if s.usage == nil || roundValue == nil || slot == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	if slot.resultUsageWritten || !usagesvc.MessageHasUsage(message) {
		return
	}
	s.writeUsage(roundValue, message)
}

func (s *RealtimeService) writeUsage(roundValue *activeRoomRound, message protocol.Message) bool {
	input := usagesvc.MessageRecordInput(roundValue.OwnerUserID, "room_runtime", message)
	if err := s.usage.RecordMessageUsage(context.Background(), input); err != nil {
		s.loggerFor(context.Background()).Error("Room token usage 写入失败",
			"s", roundValue.SessionKey,
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"err", err,
		)
		return false
	}
	return true
}
