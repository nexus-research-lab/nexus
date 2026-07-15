// INPUT: Room 消息目标、活跃 round slot 与持久化输入队列。
// OUTPUT: queue 按 Agent 独立投递到最新 slot；guide 只原子投递到同一 active root。
// POS: Room 活跃执行目标解析与输入登记的数据面。
package room

import (
	"context"
	"maps"
	"slices"
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
	userMessageID string,
	ownerUserID string,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findActiveDeliverySlotsByAgent(sessionKey, conversationID, targetAgentIDs)
	queuedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(slotsByAgentID))
	for _, agentID := range slices.Sorted(maps.Keys(slotsByAgentID)) {
		slot := slotsByAgentID[agentID]
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
		entries = append(entries, workspacestore.InputQueueEnqueue{
			Location: location,
			Item: protocol.InputQueueItem{
				ID:              strings.TrimSpace(roundID),
				Scope:           protocol.InputQueueScopeRoom,
				SessionKey:      slot.RuntimeSessionKey,
				RoomID:          roomID,
				ConversationID:  conversationID,
				AgentID:         agentID,
				SourceMessageID: strings.TrimSpace(userMessageID),
				TargetAgentIDs:  []string{agentID},
				Source:          protocol.InputQueueSourceUser,
				Content:         strings.TrimSpace(content),
				Attachments:     protocol.NormalizeChatAttachments(attachments, agentID),
				DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
				OwnerUserID:     strings.TrimSpace(ownerUserID),
				RootRoundID:     strings.TrimSpace(roundID),
			},
		})
	}
	if err := s.inputQueue.EnqueueBatch(entries); err != nil {
		return queuedAgentIDs, err
	}
	for _, entry := range entries {
		agentID := entry.Item.AgentID
		slot := slotsByAgentID[agentID]
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

// findActiveDeliverySlotsByAgent 为每个目标独立选择最新活跃 slot。它用于
// queue/空闲判断：一个目标忙碌不能让同一条多目标输入把它再次启动，也不能
// 阻止其他空闲目标立即开始。
func (s *RealtimeService) findActiveDeliverySlotsByAgent(
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
	result := make(map[string]*activeRoomSlot, len(targets))
	if len(targets) == 0 {
		return result
	}

	s.mu.Lock()
	defer s.mu.Unlock()
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
			if current == nil || slot.TimestampMS > current.TimestampMS ||
				(slot.TimestampMS == current.TimestampMS && slot.AgentRoundID < current.AgentRoundID) {
				result[slot.AgentID] = slot
			}
		}
	}
	return result
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

	// guide 会把用户消息挂到正在流式输出的 root；多目标只有同属一个
	// active root 才能原子注入，避免同一 public message 被不同 root 反复改写。
	slotsByRoot := make(map[string]map[string]*activeRoomSlot)
	latestTimestampByRoot := make(map[string]int64)
	for roundKey, roundValue := range s.activeRounds {
		if roundValue == nil ||
			roundValue.SessionKey != sessionKey ||
			roundValue.ConversationID != conversationID {
			continue
		}
		rootRoundID := roomRootRoundID(roundValue)
		if rootRoundID == "" {
			// 生产 round 始终有 ID；此 fallback 只用于保持构造中状态和单元测试
			// 的同一 activeRoomRound 边界。
			rootRoundID = "active_round:" + roundKey
		}
		for _, slot := range roundValue.Slots {
			if slot == nil || !isActiveDeliverySlot(slot) {
				continue
			}
			if _, ok := targets[slot.AgentID]; !ok {
				continue
			}
			slots := slotsByRoot[rootRoundID]
			if slots == nil {
				slots = make(map[string]*activeRoomSlot, len(targets))
				slotsByRoot[rootRoundID] = slots
			}
			current := slots[slot.AgentID]
			if current == nil || slot.TimestampMS > current.TimestampMS {
				slots[slot.AgentID] = slot
			}
			if slot.TimestampMS > latestTimestampByRoot[rootRoundID] {
				latestTimestampByRoot[rootRoundID] = slot.TimestampMS
			}
		}
	}

	selectedRoot := ""
	var selectedTimestamp int64
	for rootRoundID, slots := range slotsByRoot {
		if len(slots) != len(targets) {
			continue
		}
		timestamp := latestTimestampByRoot[rootRoundID]
		if selectedRoot == "" || timestamp > selectedTimestamp ||
			(timestamp == selectedTimestamp && rootRoundID < selectedRoot) {
			selectedRoot = rootRoundID
			selectedTimestamp = timestamp
		}
	}
	if selectedRoot == "" {
		return map[string]*activeRoomSlot{}
	}
	return slotsByRoot[selectedRoot]
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
	sourceItem protocol.InputQueueItem,
) (map[string]struct{}, error) {
	slotsByAgentID := s.findActiveDeliverySlots(sessionKey, conversationID, targetAgentIDs)
	guidedAgentIDs := make(map[string]struct{}, len(slotsByAgentID))
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(slotsByAgentID))
	for _, agentID := range slices.Sorted(maps.Keys(slotsByAgentID)) {
		slot := slotsByAgentID[agentID]
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
		entries = append(entries, workspacestore.InputQueueEnqueue{
			Location: location,
			Item: protocol.InputQueueItem{
				ID:              strings.TrimSpace(sourceItem.ID),
				Scope:           protocol.InputQueueScopeRoom,
				SessionKey:      slot.RuntimeSessionKey,
				RoomID:          roomID,
				ConversationID:  conversationID,
				AgentID:         agentID,
				SourceAgentID:   strings.TrimSpace(sourceItem.SourceAgentID),
				SourceMessageID: strings.TrimSpace(sourceItem.SourceMessageID),
				HandoffID:       strings.TrimSpace(sourceItem.HandoffID),
				TargetAgentIDs:  []string{agentID},
				Source:          protocol.NormalizeInputQueueSource(string(sourceItem.Source)),
				Content:         strings.TrimSpace(sourceItem.Content),
				Attachments:     protocol.NormalizeChatAttachments(sourceItem.Attachments, agentID),
				DeliveryPolicy:  protocol.ChatDeliveryPolicyGuide,
				ReplyRoute:      sourceItem.ReplyRoute,
				OwnerUserID:     strings.TrimSpace(sourceItem.OwnerUserID),
				RootRoundID:     slot.AgentRoundID,
				HopIndex:        sourceItem.HopIndex,
			},
		})
	}
	if err := s.inputQueue.EnqueueBatch(entries); err != nil {
		return guidedAgentIDs, err
	}
	for _, entry := range entries {
		agentID := entry.Item.AgentID
		slot := slotsByAgentID[agentID]
		guidedAgentIDs[agentID] = struct{}{}
		s.loggerFor(ctx).Info("持久化 Room 引导消息等待 PostToolUse 注入",
			"session_key", sessionKey,
			"room_id", roomID,
			"runtime_session_key", slot.RuntimeSessionKey,
			"conversation_id", conversationID,
			"agent_id", agentID,
			"round_id", sourceItem.ID,
			"active_round_id", slot.AgentRoundID,
			"msg_id", slot.MsgID,
			"content_chars", utf8.RuneCountInString(strings.TrimSpace(sourceItem.Content)),
			"content_preview", logx.PreviewText(sourceItem.Content, 240),
		)
	}
	return guidedAgentIDs, nil
}
