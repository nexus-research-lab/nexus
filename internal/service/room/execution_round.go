package room

import (
	"context"
	"sync"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *RealtimeService) runRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	history []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*protocol.Agent,
) {
	ctx = contextWithQueueOwner(ctx, roundValue.OwnerUserID)
	logger := s.loggerFor(ctx).With(
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"round_id", roundValue.RoundID,
	)
	logger.Info("开始执行 Room round", "slot_count", len(roundValue.Slots))
	var waitGroup sync.WaitGroup
	for _, slot := range roundValue.Slots {
		waitGroup.Add(1)
		go func(currentSlot *activeRoomSlot) {
			defer waitGroup.Done()
			s.runSlot(ctx, roundValue, currentSlot, history, agentNameByID, agentByID[currentSlot.AgentID])
		}(slot)
	}
	waitGroup.Wait()

	s.finishRound(roundValue)

	finalStatus := "finished"
	if roundValue.allSlotsCancelled() {
		finalStatus = "interrupted"
	} else if roundValue.hasSlotError() {
		finalStatus = "error"
	}
	logger.Info("Room round 结束", "status", finalStatus)
	s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, roomdomain.WrapRoundStatusEvent(
		roundValue.SessionKey,
		roundValue.RoomID,
		roundValue.ConversationID,
		roundValue.RoundID,
		finalStatus,
		mapTerminalSubtype(finalStatus),
	))
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
	if finalStatus == "finished" {
		s.startQueuedPublicMentionWakes(context.Background(), roundValue)
	}
	go s.dispatchPostRoundWork(contextWithQueueOwner(context.Background(), roundValue.OwnerUserID), roundValue)
}
