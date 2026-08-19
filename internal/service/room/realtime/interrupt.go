// INPUT: Room session、root round 或 agent slot 的精确中断目标。
// OUTPUT: 幂等精确停止、整轮停止，以及可识别的目标已结束结果。
// POS: Room runtime 取消边界；精确目标不得扩大为整个共享 session 中断。
package realtime

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ErrTargetRoomRoundNotRunning 表示精确 root round 已自然结束或从未运行。
var ErrTargetRoomRoundNotRunning = errors.New("target room round not found or already ended")

// HandleInterrupt 处理中断请求。带 agent_round_id 时只停对应 slot，否则停整个 round。
func (s *Service) HandleInterrupt(ctx context.Context, request InterruptRequest) error {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return err
	}
	if agentRoundID := strings.TrimSpace(request.AgentRoundID); agentRoundID != "" {
		roundValue, slot := s.findActiveSlotByAgentRoundID(sessionKey, agentRoundID)
		if slot == nil {
			// 精确停止与自然完成存在合法竞态。目标已经离开 active registry 时
			// 按幂等成功收口，禁止客户端因迟到错误把已完成执行重新暴露为可停止。
			return nil
		}
		return s.interruptActiveSlot(ctx, roundValue, slot, "")
	}
	if roundID := strings.TrimSpace(request.RoundID); roundID != "" {
		roundValue := s.findActiveRoundByRoundID(sessionKey, roundID)
		if roundValue == nil {
			return ErrTargetRoomRoundNotRunning
		}
		return s.interruptActiveRound(ctx, roundValue, "")
	}
	return s.interruptRound(ctx, sessionKey, "", "", false)
}

// InterruptConversation 中断指定 conversation 的全部活跃轮次。
func (s *Service) InterruptConversation(ctx context.Context, conversationID string, message string) error {
	normalizedConversationID := strings.TrimSpace(conversationID)
	if normalizedConversationID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectRoundTargets(func(roundValue *activeRoomRound) bool {
		return roundValue.ConversationID == normalizedConversationID
	}), message, true)
}

// InterruptRoom 中断指定 Room 下的全部活跃轮次。
func (s *Service) InterruptRoom(ctx context.Context, roomID string, message string) error {
	normalizedRoomID := strings.TrimSpace(roomID)
	if normalizedRoomID == "" {
		return nil
	}
	return s.interruptTargets(ctx, s.collectRoundTargets(func(roundValue *activeRoomRound) bool {
		return roundValue.RoomID == normalizedRoomID
	}), message, true)
}

// InterruptAgentTasks 中断指定成员在 Room 中的全部活跃子任务。
func (s *Service) InterruptAgentTasks(ctx context.Context, roomID string, agentID string, message string) error {
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

func (s *Service) interruptAgentSlots(
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

func (s *Service) collectRoundTargets(
	matcher func(*activeRoomRound) bool,
) []interruptTarget {
	targets := make([]interruptTarget, 0)
	seen := make(map[string]struct{})
	for _, roundValue := range s.roundsRegistry().snapshot() {
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

func (s *Service) collectSlotTargets(
	matcher func(*activeRoomRound, *activeRoomSlot) bool,
) []interruptTarget {
	targets := make([]interruptTarget, 0)
	seen := make(map[string]struct{})
	for _, roundValue := range s.roundsRegistry().snapshot() {
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

func (s *Service) interruptTargets(
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

func (s *Service) interruptRound(
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
		return s.interruptActiveSlot(ctx, roundValue, slot, message)
	}

	rounds := s.activeRoundsForSession(sessionKey)
	errs := make([]error, 0)
	for _, roundValue := range rounds {
		if err := s.interruptActiveRound(ctx, roundValue, message); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) activeRoundsForSession(sessionKey string) []*activeRoomRound {
	result := make([]*activeRoomRound, 0)
	for _, roundValue := range s.roundsRegistry().snapshot() {
		if roundValue != nil && roundValue.SessionKey == sessionKey {
			result = append(result, roundValue)
		}
	}
	return result
}

func (s *Service) findActiveSlotByAgentRoundID(sessionKey string, agentRoundID string) (*activeRoomRound, *activeRoomSlot) {
	agentRoundID = strings.TrimSpace(agentRoundID)
	if agentRoundID == "" {
		return nil, nil
	}
	return s.roundsRegistry().findSlotByAgentRound(sessionKey, agentRoundID)
}

func (s *Service) findActiveRoundByRoundID(sessionKey string, roundID string) *activeRoomRound {
	return s.roundsRegistry().findByRoundID(sessionKey, roundID)
}

func (s *Service) findActiveSlot(sessionKey string, msgID string) (*activeRoomRound, *activeRoomSlot) {
	msgID = strings.TrimSpace(msgID)
	if msgID == "" {
		return nil, nil
	}
	return s.roundsRegistry().findSlot(sessionKey, msgID)
}

func (s *Service) interruptActiveSlot(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message string,
) error {
	if roundValue == nil || slot == nil {
		return nil
	}
	interruptReason := normalizeRoomInterruptReason(message)
	displayInterruptReason := roomInterruptDisplayReason(interruptReason)
	markRoomSlotInterrupted(slot, interruptReason)
	shouldBroadcast := !slot.isTerminal()
	if client := slot.getClient(); client != nil {
		if err := client.Interrupt(ctx); err != nil {
			s.loggerFor(ctx).Warn("Room slot 中断 client 失败，继续强制取消",
				"session_key", roundValue.SessionKey,
				"room_id", roundValue.RoomID,
				"conversation_id", roundValue.ConversationID,
				"agent_id", slot.AgentID,
				"round_id", slot.AgentRoundID,
				"msg_id", slot.MsgID,
				"err", err,
			)
		}
	}
	s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, displayInterruptReason)
	if shouldBroadcast {
		s.loggerFor(ctx).Warn("请求中断 Room slot",
			"session_key", roundValue.SessionKey,
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"agent_id", slot.AgentID,
			"round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"reason", displayInterruptReason,
		)
	}
	select {
	case <-slot.doneChannel():
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(interruptForceCancelDelay):
		slot.cancelRuntime()
		select {
		case <-slot.doneChannel():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.broadcastSessionStatus(ctx, roundValue.SessionKey)
	return nil
}

func (s *Service) interruptActiveRound(
	ctx context.Context,
	roundValue *activeRoomRound,
	message string,
) error {
	if roundValue == nil {
		return nil
	}
	// 整轮停止时同步收口所有派生 public handoff，避免 root 取消后
	// 持久化 queue 在稍后又把目标 Agent 唤醒。
	s.cancelRootPublicHandoffs(ctx, roundValue, "interrupted")
	interruptReason := normalizeRoomInterruptReason(message)
	displayInterruptReason := roomInterruptDisplayReason(interruptReason)
	s.loggerFor(ctx).Warn("请求中断 Room round",
		"session_key", roundValue.SessionKey,
		"room_id", roundValue.RoomID,
		"conversation_id", roundValue.ConversationID,
		"round_id", roundValue.RoundID,
		"reason", displayInterruptReason,
	)
	for _, slot := range roundValue.Slots {
		markRoomSlotInterrupted(slot, interruptReason)
		if client := slot.getClient(); client != nil {
			if err := client.Interrupt(ctx); err != nil {
				s.loggerFor(ctx).Warn("Room round 中断 client 失败，继续强制取消",
					"session_key", roundValue.SessionKey,
					"room_id", roundValue.RoomID,
					"conversation_id", roundValue.ConversationID,
					"agent_id", slot.AgentID,
					"round_id", slot.AgentRoundID,
					"msg_id", slot.MsgID,
					"err", err,
				)
			}
		}
		s.permission.CancelRequestsForSession(slot.RuntimeSessionKey, displayInterruptReason)
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
