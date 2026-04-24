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
	}), message)
}

// InterruptRoom 中断指定 Room 下的全部活跃轮次。
func (s *RealtimeService) InterruptRoom(ctx context.Context, roomID string, message string) error {
	normalizedRoomID := strings.TrimSpace(roomID)
	if normalizedRoomID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectRoundTargets(func(roundValue *activeRoomRound) bool {
		return roundValue.RoomID == normalizedRoomID
	}), message)
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
	}), message)
}

type interruptTarget struct {
	SessionKey string
	MsgID      string
}

func (s *RealtimeService) collectRoundTargets(
	matcher func(*activeRoomRound) bool,
) []interruptTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	targets := make([]interruptTarget, 0)
	for _, roundValue := range s.activeRounds {
		if roundValue == nil || !matcher(roundValue) {
			continue
		}
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
) error {
	errs := make([]error, 0)
	for _, target := range targets {
		if err := s.interruptRound(ctx, target.SessionKey, target.MsgID, message, true); err != nil {
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
	s.mu.Lock()
	roundValue := s.activeRounds[sessionKey]
	s.mu.Unlock()
	if roundValue == nil {
		return nil
	}

	if strings.TrimSpace(msgID) != "" {
		slot := roundValue.Slots[msgID]
		if slot == nil {
			if suppressError {
				return nil
			}
			return errors.New("target room slot not found")
		}
		shouldBroadcast := slot.Status != "finished" && slot.Status != "error" && slot.Status != "cancelled"
		if slot.Client != nil {
			if err := slot.Client.Interrupt(ctx); err != nil && !suppressError {
				return err
			}
		}
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, message)
		if shouldBroadcast {
			s.loggerFor(ctx).Warn("请求中断 Room slot",
				"session_key", sessionKey,
				"room_id", roundValue.RoomID,
				"conversation_id", roundValue.ConversationID,
				"agent_id", slot.AgentID,
				"round_id", slot.AgentRoundID,
				"msg_id", slot.MsgID,
				"reason", message,
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
		s.broadcastSessionStatus(ctx, sessionKey)
		return nil
	}

	s.loggerFor(ctx).Warn("请求中断 Room round",
		"session_key", sessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"round_id", roundValue.RoundID,
		"reason", message,
	)
	for _, slot := range roundValue.Slots {
		if slot.Client != nil {
			if err := slot.Client.Interrupt(ctx); err != nil && !suppressError {
				return err
			}
		}
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, message)
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
	s.broadcastSessionStatus(ctx, sessionKey)
	return nil
}
