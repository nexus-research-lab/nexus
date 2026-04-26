package room

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomMaxWakeHops = 16

func (s *RealtimeService) collectPublicMentionWakes(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	if roundValue == nil || roundValue.Context == nil || slot == nil {
		return nil
	}
	if !isFinalPublicAssistantMessage(message) {
		return nil
	}
	content := strings.TrimSpace(extractAssistantResultText(message))
	if content == "" {
		return nil
	}
	targetAgentIDs := ResolveMentionAgentIDs(content, buildRoomMentionAliases(roundValue.Context))
	if len(targetAgentIDs) == 0 {
		return nil
	}

	messageID := strings.TrimSpace(anyString(message["message_id"]))
	for _, targetAgentID := range targetAgentIDs {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" || targetAgentID == slot.AgentID || !isRoomMemberAgent(roundValue.Context.Members, targetAgentID) {
			continue
		}
		if isReciprocalPublicMention(slot, targetAgentID) {
			continue
		}
		s.enqueuePublicMentionWake(roundValue, publicMentionWake{
			SourceAgentID: slot.AgentID,
			TargetAgentID: targetAgentID,
			Content:       content,
			MessageID:     messageID,
		})
	}
	return nil
}

func isReciprocalPublicMention(slot *activeRoomSlot, targetAgentID string) bool {
	if slot == nil {
		return false
	}
	if strings.TrimSpace(slot.Trigger.TriggerType) != "public_mention" {
		return false
	}
	return strings.TrimSpace(slot.Trigger.SourceAgentID) == strings.TrimSpace(targetAgentID)
}

func isFinalPublicAssistantMessage(message protocol.Message) bool {
	if protocol.MessageRole(message) != "assistant" {
		return false
	}
	if message["is_complete"] == true {
		return true
	}
	_, hasResultSummary := message["result_summary"]
	return hasResultSummary
}

func (s *RealtimeService) enqueuePublicMentionWake(roundValue *activeRoomRound, wake publicMentionWake) {
	if roundValue == nil || strings.TrimSpace(wake.TargetAgentID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range roundValue.PublicMentions {
		if existing.TargetAgentID == wake.TargetAgentID &&
			strings.TrimSpace(existing.MessageID) == strings.TrimSpace(wake.MessageID) &&
			strings.TrimSpace(existing.Content) == strings.TrimSpace(wake.Content) {
			return
		}
	}
	roundValue.PublicMentions = append(roundValue.PublicMentions, wake)
}

func (s *RealtimeService) takePublicMentionWakes(roundValue *activeRoomRound) []publicMentionWake {
	if roundValue == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	wakes := append([]publicMentionWake(nil), roundValue.PublicMentions...)
	roundValue.PublicMentions = nil
	return wakes
}

func (s *RealtimeService) startQueuedPublicMentionWakes(ctx context.Context, roundValue *activeRoomRound) bool {
	wakes := s.takePublicMentionWakes(roundValue)
	if len(wakes) == 0 {
		return false
	}
	if roundValue.HopIndex >= roomMaxWakeHops {
		s.loggerFor(ctx).Warn("Room 公区 @ 唤醒达到跳数上限",
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"root_round_id", roomRootRoundID(roundValue),
		)
		return false
	}
	if err := s.startPublicMentionRound(ctx, roundValue, wakes); err != nil {
		s.loggerFor(ctx).Error("启动 Room 公区 @ 唤醒失败",
			"room_id", roundValue.RoomID,
			"conversation_id", roundValue.ConversationID,
			"root_round_id", roomRootRoundID(roundValue),
			"err", err,
		)
		return false
	}
	return true
}

func (s *RealtimeService) startPublicMentionRound(
	ctx context.Context,
	parentRound *activeRoomRound,
	wakes []publicMentionWake,
) error {
	if parentRound == nil || parentRound.Context == nil || len(wakes) == 0 {
		return nil
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(parentRound.ConversationID)
	contextValue := parentRound.Context
	wakes, err := s.queueBusyPublicMentionWakes(ctx, parentRound, sessionKey, wakes)
	if err != nil {
		return err
	}
	if len(wakes) == 0 {
		return nil
	}
	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue.Members)
	if err != nil {
		return err
	}
	publicHistory, err := s.roomHistory.ReadMessages(contextValue.Conversation.ID, nil)
	if err != nil {
		return err
	}

	roundID := "room_mention_" + newRealtimeID()
	rootRoundID := firstNonEmpty(roomRootRoundID(parentRound), roundID)
	activeRound := &activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         contextValue.Room.ID,
		ConversationID: contextValue.Conversation.ID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        roundID,
		RootRoundID:    rootRoundID,
		HopIndex:       parentRound.HopIndex + 1,
		OwnerUserID:    parentRound.OwnerUserID,
		Slots:          make(map[string]*activeRoomSlot),
		Done:           make(chan struct{}),
	}

	type pendingPublicMentionSlot struct {
		wake          publicMentionWake
		targetAgentID string
		sessionRecord SessionRecord
		agentValue    *agent2.Agent
	}
	pendingSlots := make([]pendingPublicMentionSlot, 0, len(wakes))
	targetSeen := make(map[string]struct{}, len(wakes))
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID == "" {
			continue
		}
		if _, exists := targetSeen[targetAgentID]; exists {
			continue
		}
		targetSeen[targetAgentID] = struct{}{}
		sessionRecord, ok := findRoomSessionForAgent(contextValue.Sessions, targetAgentID)
		if !ok {
			continue
		}
		agentValue := agentByID[targetAgentID]
		if agentValue == nil {
			continue
		}
		pendingSlots = append(pendingSlots, pendingPublicMentionSlot{
			wake:          wake,
			targetAgentID: targetAgentID,
			sessionRecord: sessionRecord,
			agentValue:    agentValue,
		})
	}
	if len(pendingSlots) == 0 {
		return nil
	}

	targetAgentIDs := make([]string, 0, len(pendingSlots))
	for _, pendingSlot := range pendingSlots {
		targetAgentIDs = append(targetAgentIDs, pendingSlot.targetAgentID)
	}

	pending := make([]map[string]any, 0, len(pendingSlots))
	for index, pendingSlot := range pendingSlots {
		msgID := newRealtimeID()
		agentRoundID := roundID
		if len(pendingSlots) > 1 {
			agentRoundID = fmt.Sprintf("%s:%s", roundID, pendingSlot.targetAgentID)
		}
		slotIndex := index
		activeRound.Slots[msgID] = buildPublicMentionSlot(
			contextValue,
			pendingSlot.sessionRecord,
			pendingSlot.agentValue,
			pendingSlot.wake,
			agentRoundID,
			msgID,
			slotIndex,
			targetAgentIDs,
			agentNameByID,
		)
		pending = append(pending, map[string]any{
			"agent_id":  pendingSlot.targetAgentID,
			"msg_id":    msgID,
			"round_id":  agentRoundID,
			"status":    "pending",
			"timestamp": time.Now().UnixMilli(),
			"index":     slotIndex,
		})
	}

	roundCtx, cancel := context.WithCancel(context.Background())
	activeRound.Cancel = cancel
	s.registerRound(activeRound)
	s.runtime.StartRound(sessionKey, roundID, cancel)
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, wrapRoomRoundStatusEvent(sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, roundID, "running", ""))
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, wrapRoomChatAckEvent(sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, roundID, roundID, pending))
	s.broadcastSessionStatus(ctx, sessionKey)
	go s.runRound(roundCtx, activeRound, publicHistory, agentNameByID, agentByID)
	return nil
}

func (s *RealtimeService) queueBusyPublicMentionWakes(
	ctx context.Context,
	parentRound *activeRoomRound,
	sessionKey string,
	wakes []publicMentionWake,
) ([]publicMentionWake, error) {
	if parentRound == nil || len(wakes) == 0 {
		return wakes, nil
	}
	targetAgentIDs := make([]string, 0, len(wakes))
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID != "" {
			targetAgentIDs = append(targetAgentIDs, targetAgentID)
		}
	}
	busySlots := s.findQueueSlots(sessionKey, parentRound.ConversationID, targetAgentIDs)
	if len(busySlots) == 0 {
		return wakes, nil
	}

	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, parentRound.Context)
	if err != nil {
		return nil, err
	}
	ready := make([]publicMentionWake, 0, len(wakes))
	queued := false
	for _, wake := range wakes {
		targetAgentID := strings.TrimSpace(wake.TargetAgentID)
		if targetAgentID == "" {
			continue
		}
		if _, busy := busySlots[targetAgentID]; !busy {
			ready = append(ready, wake)
			continue
		}
		location, ok := locationsByAgentID[targetAgentID]
		if !ok {
			continue
		}
		if _, err := s.inputQueue.Enqueue(location.Location, protocol.InputQueueItem{
			Scope:           protocol.InputQueueScopeRoom,
			SessionKey:      location.Location.SessionKey,
			RoomID:          parentRound.RoomID,
			ConversationID:  parentRound.ConversationID,
			AgentID:         targetAgentID,
			SourceAgentID:   strings.TrimSpace(wake.SourceAgentID),
			SourceMessageID: strings.TrimSpace(wake.MessageID),
			TargetAgentIDs:  []string{targetAgentID},
			Source:          protocol.InputQueueSourceAgentPublicMention,
			Content:         strings.TrimSpace(wake.Content),
			DeliveryPolicy:  protocol.ChatDeliveryPolicyQueue,
			OwnerUserID:     parentRound.OwnerUserID,
			RootRoundID:     roomRootRoundID(parentRound),
			HopIndex:        parentRound.HopIndex,
		}); err != nil {
			return nil, err
		}
		queued = true
		s.loggerFor(ctx).Info("Room 公区 @ 目标正忙，写入后端待发送队列",
			"session_key", sessionKey,
			"queue_session_key", location.Location.SessionKey,
			"room_id", parentRound.RoomID,
			"conversation_id", parentRound.ConversationID,
			"source_agent_id", wake.SourceAgentID,
			"target_agent_id", targetAgentID,
			"message_id", wake.MessageID,
		)
	}
	if queued {
		if err := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, parentRound.Context); err != nil {
			return nil, err
		}
	}
	return ready, nil
}

func buildPublicMentionSlot(
	contextValue *ConversationContextAggregate,
	sessionRecord SessionRecord,
	agentValue *agent2.Agent,
	wake publicMentionWake,
	agentRoundID string,
	msgID string,
	index int,
	targetAgentIDs []string,
	agentNameByID map[string]string,
) *activeRoomSlot {
	trigger := roomTrigger{
		TriggerType:   "public_mention",
		Content:       strings.TrimSpace(wake.Content),
		MessageID:     strings.TrimSpace(wake.MessageID),
		SourceAgentID: strings.TrimSpace(wake.SourceAgentID),
		TargetAgentID: strings.TrimSpace(wake.TargetAgentID),
		Metadata:      buildPublicMentionTriggerMetadata(targetAgentIDs, index, agentNameByID),
	}
	return &activeRoomSlot{
		RoomSessionID:     sessionRecord.ID,
		SDKSessionID:      strings.TrimSpace(sessionRecord.SDKSessionID),
		AgentID:           strings.TrimSpace(wake.TargetAgentID),
		AgentRoundID:      agentRoundID,
		MsgID:             msgID,
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(contextValue.Conversation.ID, wake.TargetAgentID, contextValue.Room.RoomType),
		WorkspacePath:     agentValue.WorkspacePath,
		Status:            "pending",
		Index:             index,
		TimestampMS:       time.Now().UnixMilli(),
		Trigger:           trigger,
		Done:              make(chan struct{}),
	}
}

func buildPublicMentionTriggerMetadata(targetAgentIDs []string, targetIndex int, agentNameByID map[string]string) map[string]any {
	targets := make([]string, 0, len(targetAgentIDs))
	targetNames := make([]string, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" {
			continue
		}
		targets = append(targets, targetAgentID)
		targetNames = append(targetNames, firstNonEmpty(agentNameByID[targetAgentID], targetAgentID))
	}
	if len(targets) == 0 {
		return nil
	}
	return map[string]any{
		"public_mention_target_count": len(targets),
		"public_mention_target_ids":   targets,
		"public_mention_target_index": targetIndex,
		"public_mention_target_names": targetNames,
	}
}

func isRoomMemberAgent(members []MemberRecord, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return false
	}
	for _, member := range members {
		if member.MemberType == MemberTypeAgent && strings.TrimSpace(member.MemberAgentID) == agentID {
			return true
		}
	}
	return false
}

func findRoomSessionForAgent(sessions []SessionRecord, agentID string) (SessionRecord, bool) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return SessionRecord{}, false
	}
	for _, sessionRecord := range sessions {
		if strings.TrimSpace(sessionRecord.AgentID) == agentID {
			return sessionRecord, true
		}
	}
	return SessionRecord{}, false
}

func buildRoomMentionAliases(contextValue *ConversationContextAggregate) map[string]string {
	if contextValue == nil {
		return nil
	}
	aliases := make(map[string]string, len(contextValue.MemberAgents)*3)
	for _, agentValue := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(agentValue.AgentID)
		if agentID == "" {
			continue
		}
		for _, candidate := range []string{agentValue.Name, agentValue.DisplayName, agentID} {
			alias := strings.TrimSpace(candidate)
			if alias != "" {
				aliases[alias] = agentID
				aliases[strings.ToLower(alias)] = agentID
			}
		}
	}
	for _, member := range contextValue.Members {
		if member.MemberType != MemberTypeAgent || strings.TrimSpace(member.MemberAgentID) == "" {
			continue
		}
		if _, exists := aliases[member.MemberAgentID]; !exists {
			aliases[member.MemberAgentID] = member.MemberAgentID
		}
	}
	return aliases
}
