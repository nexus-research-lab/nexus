package room

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomMaxWakeHops = 16

type pendingPublicMentionSlot struct {
	wake          publicMentionWake
	targetAgentID string
	sessionRecord protocol.SessionRecord
	agentValue    *protocol.Agent
}

func (s *RealtimeService) collectPublicMentionWakes(
	ctx context.Context,
	roundValue *activeRoomRound,
	slot *activeRoomSlot,
	message protocol.Message,
) error {
	if roundValue == nil || roundValue.Context == nil || slot == nil {
		return nil
	}
	messageID := strings.TrimSpace(anyString(message["message_id"]))
	if !roomdomain.IsFinalPublicAssistantMessage(message) {
		return nil
	}
	content := strings.TrimSpace(roomdomain.ExtractAssistantResultText(message))
	if content == "" {
		return nil
	}
	targetAgentIDs := roomdomain.ResolveMentionAgentIDs(content, roomdomain.BuildMentionAliases(roundValue.Context))
	if len(targetAgentIDs) == 0 {
		return nil
	}

	for _, targetAgentID := range targetAgentIDs {
		targetAgentID = strings.TrimSpace(targetAgentID)
		if targetAgentID == "" {
			continue
		}
		if targetAgentID == slot.AgentID {
			continue
		}
		if !roomdomain.IsMemberAgent(roundValue.Context.Members, targetAgentID) {
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
	wakes := slices.Clone(roundValue.PublicMentions)
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
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"root", roomRootRoundID(roundValue),
		)
		return false
	}
	if err := s.startPublicMentionRound(ctx, roundValue, wakes); err != nil {
		s.loggerFor(ctx).Error("启动 Room 公区 @ 唤醒失败",
			"r", roundValue.RoomID,
			"c", roundValue.ConversationID,
			"root", roomRootRoundID(roundValue),
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
	if parentRound.HopIndex >= roomMaxWakeHops {
		s.loggerFor(ctx).Warn("Room 唤醒达到跳数上限",
			"r", parentRound.RoomID,
			"c", parentRound.ConversationID,
			"root", roomRootRoundID(parentRound),
		)
		return nil
	}
	sessionKey := protocol.BuildRoomSharedSessionKey(parentRound.ConversationID)
	contextValue := parentRound.Context
	wakes, err := s.queueBusyPublicMentionWakes(ctx, parentRound, sessionKey, wakes)
	if err != nil {
		return err
	}
	if len(wakes) == 0 {
		s.logQueuedPublicMentionWakes(ctx, parentRound, sessionKey)
		return nil
	}
	agentNameByID, agentByID, err := s.buildAgentDirectory(ctx, contextValue)
	if err != nil {
		return err
	}
	publicHistory, err := s.roomHistory.ReadMessages(contextValue.Conversation.ID, nil)
	if err != nil {
		return err
	}
	pendingSlots := buildPendingPublicMentionSlots(contextValue, wakes, agentByID)
	if len(pendingSlots) == 0 {
		s.logMissingPublicMentionSlots(ctx, sessionKey, contextValue, len(wakes))
		return nil
	}
	roundID := roomWakeRoundID(wakes)
	activeRound := newPublicMentionRound(parentRound, sessionKey, roundID)
	targetAgentIDs, pending := addPublicMentionSlots(activeRound, contextValue, pendingSlots)
	s.launchPublicMentionRound(
		ctx,
		activeRound,
		wakes,
		pendingSlots,
		targetAgentIDs,
		pending,
		publicHistory,
		agentNameByID,
		agentByID,
	)
	return nil
}

func (s *RealtimeService) logQueuedPublicMentionWakes(
	ctx context.Context,
	parentRound *activeRoomRound,
	sessionKey string,
) {
	s.loggerFor(ctx).Info("Room 公区 @ 目标均已进入队列",
		"s", sessionKey,
		"r", parentRound.Context.Room.ID,
		"c", parentRound.Context.Conversation.ID,
		"parent", parentRound.RoundID,
		"root", roomRootRoundID(parentRound),
	)
}

func buildPendingPublicMentionSlots(
	contextValue *protocol.ConversationContextAggregate,
	wakes []publicMentionWake,
	agentByID map[string]*protocol.Agent,
) []pendingPublicMentionSlot {
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
		if !ok || agentByID[targetAgentID] == nil {
			continue
		}
		pendingSlots = append(pendingSlots, pendingPublicMentionSlot{
			wake:          wake,
			targetAgentID: targetAgentID,
			sessionRecord: sessionRecord,
			agentValue:    agentByID[targetAgentID],
		})
	}
	return pendingSlots
}

func (s *RealtimeService) logMissingPublicMentionSlots(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	wakeCount int,
) {
	s.loggerFor(ctx).Warn("Room 公区 @ 没有可启动的目标 slot",
		"s", sessionKey,
		"r", contextValue.Room.ID,
		"c", contextValue.Conversation.ID,
		"wakes", wakeCount,
	)
}

func newPublicMentionRound(parentRound *activeRoomRound, sessionKey string, roundID string) *activeRoomRound {
	contextValue := parentRound.Context
	return &activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         contextValue.Room.ID,
		ConversationID: contextValue.Conversation.ID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        roundID,
		RootRoundID:    cmp.Or(roomRootRoundID(parentRound), roundID),
		HopIndex:       parentRound.HopIndex + 1,
		OwnerUserID:    parentRound.OwnerUserID,
		Slots:          make(map[string]*activeRoomSlot),
		Done:           make(chan struct{}),
	}
}

func addPublicMentionSlots(
	activeRound *activeRoomRound,
	contextValue *protocol.ConversationContextAggregate,
	pendingSlots []pendingPublicMentionSlot,
) ([]string, []protocol.ChatAckPendingSlot) {
	targetAgentIDs := make([]string, 0, len(pendingSlots))
	pending := make([]protocol.ChatAckPendingSlot, 0, len(pendingSlots))
	for index, pendingSlot := range pendingSlots {
		targetAgentIDs = append(targetAgentIDs, pendingSlot.targetAgentID)
		msgID := newRealtimeID()
		agentRoundID := protocol.NewAgentRoundID()
		slotIndex := index
		activeRound.Slots[msgID] = buildPublicMentionSlot(
			contextValue,
			pendingSlot.sessionRecord,
			pendingSlot.agentValue,
			pendingSlot.wake,
			agentRoundID,
			msgID,
			slotIndex,
		)
		pending = append(pending, protocol.ChatAckPendingSlot{
			AgentID:      pendingSlot.targetAgentID,
			AgentRoundID: agentRoundID,
			MsgID:        msgID,
			Status:       "pending",
			Timestamp:    time.Now().UnixMilli(),
			Index:        slotIndex,
		})
	}
	return targetAgentIDs, pending
}

func (s *RealtimeService) launchPublicMentionRound(
	ctx context.Context,
	activeRound *activeRoomRound,
	wakes []publicMentionWake,
	pendingSlots []pendingPublicMentionSlot,
	targetAgentIDs []string,
	pending []protocol.ChatAckPendingSlot,
	publicHistory []protocol.Message,
	agentNameByID map[string]string,
	agentByID map[string]*protocol.Agent,
) {
	sessionKey := activeRound.SessionKey
	contextValue := activeRound.Context
	roundID := activeRound.RoundID
	roundCtx, cancel := context.WithCancel(context.Background())
	activeRound.Cancel = cancel
	s.registerRound(activeRound)
	s.runtime.StartRound(sessionKey, roundID, cancel)
	s.loggerFor(ctx).Info(roomWakeStartLogMessage(wakes),
		"s", sessionKey,
		"r", contextValue.Room.ID,
		"c", contextValue.Conversation.ID,
		"hop", activeRound.HopIndex,
		"targets", targetAgentIDs,
		"pending", len(pending),
	)
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapRoundStatusEvent(sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, roundID, "running", ""))
	// 公区 @ 唤醒由后端发起，没有前端请求，client 关联字段留空。
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapChatAckEvent(sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, "", "", roundID, "", false, pending))
	for _, pendingSlot := range pendingSlots {
		if normalizeWakeQueueSource(pendingSlot.wake) != protocol.InputQueueSourceAgentRoomMessage {
			continue
		}
		s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, newRoomDirectedMessageWakeEvent(activeRound, pendingSlot.wake, "wake_started", map[string]any{
			"round_id": roundID,
		}))
	}
	s.broadcastSessionStatus(ctx, sessionKey)
	go s.runRound(roundCtx, activeRound, publicHistory, agentNameByID, agentByID)
}

func buildPublicMentionSlot(
	contextValue *protocol.ConversationContextAggregate,
	sessionRecord protocol.SessionRecord,
	agentValue *protocol.Agent,
	wake publicMentionWake,
	agentRoundID string,
	msgID string,
	index int,
) *activeRoomSlot {
	triggerType := strings.TrimSpace(wake.TriggerType)
	if triggerType == "" {
		triggerType = "public_mention"
	}
	trigger := roomTrigger{
		TriggerType:   triggerType,
		Content:       strings.TrimSpace(wake.Content),
		MessageID:     strings.TrimSpace(wake.MessageID),
		SourceAgentID: strings.TrimSpace(wake.SourceAgentID),
		TargetAgentID: strings.TrimSpace(wake.TargetAgentID),
		ReplyRoute:    wake.ReplyRoute,
	}
	return &activeRoomSlot{
		RoomSessionID:      sessionRecord.ID,
		SDKSessionID:       strings.TrimSpace(sessionRecord.SDKSessionID),
		AgentID:            strings.TrimSpace(wake.TargetAgentID),
		AgentRoundID:       agentRoundID,
		MsgID:              msgID,
		RuntimeSessionKey:  protocol.BuildRoomAgentSessionKey(contextValue.Conversation.ID, wake.TargetAgentID, contextValue.Room.RoomType),
		WorkspacePath:      agentValue.WorkspacePath,
		Status:             "pending",
		Index:              index,
		TimestampMS:        time.Now().UnixMilli(),
		Trigger:            trigger,
		ReplyRoute:         wake.ReplyRoute,
		ReplySourceMessage: strings.TrimSpace(wake.MessageID),
		ReplySourceAgent:   strings.TrimSpace(wake.SourceAgentID),
		Done:               make(chan struct{}),
	}
}

func normalizeWakeQueueSource(wake publicMentionWake) protocol.InputQueueSource {
	if wake.QueueSource == protocol.InputQueueSourceAgentRoomMessage {
		return protocol.InputQueueSourceAgentRoomMessage
	}
	return protocol.InputQueueSourceAgentPublicMention
}

func roomWakeRoundID(wakes []publicMentionWake) string {
	prefix := "room_mention_"
	if len(wakes) > 0 && normalizeWakeQueueSource(wakes[0]) == protocol.InputQueueSourceAgentRoomMessage {
		prefix = "room_directed_message_"
	}
	return prefix + newRealtimeID()
}

func roomWakeStartLogMessage(wakes []publicMentionWake) string {
	if len(wakes) > 0 && normalizeWakeQueueSource(wakes[0]) == protocol.InputQueueSourceAgentRoomMessage {
		return "启动 Room directed message 唤醒 round"
	}
	return "启动 Room 公区 @ 唤醒 round"
}

func roomWakeQueuedLogMessage(wake publicMentionWake) string {
	if normalizeWakeQueueSource(wake) == protocol.InputQueueSourceAgentRoomMessage {
		return "Room directed message 目标正忙，写入后端待发送队列"
	}
	return "Room 公区 @ 目标正忙，写入后端待发送队列"
}

func findRoomSessionForAgent(sessions []protocol.SessionRecord, agentID string) (protocol.SessionRecord, bool) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return protocol.SessionRecord{}, false
	}
	for _, sessionRecord := range sessions {
		if strings.TrimSpace(sessionRecord.AgentID) == agentID {
			return sessionRecord, true
		}
	}
	return protocol.SessionRecord{}, false
}
