package room

import (
	"context"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (s *RealtimeService) startIdleSubagentNotificationDrains(ctx context.Context, roundValue *activeRoomRound) {
	if s == nil || roundValue == nil {
		return
	}
	for _, slot := range roundValue.Slots {
		if slot == nil || !slot.hasRunningSubagentTask() {
			continue
		}
		client := slot.getClient()
		if client == nil {
			continue
		}
		mapper := roomdomain.NewSlotMessageMapper(
			roundValue.SessionKey,
			roundValue.RoomID,
			roundValue.ConversationID,
			slot.AgentID,
			slot.MsgID,
			roundValue.RootRoundID,
			slot.AgentRoundID,
			slot.WorkspacePath,
		)
		go s.drainIdleSubagentNotifications(ctx, roundValue, slot, mapper, client.ReceiveMessages(ctx))
	}
}

func (s *RealtimeService) drainIdleSubagentNotifications(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	mapper *roomdomain.SlotMessageMapper,
	messageCh <-chan sdkprotocol.ReceivedMessage,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case incoming, ok := <-messageCh:
			if !ok {
				return
			}
			if !s.handleIdleSubagentMessage(ctx, roundValue, slot, mapper, incoming) {
				return
			}
		}
	}
}

func (s *RealtimeService) handleIdleSubagentMessage(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	mapper *roomdomain.SlotMessageMapper,
	incoming sdkprotocol.ReceivedMessage,
) bool {
	events, durableMessages, _, err := mapper.Map(incoming)
	if err != nil {
		s.loggerFor(ctx).Warn("处理 Room idle subagent 通知失败",
			"session_key", roundValue.SessionKey,
			"round_id", roundValue.RoundID,
			"agent_id", slot.AgentID,
			"err", err,
		)
		return true
	}
	for _, messageValue := range durableMessages {
		if messageValue == nil {
			continue
		}
		if err := s.handleIdleSubagentDurableMessage(ctx, roundValue, slot, messageValue); err != nil {
			s.loggerFor(ctx).Warn("写入 Room idle subagent 通知失败",
				"session_key", roundValue.SessionKey,
				"round_id", roundValue.RoundID,
				"agent_id", slot.AgentID,
				"err", err,
			)
			return true
		}
	}
	for _, event := range events {
		if roomSlotShouldDropPublicOutputEvent(slot, event) {
			continue
		}
		for _, readyEvent := range slot.eventsReadyForEmission(event) {
			s.broadcastSharedEventWithTimeout(ctx, roundValue.SessionKey, roundValue.RoomID, readyEvent)
		}
	}
	if slot.hasRunningSubagentTask() {
		return true
	}
	s.releaseRoundSubagentWait(roundValue)
	return false
}

func (s *RealtimeService) handleIdleSubagentDurableMessage(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	messageValue protocol.Message,
) error {
	slot.rememberSubagentTaskMessage(messageValue)
	if !roomSlotPublishesPublicOutput(slot) {
		if !protocol.IsTranscriptNativeMessage(messageValue) {
			if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(messageValue, slot.RuntimeSessionKey)); err != nil {
				return err
			}
		}
		s.recordGoalUsageFromSlotAssistantMessage(ctx, slot, messageValue)
		return nil
	}
	if err := s.persistSharedDurableMessage(roundValue.ConversationID, slot, messageValue); err != nil {
		return err
	}
	if !protocol.IsTranscriptNativeMessage(messageValue) {
		if err := s.persistPrivateOverlayMessage(slot, cloneMessageWithSessionKey(messageValue, slot.RuntimeSessionKey)); err != nil {
			return err
		}
	}
	s.recordGoalUsageFromSlotAssistantMessage(ctx, slot, messageValue)
	return nil
}

func (s *RealtimeService) releaseRoundSubagentWait(roundValue *activeRoomRound) {
	if s == nil || roundValue == nil {
		return
	}
	shouldDispatch := false
	s.mu.Lock()
	if roundValue.RunningSubagents.Load() && !roundValue.hasRunningSubagentTasks() {
		roundValue.RunningSubagents.Store(false)
		shouldDispatch = true
	}
	s.mu.Unlock()
	if shouldDispatch {
		s.dispatchPostRoundWork(contextWithQueueOwner(context.Background(), roundValue.OwnerUserID), roundValue)
	}
}
