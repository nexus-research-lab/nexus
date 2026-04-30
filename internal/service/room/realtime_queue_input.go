package room

import (
	"context"
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func (s *RealtimeService) queueActiveAgentSlots(
	ctx context.Context,
	sessionKey string,
	conversationID string,
	targetAgentIDs []string,
	content string,
	roundID string,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findQueueSlots(sessionKey, conversationID, targetAgentIDs)
	queuedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	for agentID, slot := range slotsByAgentID {
		if slot == nil {
			continue
		}
		client := slot.getClient()
		if client == nil {
			slot.enqueueQueuedInput(roundID, content)
			queuedAgentIDs[agentID] = struct{}{}
			s.loggerFor(ctx).Info("Room 排队消息等待 slot 启动",
				"session_key", sessionKey,
				"conversation_id", conversationID,
				"agent_id", agentID,
				"round_id", roundID,
				"active_round_id", slot.AgentRoundID,
				"msg_id", slot.MsgID,
			)
			continue
		}
		if err := runtimectx.SendClientContent(ctx, client, strings.TrimSpace(content)); err != nil {
			return queuedAgentIDs, err
		}
		queuedAgentIDs[agentID] = struct{}{}
		s.loggerFor(ctx).Info("排队 Room 消息到运行中 slot",
			"session_key", sessionKey,
			"conversation_id", conversationID,
			"agent_id", agentID,
			"round_id", roundID,
			"active_round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
		)
	}
	return queuedAgentIDs, nil
}

func (s *RealtimeService) findQueueSlots(
	sessionKey string,
	conversationID string,
	targetAgentIDs []string,
) map[string]*activeRoomSlot {
	targets := make(map[string]struct{}, len(targetAgentIDs))
	for _, agentID := range targetAgentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" {
			targets[agentID] = struct{}{}
		}
	}
	if len(targets) == 0 {
		return map[string]*activeRoomSlot{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]*activeRoomSlot, len(targets))
	for _, roundValue := range s.activeRounds {
		if roundValue == nil ||
			roundValue.SessionKey != sessionKey ||
			roundValue.ConversationID != conversationID {
			continue
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || !isActiveQueueSlot(slot) {
				continue
			}
			if _, ok := targets[slot.AgentID]; !ok {
				continue
			}
			current := result[slot.AgentID]
			if current == nil || slot.TimestampMS > current.TimestampMS {
				result[slot.AgentID] = slot
			}
		}
	}
	return result
}

func isActiveQueueSlot(slot *activeRoomSlot) bool {
	return slot != nil && !slot.isTerminal()
}

func filterQueuedAgentIDs(agentIDs []string, queued map[string]struct{}) []string {
	if len(queued) == 0 {
		return agentIDs
	}
	result := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		if _, ok := queued[agentID]; ok {
			continue
		}
		result = append(result, agentID)
	}
	return result
}

func (s *RealtimeService) guideActiveAgentSlots(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	targetAgentIDs []string,
	content string,
	roundID string,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findQueueSlots(sessionKey, conversationID, targetAgentIDs)
	guidedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	for agentID, slot := range slotsByAgentID {
		if slot == nil {
			continue
		}
		slot.enqueueGuidedInput(roundID, content)
		guidanceMessage := buildRoomGuidanceMessage(sessionKey, roomID, conversationID, slot, roundID, content)
		s.broadcastSlotGuidanceMessage(ctx, sessionKey, roomID, conversationID, roundID, guidanceMessage)
		guidedAgentIDs[agentID] = struct{}{}
		s.loggerFor(ctx).Info("登记 Room 引导消息等待 PostToolUse 注入",
			"session_key", sessionKey,
			"room_id", roomID,
			"runtime_session_key", slot.RuntimeSessionKey,
			"conversation_id", conversationID,
			"agent_id", agentID,
			"round_id", roundID,
			"active_round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
		)
	}
	return guidedAgentIDs, nil
}
