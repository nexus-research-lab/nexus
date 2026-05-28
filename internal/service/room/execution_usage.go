package room

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
)

func (s *RealtimeService) recordUsage(roundValue *activeRoomRound, message protocol.Message) {
	if s.usage == nil || roundValue == nil || protocol.MessageRole(message) != "result" {
		return
	}
	s.writeUsage(roundValue, message)
}

func (s *RealtimeService) recordTerminalAssistantUsage(roundValue *activeRoomRound, message protocol.Message) {
	if s.usage == nil || roundValue == nil || protocol.MessageRole(message) != "assistant" {
		return
	}
	s.writeUsage(roundValue, message)
}

func (s *RealtimeService) writeUsage(roundValue *activeRoomRound, message protocol.Message) {
	input := usagesvc.MessageRecordInput(roundValue.OwnerUserID, "room_runtime", message)
	if err := s.usage.RecordMessageUsage(context.Background(), input); err != nil {
		s.loggerFor(context.Background()).Error("Room token usage 写入失败",
			"s", roundValue.SessionKey,
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"err", err,
		)
	}
}
