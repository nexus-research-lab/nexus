package room

import (
	"context"
	"errors"
	"sort"
	"strings"

	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// InputQueueRequest 表示 Room 待发送队列控制请求。
type InputQueueRequest struct {
	SessionKey     string
	RoomID         string
	ConversationID string
	Action         string
	ItemID         string
	Content        string
	OrderedIDs     []string
	DeliveryPolicy protocol.ChatDeliveryPolicy
}

type roomInputQueueLocation struct {
	AgentID  string
	Location workspacestore.InputQueueLocation
}

type roomInputQueueEntry struct {
	Item     protocol.InputQueueItem
	Location workspacestore.InputQueueLocation
}

// HandleInputQueue 处理 Room 待发送队列控制消息。
func (s *RealtimeService) HandleInputQueue(ctx context.Context, request InputQueueRequest) error {
	sessionKey, contextValue, err := s.resolveInputQueueContext(ctx, request)
	if err != nil {
		return err
	}

	action := strings.TrimSpace(request.Action)
	switch action {
	case "enqueue", "":
		content := strings.TrimSpace(request.Content)
		if content == "" {
			return errors.New("content is required")
		}
		location, targetAgentIDs, err := s.resolveRoomInputQueuePrimaryLocation(ctx, contextValue, content)
		if err != nil {
			return err
		}
		if _, err = s.inputQueue.Enqueue(location, protocol.InputQueueItem{
			Scope:          protocol.InputQueueScopeRoom,
			SessionKey:     location.SessionKey,
			RoomID:         contextValue.Room.ID,
			ConversationID: contextValue.Conversation.ID,
			AgentID:        inputQueueLocationAgentID(location),
			TargetAgentIDs: targetAgentIDs,
			Source:         protocol.InputQueueSourceUser,
			Content:        content,
			DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy)),
			OwnerUserID:    ownerUserIDFromContext(ctx),
		}); err != nil {
			return err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return err
		}
		go s.dispatchNextInputQueueItem(contextWithQueueOwner(context.Background(), ownerUserIDFromContext(ctx)), sessionKey, contextValue.Room.ID, contextValue.Conversation.ID)
		return nil
	case "delete":
		if err = s.deleteRoomInputQueueItem(ctx, contextValue, request.ItemID); err != nil {
			return err
		}
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	case "reorder":
		if err = s.reorderRoomInputQueueItems(ctx, contextValue, request.OrderedIDs); err != nil {
			return err
		}
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	case "guide":
		return s.guideInputQueueItem(ctx, sessionKey, contextValue, request.ItemID)
	default:
		return errors.New("unsupported input_queue action")
	}
}

// InputQueueSnapshotEvent 构造 Room 队列快照事件，供新订阅连接恢复状态。
func (s *RealtimeService) InputQueueSnapshotEvent(
	ctx context.Context,
	roomID string,
	conversationID string,
) (protocol.EventMessage, error) {
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return protocol.EventMessage{}, err
	}
	if contextValue == nil {
		return protocol.EventMessage{}, errors.New("room conversation not found")
	}
	items, err := s.roomInputQueueItems(ctx, contextValue)
	if err != nil {
		return protocol.EventMessage{}, err
	}
	event := newRoomInputQueueEvent(sessionKey, strings.TrimSpace(roomID), strings.TrimSpace(conversationID), items)
	go s.dispatchNextInputQueueItem(ctx, sessionKey, roomID, conversationID)
	return event, nil
}

func (s *RealtimeService) guideInputQueueItem(
	ctx context.Context,
	sessionKey string,
	contextValue *ConversationContextAggregate,
	itemID string,
) error {
	entry, ok, err := s.findRoomInputQueueEntry(ctx, contextValue, itemID)
	if err != nil {
		return err
	}
	if !ok {
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	}
	if _, err = s.inputQueue.Delete(entry.Location, entry.Item.ID); err != nil {
		return err
	}
	if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
		return err
	}
	if entry.Item.Source == protocol.InputQueueSourceAgentPublicMention {
		return s.dispatchAgentPublicMentionQueueItem(
			contextWithQueueOwner(ctx, entry.Item.OwnerUserID),
			sessionKey,
			contextValue.Room.ID,
			contextValue.Conversation.ID,
			entry.Item,
			protocol.ChatDeliveryPolicyGuide,
		)
	}
	return s.HandleChat(contextWithQueueOwner(ctx, entry.Item.OwnerUserID), ChatRequest{
		SessionKey:     sessionKey,
		RoomID:         contextValue.Room.ID,
		ConversationID: contextValue.Conversation.ID,
		Content:        entry.Item.Content,
		RoundID:        "queue_" + entry.Item.ID,
		ReqID:          "queue_" + entry.Item.ID,
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	})
}

func (s *RealtimeService) dispatchNextInputQueueItem(ctx context.Context, sessionKey string, roomID string, conversationID string) {
	if strings.TrimSpace(sessionKey) == "" {
		return
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil || contextValue == nil {
		if err != nil {
			s.loggerFor(ctx).Error("读取 Room 待发送队列上下文失败", "session_key", sessionKey, "err", err)
		}
		return
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		s.loggerFor(ctx).Error("读取 Room 待发送队列失败", "session_key", sessionKey, "err", err)
		return
	}
	if len(entries) == 0 || !s.canDispatchInputQueueItem(sessionKey, conversationID, entries[0].Item) {
		return
	}
	entry := entries[0]
	if _, err = s.inputQueue.Dispatch(entry.Location, entry.Item.ID); err != nil {
		s.loggerFor(ctx).Error("弹出 Room 待发送队列失败", "session_key", sessionKey, "err", err)
		return
	}
	if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
		s.loggerFor(ctx).Warn("广播 Room 待发送队列快照失败", "session_key", sessionKey, "err", err)
	}
	err = s.dispatchInputQueueItem(ctx, sessionKey, roomID, conversationID, entry.Item)
	if err == nil {
		if s.canDispatchMoreInputQueueItems(ctx, sessionKey, conversationID) {
			go s.dispatchNextInputQueueItem(ctx, sessionKey, roomID, conversationID)
		}
		return
	}
	s.loggerFor(ctx).Error("派发 Room 待发送队列失败",
		"session_key", sessionKey,
		"room_id", roomID,
		"conversation_id", conversationID,
		"item_id", entry.Item.ID,
		"err", err,
	)
	if _, restoreErr := s.inputQueue.Enqueue(entry.Location, entry.Item); restoreErr != nil {
		s.loggerFor(ctx).Error("恢复 Room 待发送队列项失败",
			"session_key", sessionKey,
			"item_id", entry.Item.ID,
			"err", restoreErr,
		)
	} else if snapshotErr := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); snapshotErr != nil {
		s.loggerFor(ctx).Warn("广播恢复后的 Room 待发送队列快照失败", "session_key", sessionKey, "err", snapshotErr)
	}
	s.broadcastSharedEvent(ctx, sessionKey, roomID, s.newRoomErrorEvent(sessionKey, roomID, conversationID, "input_queue_error", "待发送消息派发失败", entry.Item.ID))
}

func (s *RealtimeService) dispatchInputQueueItem(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
) error {
	if item.Source == protocol.InputQueueSourceAgentPublicMention {
		return s.dispatchAgentPublicMentionQueueItem(
			contextWithQueueOwner(ctx, item.OwnerUserID),
			sessionKey,
			roomID,
			conversationID,
			item,
			protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		)
	}
	return s.HandleChat(contextWithQueueOwner(ctx, item.OwnerUserID), ChatRequest{
		SessionKey:     sessionKey,
		RoomID:         roomID,
		ConversationID: conversationID,
		Content:        item.Content,
		RoundID:        "queue_" + item.ID,
		ReqID:          "queue_" + item.ID,
		DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
	})
}

func (s *RealtimeService) dispatchAgentPublicMentionQueueItem(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	item protocol.InputQueueItem,
	deliveryPolicy protocol.ChatDeliveryPolicy,
) error {
	targetAgentIDs := inputQueueTargetAgentIDs(item)
	if len(targetAgentIDs) == 0 {
		return errors.New("target_agent_ids is required")
	}
	content := strings.TrimSpace(item.Content)
	if content == "" {
		return errors.New("content is required")
	}
	if protocol.ShouldGuideRunningRound(deliveryPolicy) {
		guidedAgentIDs, err := s.guideActiveAgentSlots(ctx, sessionKey, conversationID, targetAgentIDs, content, "queue_"+item.ID)
		if err != nil {
			return err
		}
		if len(guidedAgentIDs) > 0 {
			s.broadcastSessionStatus(ctx, sessionKey)
			return nil
		}
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return err
	}
	wakes := make([]publicMentionWake, 0, len(targetAgentIDs))
	for _, targetAgentID := range targetAgentIDs {
		wakes = append(wakes, publicMentionWake{
			SourceAgentID: strings.TrimSpace(item.SourceAgentID),
			TargetAgentID: targetAgentID,
			Content:       content,
			MessageID:     firstNonEmpty(strings.TrimSpace(item.SourceMessageID), "queue_"+item.ID),
		})
	}
	parentRound := &activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         firstNonEmpty(strings.TrimSpace(roomID), contextValue.Room.ID),
		ConversationID: conversationID,
		RoomType:       contextValue.Room.RoomType,
		Context:        contextValue,
		RoundID:        firstNonEmpty(strings.TrimSpace(item.SourceMessageID), "queue_"+item.ID),
		RootRoundID:    strings.TrimSpace(item.RootRoundID),
		HopIndex:       item.HopIndex,
		OwnerUserID:    strings.TrimSpace(item.OwnerUserID),
	}
	return s.startPublicMentionRound(ctx, parentRound, wakes)
}

func (s *RealtimeService) canDispatchInputQueueItem(sessionKey string, conversationID string, item protocol.InputQueueItem) bool {
	if item.Source == protocol.InputQueueSourceAgentPublicMention {
		return len(s.findQueueSlots(sessionKey, conversationID, inputQueueTargetAgentIDs(item))) == 0
	}
	return len(s.runtime.GetRunningRoundIDs(sessionKey)) == 0
}

func (s *RealtimeService) canDispatchMoreInputQueueItems(ctx context.Context, sessionKey string, conversationID string) bool {
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil || contextValue == nil {
		return false
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil || len(entries) == 0 {
		return false
	}
	return s.canDispatchInputQueueItem(sessionKey, conversationID, entries[0].Item)
}

func (s *RealtimeService) resolveInputQueueContext(
	ctx context.Context,
	request InputQueueRequest,
) (string, *ConversationContextAggregate, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", nil, err
	}
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return "", nil, errors.New("session_key must be room shared key")
	}
	conversationID := firstNonEmpty(strings.TrimSpace(request.ConversationID), protocol.ParseRoomConversationID(sessionKey))
	if conversationID == "" {
		return "", nil, errors.New("conversation_id is required")
	}
	contextValue, err := s.rooms.GetConversationContext(ctx, conversationID)
	if err != nil {
		return "", nil, err
	}
	if contextValue == nil {
		return "", nil, errors.New("room conversation not found")
	}
	return sessionKey, contextValue, nil
}

func (s *RealtimeService) resolveRoomInputQueuePrimaryLocation(
	ctx context.Context,
	contextValue *ConversationContextAggregate,
	content string,
) (workspacestore.InputQueueLocation, []string, error) {
	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return workspacestore.InputQueueLocation{}, nil, err
	}
	targetAgentIDs := ResolveMentionAgentIDs(content, buildRoomMentionAliases(contextValue))
	if len(targetAgentIDs) == 0 && len(locationsByAgentID) == 1 {
		for agentID := range locationsByAgentID {
			targetAgentIDs = []string{agentID}
		}
	}
	if len(targetAgentIDs) == 0 {
		return workspacestore.InputQueueLocation{}, nil, errors.New("room input_queue content must mention target agent")
	}

	cleanTargets := make([]string, 0, len(targetAgentIDs))
	for _, agentID := range targetAgentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			continue
		}
		if _, ok := locationsByAgentID[agentID]; !ok {
			continue
		}
		cleanTargets = append(cleanTargets, agentID)
	}
	if len(cleanTargets) == 0 {
		return workspacestore.InputQueueLocation{}, nil, errors.New("room input_queue target agent not found")
	}
	return locationsByAgentID[cleanTargets[0]].Location, cleanTargets, nil
}

func (s *RealtimeService) roomInputQueueItems(ctx context.Context, contextValue *ConversationContextAggregate) ([]protocol.InputQueueItem, error) {
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	items := make([]protocol.InputQueueItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.Item)
	}
	return items, nil
}

func (s *RealtimeService) roomInputQueueEntries(ctx context.Context, contextValue *ConversationContextAggregate) ([]roomInputQueueEntry, error) {
	locations, err := s.roomInputQueueLocations(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	entries := make([]roomInputQueueEntry, 0)
	for _, location := range locations {
		items, snapshotErr := s.inputQueue.Snapshot(location.Location)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		for _, item := range items {
			entries = append(entries, roomInputQueueEntry{
				Item:     item,
				Location: location.Location,
			})
		}
	}
	sort.SliceStable(entries, func(i int, j int) bool {
		left := entries[i].Item
		right := entries[j].Item
		if left.QueueOrder != right.QueueOrder {
			return left.QueueOrder < right.QueueOrder
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt < right.CreatedAt
		}
		return left.ID < right.ID
	})
	return entries, nil
}

func (s *RealtimeService) findRoomInputQueueEntry(
	ctx context.Context,
	contextValue *ConversationContextAggregate,
	itemID string,
) (roomInputQueueEntry, bool, error) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return roomInputQueueEntry{}, false, nil
	}
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return roomInputQueueEntry{}, false, err
	}
	for _, entry := range entries {
		if entry.Item.ID == itemID {
			return entry, true, nil
		}
	}
	return roomInputQueueEntry{}, false, nil
}

func (s *RealtimeService) deleteRoomInputQueueItem(ctx context.Context, contextValue *ConversationContextAggregate, itemID string) error {
	entry, ok, err := s.findRoomInputQueueEntry(ctx, contextValue, itemID)
	if err != nil || !ok {
		return err
	}
	_, err = s.inputQueue.Delete(entry.Location, itemID)
	return err
}

func (s *RealtimeService) reorderRoomInputQueueItems(
	ctx context.Context,
	contextValue *ConversationContextAggregate,
	orderedIDs []string,
) error {
	entries, err := s.roomInputQueueEntries(ctx, contextValue)
	if err != nil {
		return err
	}
	locationByKey := make(map[string]workspacestore.InputQueueLocation)
	for _, entry := range entries {
		for _, orderedID := range orderedIDs {
			if entry.Item.ID != strings.TrimSpace(orderedID) {
				continue
			}
			locationByKey[inputQueueLocationKey(entry.Location)] = entry.Location
			break
		}
	}
	for _, location := range locationByKey {
		if _, err = s.inputQueue.Reorder(location, orderedIDs); err != nil {
			return err
		}
	}
	return nil
}

func (s *RealtimeService) roomInputQueueLocations(
	ctx context.Context,
	contextValue *ConversationContextAggregate,
) ([]roomInputQueueLocation, error) {
	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	locations := make([]roomInputQueueLocation, 0, len(locationsByAgentID))
	for _, member := range contextValue.Members {
		if member.MemberType != MemberTypeAgent {
			continue
		}
		if location, ok := locationsByAgentID[strings.TrimSpace(member.MemberAgentID)]; ok {
			locations = append(locations, location)
		}
	}
	sort.SliceStable(locations, func(i int, j int) bool {
		return locations[i].AgentID < locations[j].AgentID
	})
	return locations, nil
}

func (s *RealtimeService) roomInputQueueLocationsByAgent(
	ctx context.Context,
	contextValue *ConversationContextAggregate,
) (map[string]roomInputQueueLocation, error) {
	if contextValue == nil {
		return map[string]roomInputQueueLocation{}, nil
	}
	agentsByID := make(map[string]protocol.Agent, len(contextValue.MemberAgents))
	for _, agentValue := range contextValue.MemberAgents {
		agentID := strings.TrimSpace(agentValue.AgentID)
		if agentID != "" {
			agentsByID[agentID] = agentValue
		}
	}
	for _, member := range contextValue.Members {
		agentID := strings.TrimSpace(member.MemberAgentID)
		if member.MemberType != MemberTypeAgent || agentID == "" {
			continue
		}
		if _, exists := agentsByID[agentID]; exists {
			continue
		}
		agentValue, err := s.agents.GetAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		agentsByID[agentID] = *agentValue
	}

	result := make(map[string]roomInputQueueLocation, len(agentsByID))
	for agentID, agentValue := range agentsByID {
		workspacePath := strings.TrimSpace(agentValue.WorkspacePath)
		if workspacePath == "" {
			continue
		}
		result[agentID] = roomInputQueueLocation{
			AgentID: agentID,
			Location: workspacestore.InputQueueLocation{
				Scope:          protocol.InputQueueScopeRoom,
				WorkspacePath:  workspacePath,
				SessionKey:     protocol.BuildRoomAgentSessionKey(contextValue.Conversation.ID, agentID, contextValue.Room.RoomType),
				RoomID:         contextValue.Room.ID,
				ConversationID: contextValue.Conversation.ID,
			},
		}
	}
	return result, nil
}

func (s *RealtimeService) broadcastRoomInputQueueSnapshot(
	ctx context.Context,
	sessionKey string,
	contextValue *ConversationContextAggregate,
) error {
	items, err := s.roomInputQueueItems(ctx, contextValue)
	if err != nil {
		return err
	}
	s.broadcastInputQueueItems(ctx, sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, items)
	return nil
}

func (s *RealtimeService) broadcastInputQueueItems(
	ctx context.Context,
	sessionKey string,
	roomID string,
	conversationID string,
	items []protocol.InputQueueItem,
) {
	s.broadcastSharedEvent(ctx, sessionKey, roomID, newRoomInputQueueEvent(sessionKey, roomID, conversationID, items))
}

func newRoomInputQueueEvent(sessionKey string, roomID string, conversationID string, items []protocol.InputQueueItem) protocol.EventMessage {
	event := protocol.NewInputQueueEvent(sessionKey, items)
	event.Data["scope"] = string(protocol.InputQueueScopeRoom)
	event.RoomID = strings.TrimSpace(roomID)
	event.ConversationID = strings.TrimSpace(conversationID)
	return event
}

func inputQueueTargetAgentIDs(item protocol.InputQueueItem) []string {
	targets := make([]string, 0, len(item.TargetAgentIDs)+1)
	seen := make(map[string]struct{}, len(item.TargetAgentIDs)+1)
	appendTarget := func(agentID string) {
		agentID = strings.TrimSpace(agentID)
		if agentID == "" {
			return
		}
		if _, exists := seen[agentID]; exists {
			return
		}
		seen[agentID] = struct{}{}
		targets = append(targets, agentID)
	}
	appendTarget(item.AgentID)
	for _, agentID := range item.TargetAgentIDs {
		appendTarget(agentID)
	}
	return targets
}

func inputQueueLocationAgentID(location workspacestore.InputQueueLocation) string {
	return strings.TrimSpace(protocol.ParseSessionKey(location.SessionKey).AgentID)
}

func inputQueueLocationKey(location workspacestore.InputQueueLocation) string {
	return strings.TrimSpace(location.WorkspacePath) + "::" + strings.TrimSpace(location.SessionKey)
}

func contextWithQueueOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return ctx
	}
	if _, ok := authsvc.CurrentUserID(ctx); ok {
		return ctx
	}
	return authsvc.WithPrincipal(ctx, &authsvc.Principal{
		UserID: ownerUserID,
		Role:   authsvc.RoleOwner,
	})
}
