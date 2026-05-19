package room

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *RealtimeService) enqueueForActiveAgentSlots(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	targetAgentIDs []string,
	content string,
	attachments []protocol.ChatAttachment,
	roundID string,
	ownerUserID string,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findActiveDeliverySlots(sessionKey, conversationID, targetAgentIDs)
	queuedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	for agentID, slot := range slotsByAgentID {
		if slot == nil {
			continue
		}
		location := workspacestore.InputQueueLocation{
			Scope:          protocol.InputQueueScopeRoom,
			WorkspacePath:  slot.WorkspacePath,
			SessionKey:     slot.RuntimeSessionKey,
			RoomID:         roomID,
			ConversationID: conversationID,
		}
		if _, err := s.inputQueue.Enqueue(location, protocol.InputQueueItem{
			Scope:           protocol.InputQueueScopeRoom,
			SessionKey:      slot.RuntimeSessionKey,
			RoomID:          roomID,
			ConversationID:  conversationID,
			AgentID:         agentID,
			SourceMessageID: strings.TrimSpace(roundID),
			TargetAgentIDs:  []string{agentID},
			Source:          protocol.InputQueueSourceUser,
			Content:         strings.TrimSpace(content),
			Attachments:     protocol.NormalizeChatAttachments(attachments, agentID),
			DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
			OwnerUserID:     strings.TrimSpace(ownerUserID),
			RootRoundID:     strings.TrimSpace(roundID),
		}); err != nil {
			return queuedAgentIDs, err
		}
		queuedAgentIDs[agentID] = struct{}{}
		s.loggerFor(ctx).Info("Room 公区消息写入目标 agent 待处理队列",
			"session_key", sessionKey,
			"conversation_id", conversationID,
			"agent_id", agentID,
			"round_id", roundID,
			"active_round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"content_chars", utf8.RuneCountInString(strings.TrimSpace(content)),
			"content_preview", logx.PreviewText(content, 240),
		)
	}
	return queuedAgentIDs, nil
}

func (s *RealtimeService) findActiveDeliverySlots(
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
			if slot == nil || !isActiveDeliverySlot(slot) {
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

func isActiveDeliverySlot(slot *activeRoomSlot) bool {
	if slot == nil {
		return false
	}
	switch slot.getStatus() {
	case "finished", "error", "cancelled":
		return false
	default:
		return true
	}
}

func filterHandledAgentIDs(agentIDs []string, handled map[string]struct{}) []string {
	if len(handled) == 0 {
		return agentIDs
	}
	result := make([]string, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		if _, ok := handled[agentID]; ok {
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
	runtimeContent string,
	roundID string,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findActiveDeliverySlots(sessionKey, conversationID, targetAgentIDs)
	guidedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	for agentID, slot := range slotsByAgentID {
		if slot == nil {
			continue
		}
		slot.enqueueGuidedInput(roundID, runtimeContent)
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
			"content_chars", utf8.RuneCountInString(strings.TrimSpace(content)),
			"content_preview", logx.PreviewText(content, 240),
		)
	}
	return guidedAgentIDs, nil
}
