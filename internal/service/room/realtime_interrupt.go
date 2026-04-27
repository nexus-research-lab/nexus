package room

import (
	"context"
	"errors"
	"strings"
	"time"
)

// InterruptConversation 中断指定 conversation 的全部活跃轮次。
func (s *RealtimeService) InterruptConversation(ctx context.Context, conversationID string, message string) error {
	normalizedConversationID := strings.TrimSpace(conversationID)
	if normalizedConversationID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectRoundTargets(func(roundValue *activeRoomRound) bool {
		return roundValue.ConversationID == normalizedConversationID
	}), message, true)
}

// InterruptRoom 中断指定 Room 下的全部活跃轮次。
func (s *RealtimeService) InterruptRoom(ctx context.Context, roomID string, message string) error {
	normalizedRoomID := strings.TrimSpace(roomID)
	if normalizedRoomID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectRoundTargets(func(roundValue *activeRoomRound) bool {
		return roundValue.RoomID == normalizedRoomID
	}), message, true)
}

// InterruptAgentTasks 中断指定成员在 Room 中的全部活跃子任务。
func (s *RealtimeService) InterruptAgentTasks(ctx context.Context, roomID string, agentID string, message string) error {
	normalizedRoomID := strings.TrimSpace(roomID)
	normalizedAgentID := strings.TrimSpace(agentID)
	if normalizedRoomID == "" || normalizedAgentID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectSlotTargets(func(roundValue *activeRoomRound, slot *activeRoomSlot) bool {
		return roundValue.RoomID == normalizedRoomID && slot.AgentID == normalizedAgentID
	}), message, true)
}

type interruptTarget struct {
	SessionKey string
	MsgID      string
}

func (s *RealtimeService) interruptAgentSlots(
	ctx context.Context,
	sessionKey string,
	agentIDs []string,
	message string,
	suppressError bool,
) error {
	targetAgents := make(map[string]struct{}, len(agentIDs))
	for _, agentID := range agentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" {
			targetAgents[agentID] = struct{}{}
		}
	}
	if len(targetAgents) == 0 {
		return nil
	}
	return s.interruptTargets(ctx, s.collectSlotTargets(func(roundValue *activeRoomRound, slot *activeRoomSlot) bool {
		if roundValue == nil || slot == nil || roundValue.SessionKey != sessionKey {
			return false
		}
		_, ok := targetAgents[strings.TrimSpace(slot.AgentID)]
		return ok
	}), message, suppressError)
}

func (s *RealtimeService) collectRoundTargets(
	matcher func(*activeRoomRound) bool,
) []interruptTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	targets := make([]interruptTarget, 0)
	seen := make(map[string]struct{})
	for _, roundValue := range s.activeRounds {
		if roundValue == nil || !matcher(roundValue) {
			continue
		}
		if _, exists := seen[roundValue.SessionKey]; exists {
			continue
		}
		seen[roundValue.SessionKey] = struct{}{}
		targets = append(targets, interruptTarget{SessionKey: roundValue.SessionKey})
	}
	return targets
}

func (s *RealtimeService) collectSlotTargets(
	matcher func(*activeRoomRound, *activeRoomSlot) bool,
) []interruptTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	targets := make([]interruptTarget, 0)
	seen := make(map[string]struct{})
	for _, roundValue := range s.activeRounds {
		if roundValue == nil {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || !matcher(roundValue, slot) {
				continue
			}
			targetKey := roundValue.SessionKey + "::" + slot.MsgID
			if _, exists := seen[targetKey]; exists {
				continue
			}
			seen[targetKey] = struct{}{}
			targets = append(targets, interruptTarget{
				SessionKey: roundValue.SessionKey,
				MsgID:      slot.MsgID,
			})
		}
	}
	return targets
}

func (s *RealtimeService) interruptTargets(
	ctx context.Context,
	targets []interruptTarget,
	message string,
	suppressError bool,
) error {
	errs := make([]error, 0)
	for _, target := range targets {
		if err := s.interruptRound(ctx, target.SessionKey, target.MsgID, message, suppressError); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *RealtimeService) interruptRound(
	ctx context.Context,
	sessionKey string,
	msgID string,
	message string,
	suppressError bool,
) error {
	if strings.TrimSpace(msgID) != "" {
		roundValue, slot := s.findActiveSlot(sessionKey, msgID)
		if slot == nil {
			if suppressError {
				return nil
			}
			return errors.New("target room slot not found")
		}
		return s.interruptActiveSlot(ctx, roundValue, slot, message, suppressError)
	}

	rounds := s.activeRoundsForSession(sessionKey)
	errs := make([]error, 0)
	for _, roundValue := range rounds {
		if err := s.interruptActiveRound(ctx, roundValue, message, suppressError); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *RealtimeService) activeRoundsForSession(sessionKey string) []*activeRoomRound {
	s.mu.Lock()
	defer s.mu.Unlock()
	rounds := make([]*activeRoomRound, 0)
	for _, roundValue := range s.activeRounds {
		if roundValue == nil || roundValue.SessionKey != sessionKey {
			continue
		}
		rounds = append(rounds, roundValue)
	}
	return rounds
}

func (s *RealtimeService) findActiveSlot(sessionKey string, msgID string) (*activeRoomRound, *activeRoomSlot) {
	msgID = strings.TrimSpace(msgID)
	if msgID == "" {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, roundValue := range s.activeRounds {
		if roundValue == nil || roundValue.SessionKey != sessionKey {
			continue
		}
		if slot := roundValue.Slots[msgID]; slot != nil {
			return roundValue, slot
		}
	}
	return nil, nil
}

func (s *RealtimeService) interruptActiveSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message string,
	suppressError bool,
) error {
	if roundValue == nil || slot == nil {
		return nil
	}
	interruptReason := normalizeRoomInterruptReason(message)
	markRoomSlotInterrupted(slot, interruptReason)
	shouldBroadcast := slot.Status != "finished" && slot.Status != "error" && slot.Status != "cancelled"
	if slot.Client != nil {
		if err := slot.Client.Interrupt(ctx); err != nil && !suppressError {
			return err
		}
	}
	s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, interruptReason)
	if shouldBroadcast {
		s.loggerFor(ctx).Warn("请求中断 Room slot",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"reason", interruptReason,
		)
	}
	select {
	case <-slot.Done:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(interruptForceCancelDelay):
		if slot.Cancel != nil {
			slot.Cancel()
		}
		select {
		case <-slot.Done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
	return nil
}

func (s *RealtimeService) interruptActiveRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	message string,
	suppressError bool,
) error {
	if roundValue == nil {
		return nil
	}
	s.loggerFor(ctx).Warn("请求中断 Room round",
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"round_id", roundValue.RoundID,
		"reason", normalizeRoomInterruptReason(message),
	)
	interruptReason := normalizeRoomInterruptReason(message)
	for _, slot := range roundValue.Slots {
		markRoomSlotInterrupted(slot, interruptReason)
		if slot.Client != nil {
			if err := slot.Client.Interrupt(ctx); err != nil && !suppressError {
				return err
			}
		}
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, interruptReason)
	}
	select {
	case <-roundValue.Done:
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(interruptForceCancelDelay):
		if roundValue.Cancel != nil {
			roundValue.Cancel()
		}
		select {
		case <-roundValue.Done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
	return nil
}
