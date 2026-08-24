// INPUT: Room 输入队列控制请求与持久化队列快照。
// OUTPUT: 跨成员幂等受理与可恢复 admission、队列变更、guide 消费轮身份同步和共享快照事件。
// POS: Room 用户输入队列的控制面。
package realtime

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"slices"
	"sort"
	"strings"
	"time"
)

// InputQueueRequest 表示 Room 待发送队列控制请求。
type InputQueueRequest struct {
	SessionKey      string
	RoomID          string
	ConversationID  string
	ClientMessageID string
	Action          string
	ItemID          string
	Content         string
	Attachments     []protocol.ChatAttachment
	TargetAgentIDs  []string
	OrderedIDs      []string
	DeliveryPolicy  protocol.ChatDeliveryPolicy
	// TrustedConfigurationContext 仅由认证 WebSocket adapter 设置。
	TrustedConfigurationContext bool
}

type roomInputQueueLocation struct {
	AgentID  string
	Location workspacestore.InputQueueLocation
}

type roomInputQueueEntry struct {
	Item     protocol.InputQueueItem
	Location workspacestore.InputQueueLocation
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
	return protocol.ParseSessionKey(location.SessionKey).AgentID
}

func inputQueueLocationKey(location workspacestore.InputQueueLocation) string {
	return strings.TrimSpace(location.WorkspacePath) + "::" + strings.TrimSpace(location.SessionKey)
}

func (s *Service) broadcastRoomInputQueueSnapshot(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
) error {
	items, err := s.roomInputQueueItems(ctx, contextValue)
	if err != nil {
		return err
	}
	s.broadcastInputQueueItems(ctx, sessionKey, contextValue.Room.ID, contextValue.Conversation.ID, items)
	return nil
}

func (s *Service) broadcastInputQueueItems(
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

// HandleInputQueue 处理 Room 待发送队列控制消息。
func (s *Service) HandleInputQueue(
	ctx context.Context,
	request InputQueueRequest,
) (protocol.InputQueueMutationResult, error) {
	sessionKey, contextValue, err := s.resolveInputQueueContext(ctx, request)
	if err != nil {
		return protocol.InputQueueMutationResult{}, err
	}

	action := strings.TrimSpace(request.Action)
	if action == "" {
		action = "enqueue"
	}
	lease := s.lockRoomDispatch(sessionKey, contextValue.Conversation.ID)
	defer lease.Unlock()
	switch action {
	case "enqueue":
		content := strings.TrimSpace(request.Content)
		attachments := s.normalizeChatAttachments(request.Attachments, "", contextValue.Room.ID, contextValue.Conversation.ID)
		if !protocol.HasChatInput(content, attachments) {
			return protocol.InputQueueMutationResult{}, errors.New("content is required")
		}
		clientMessageID := strings.TrimSpace(request.ClientMessageID)
		if clientMessageID == "" {
			// 兼容尚未发送 ACK 关联字段的旧客户端；只有新客户端提供并复用
			// 稳定 ID 时，才能获得跨重试和即时派发后的持久幂等。
			clientMessageID = "legacy_" + workspacestore.NewInputQueueID()
		}
		acceptedEntry, accepted, err := s.findAcceptedRoomInputQueueEnqueue(
			ctx,
			contextValue,
			clientMessageID,
		)
		if err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		ownerUserID := authctx.OwnerUserID(ctx)
		candidate := protocol.InputQueueItem{
			Scope:          protocol.InputQueueScopeRoom,
			RoomID:         contextValue.Room.ID,
			ConversationID: contextValue.Conversation.ID,
			Source:         protocol.InputQueueSourceUser,
			Content:        content,
			Attachments:    attachments,
			DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy)),
			OwnerUserID:    ownerUserID,
		}
		if accepted {
			candidate.SessionKey = acceptedEntry.Location.SessionKey
			candidate.AgentID = acceptedEntry.Item.AgentID
			if len(request.TargetAgentIDs) > 0 {
				candidate.TargetAgentIDs = normalizeExplicitTargetAgentIDs(request.TargetAgentIDs)
			} else {
				candidate.TargetAgentIDs = acceptedEntry.Item.TargetAgentIDs
			}
			if !workspacestore.MatchesInputQueueEnqueueIntent(acceptedEntry.Item, candidate) {
				return protocol.InputQueueMutationResult{}, workspacestore.ErrInputQueueIdempotencyConflict
			}
			if err = s.recordTrustedRoomQueueAdmission(
				ctx,
				acceptedEntry.Location,
				acceptedEntry.Item,
				request.TrustedConfigurationContext,
			); err != nil {
				return protocol.InputQueueMutationResult{}, err
			}
			currentItems, snapshotErr := s.inputQueue.Snapshot(acceptedEntry.Location)
			if snapshotErr != nil {
				return protocol.InputQueueMutationResult{}, snapshotErr
			}
			pending := slices.ContainsFunc(currentItems, func(item protocol.InputQueueItem) bool {
				return item.ID == acceptedEntry.Item.ID
			})
			if pending {
				if broadcastErr := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); broadcastErr != nil {
					s.loggerFor(ctx).Warn("广播恢复受理的 Room input_queue 快照失败",
						"session_key", sessionKey,
						"item_id", acceptedEntry.Item.ID,
						"err", broadcastErr,
					)
				}
				s.startSessionBackgroundTask(
					sessionKey,
					ownerUserID,
					func(taskCtx context.Context) {
						s.dispatchNextInputQueueItem(taskCtx, sessionKey, contextValue.Room.ID, contextValue.Conversation.ID)
					},
				)
			}
			return protocol.InputQueueMutationResult{
				Action:    action,
				ItemID:    acceptedEntry.Item.ID,
				Duplicate: true,
			}, nil
		}
		location, targetAgentIDs, err := s.resolveRoomInputQueuePrimaryLocation(
			ctx,
			contextValue,
			content,
			request.TargetAgentIDs,
		)
		if err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		candidate.SessionKey = location.SessionKey
		candidate.AgentID = inputQueueLocationAgentID(location)
		candidate.TargetAgentIDs = targetAgentIDs
		enqueueResult, err := s.inputQueue.EnqueueIdempotent(location, candidate, clientMessageID)
		if err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		if err = s.recordTrustedRoomQueueAdmission(
			ctx,
			location,
			enqueueResult.Item,
			request.TrustedConfigurationContext,
		); err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		if !enqueueResult.Duplicate {
			if broadcastErr := s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); broadcastErr != nil {
				s.loggerFor(ctx).Warn("广播已受理的 Room input_queue 快照失败",
					"session_key", sessionKey,
					"item_id", enqueueResult.Item.ID,
					"err", broadcastErr,
				)
			}
			s.startSessionBackgroundTask(
				sessionKey,
				ownerUserID,
				func(taskCtx context.Context) {
					s.dispatchNextInputQueueItem(taskCtx, sessionKey, contextValue.Room.ID, contextValue.Conversation.ID)
				},
			)
		}
		return protocol.InputQueueMutationResult{
			Action:    action,
			ItemID:    enqueueResult.Item.ID,
			Duplicate: enqueueResult.Duplicate,
		}, nil
	case "delete":
		if s.hasInFlightRoomGuidance(request.ItemID) {
			return protocol.InputQueueMutationResult{}, errors.New("该引导已发送给智能体，不能再删除")
		}
		if err = s.deleteRoomInputQueueItem(ctx, contextValue, request.ItemID); err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		return protocol.InputQueueMutationResult{Action: action, ItemID: strings.TrimSpace(request.ItemID)}, nil
	case "reorder":
		for _, itemID := range request.OrderedIDs {
			if s.hasInFlightRoomGuidance(itemID) {
				return protocol.InputQueueMutationResult{}, errors.New("已发送给智能体的引导不能重排")
			}
		}
		if err = s.reorderRoomInputQueueItems(ctx, contextValue, request.OrderedIDs); err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		return protocol.InputQueueMutationResult{Action: action}, nil
	case "guide":
		if s.hasInFlightRoomGuidance(request.ItemID) {
			return protocol.InputQueueMutationResult{}, errors.New("该引导正在等待智能体确认，不能更改投递方式")
		}
		if err = s.guideInputQueueItem(ctx, sessionKey, contextValue, request.ItemID); err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		return protocol.InputQueueMutationResult{Action: action, ItemID: strings.TrimSpace(request.ItemID)}, nil
	default:
		return protocol.InputQueueMutationResult{}, errors.New("unsupported input_queue action")
	}
}

// InputQueueSnapshotEvent 构造 Room 队列快照事件，供新订阅连接恢复状态。
func (s *Service) InputQueueSnapshotEvent(
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
	if contextValue.Room.RoomType != protocol.RoomTypeGroup {
		return newRoomInputQueueEvent(sessionKey, roomID, conversationID, nil), nil
	}
	var items []protocol.InputQueueItem
	lease := s.lockRoomDispatch(sessionKey, conversationID)
	func() {
		defer lease.Unlock()
		s.releaseUndeliveredRoomGuidanceLocked(ctx, sessionKey, contextValue)
		items, err = s.roomInputQueueItems(ctx, contextValue)
	}()
	if err != nil {
		return protocol.EventMessage{}, err
	}
	event := newRoomInputQueueEvent(sessionKey, strings.TrimSpace(roomID), strings.TrimSpace(conversationID), items)
	s.startSessionBackgroundTask(
		sessionKey,
		contextValue.Room.OwnerUserID,
		func(taskCtx context.Context) {
			s.dispatchNextInputQueueItem(taskCtx, sessionKey, roomID, conversationID)
		},
	)
	return event, nil
}

func (s *Service) guideInputQueueItem(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	itemID string,
) error {
	entry, ok, err := s.findRoomInputQueueEntry(ctx, contextValue, itemID)
	if err != nil {
		return err
	}
	if !ok {
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	}
	if err = rejectGenericControlForBoundQueueItem(entry.Item, "guide"); err != nil {
		return err
	}
	if protocol.NormalizeGoalCollaborationBinding(entry.Item.GoalCollaborationBinding) != nil {
		return errors.New("Goal collaboration handoff must remain an independently attributable queued round")
	}
	if protocol.ShouldGuideRunningRound(entry.Item.DeliveryPolicy) {
		if _, err = s.inputQueue.UpdateDeliveryPolicy(entry.Location, entry.Item.ID, protocol.ChatDeliveryPolicyQueue); err != nil {
			return err
		}
		entry.Item.DeliveryPolicy = protocol.ChatDeliveryPolicyQueue
		if err = s.syncQueuedPublicUserMessage(ctx, sessionKey, contextValue, entry.Item, "", false); err != nil {
			return err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return err
		}
		s.startSessionBackgroundTask(
			sessionKey,
			entry.Item.OwnerUserID,
			func(taskCtx context.Context) {
				s.dispatchNextInputQueueItem(
					taskCtx,
					sessionKey,
					contextValue.Room.ID,
					contextValue.Conversation.ID,
				)
			},
		)
		return nil
	}
	activeSlot := s.inputQueueGuidanceTargetSlot(sessionKey, contextValue.Conversation.ID, entry)
	if activeSlot == nil {
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	}
	if _, err = s.inputQueue.UpdateDeliveryPolicy(
		entry.Location,
		entry.Item.ID,
		protocol.ChatDeliveryPolicyGuide,
		activeSlot.AgentRoundID,
	); err != nil {
		return err
	}
	entry.Item.DeliveryPolicy = protocol.ChatDeliveryPolicyGuide
	entry.Item.RootRoundID = activeSlot.AgentRoundID
	if err = s.syncQueuedPublicUserMessage(ctx, sessionKey, contextValue, entry.Item, "", false); err != nil {
		return err
	}
	return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
}

func (s *Service) syncQueuedPublicUserMessage(
	ctx context.Context,
	sessionKey string,
	contextValue *protocol.ConversationContextAggregate,
	item protocol.InputQueueItem,
	rootRoundID string,
	materialize bool,
) error {
	if contextValue == nil || s.roomHistory == nil {
		return nil
	}
	sourceRoundID := roomInputQueueSourceRoundID(item)
	rootRoundID = strings.TrimSpace(rootRoundID)
	targetAgentIDs := inputQueueTargetAgentIDs(item)
	consumingAgentRoundID := ""
	if materialize && protocol.ShouldGuideRunningRound(item.DeliveryPolicy) && len(targetAgentIDs) == 1 {
		consumingAgentRoundID = strings.TrimSpace(item.RootRoundID)
	}
	userMessageID := strings.TrimSpace(item.SourceMessageID)
	if userMessageID == "" {
		userMessageID = "msg_user_" + sourceRoundID
	}
	messages, err := s.roomHistory.ReadMessages(
		contextValue.Room.OwnerUserID,
		contextValue.Conversation.ID,
		nil,
	)
	if err != nil {
		return err
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		messageRoundID := protocol.MessageRoundID(message)
		messageSourceRoundID, _ := message["source_round_id"].(string)
		if protocol.MessageRole(message) != "user" ||
			(message["message_id"] != userMessageID && messageRoundID != sourceRoundID &&
				strings.TrimSpace(messageSourceRoundID) != sourceRoundID) {
			continue
		}
		updated := protocol.Clone(message)
		updated["delivery_policy"] = string(item.DeliveryPolicy)
		messageTargets := roomMessageTargetAgentIDs(message["target_agent_ids"])
		updatedTargets := mergeRoomMessageTargetAgentIDs(messageTargets, targetAgentIDs)
		if len(updatedTargets) > 0 {
			updated["target_agent_ids"] = updatedTargets
		}
		messageAgentRoundID, _ := message["agent_round_id"].(string)
		messageAgentRoundID = strings.TrimSpace(messageAgentRoundID)
		if len(updatedTargets) > 1 {
			delete(updated, "agent_round_id")
		} else if consumingAgentRoundID != "" && messageAgentRoundID == "" {
			updated["agent_round_id"] = consumingAgentRoundID
		}
		annotateRoomUserMessage(contextValue, updated)
		// 第一位消费者确定公开用户消息的归组；其他 root 只聚合消费目标，
		// 不能让同一条消息在时间线中随最后完成的 Agent 来回移动。
		if rootRoundID != "" && rootRoundID != sourceRoundID &&
			strings.TrimSpace(messageSourceRoundID) == "" &&
			(messageRoundID == "" || messageRoundID == sourceRoundID) {
			updated["source_round_id"] = sourceRoundID
			updated["round_id"] = rootRoundID
		}
		messagePolicy, _ := message["delivery_policy"].(string)
		updatedPolicy, _ := updated["delivery_policy"].(string)
		updatedSourceRoundID, _ := updated["source_round_id"].(string)
		updatedAgentRoundID, _ := updated["agent_round_id"].(string)
		updatedAgentRoundID = strings.TrimSpace(updatedAgentRoundID)
		if protocol.MessageRoundID(updated) == messageRoundID &&
			strings.TrimSpace(messagePolicy) == strings.TrimSpace(updatedPolicy) &&
			strings.TrimSpace(messageSourceRoundID) == strings.TrimSpace(updatedSourceRoundID) &&
			messageAgentRoundID == updatedAgentRoundID &&
			slices.Equal(messageTargets, updatedTargets) {
			if item.Source == protocol.InputQueueSourceUser {
				return s.markConversationStarted(
					ctx,
					contextValue.Conversation.ID,
					roomMessageActivityTime(message),
				)
			}
			return nil
		}
		if err = s.persistSharedInlineMessage(
			contextValue.Room.OwnerUserID,
			contextValue.Conversation.ID,
			updated,
		); err != nil {
			return err
		}
		if item.Source == protocol.InputQueueSourceUser {
			if err = s.markConversationStarted(
				ctx,
				contextValue.Conversation.ID,
				roomMessageActivityTime(updated),
			); err != nil {
				return err
			}
		}
		s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapMessageEvent(
			contextValue.Room.ID,
			contextValue.Conversation.ID,
			updated,
			protocol.MessageRoundID(updated),
		))
		return nil
	}
	if !materialize || item.Source != protocol.InputQueueSourceUser || sourceRoundID == "" {
		return nil
	}
	messageRoundID := sourceRoundID
	messageValue := protocol.Message{
		"message_id":      userMessageID,
		"session_key":     strings.TrimSpace(sessionKey),
		"room_id":         contextValue.Room.ID,
		"conversation_id": contextValue.Conversation.ID,
		"agent_id":        "",
		"round_id":        sourceRoundID,
		"role":            "user",
		"content":         strings.TrimSpace(item.Content),
		"timestamp":       time.Now().UnixMilli(),
		"delivery_policy": string(item.DeliveryPolicy),
	}
	if rootRoundID != "" && rootRoundID != sourceRoundID {
		messageRoundID = rootRoundID
		messageValue["source_round_id"] = sourceRoundID
		messageValue["round_id"] = rootRoundID
	}
	if consumingAgentRoundID != "" {
		messageValue["agent_round_id"] = consumingAgentRoundID
	}
	if len(targetAgentIDs) > 0 {
		messageValue["target_agent_ids"] = targetAgentIDs
	}
	annotateRoomUserMessage(contextValue, messageValue)
	if attachments := protocol.NormalizeChatAttachments(item.Attachments, ""); len(attachments) > 0 {
		messageValue["attachments"] = attachments
	}
	if err = s.persistSharedInlineMessage(
		contextValue.Room.OwnerUserID,
		contextValue.Conversation.ID,
		messageValue,
	); err != nil {
		return err
	}
	if err = s.markConversationStarted(
		ctx,
		contextValue.Conversation.ID,
		roomMessageActivityTime(messageValue),
	); err != nil {
		return err
	}
	s.broadcastSharedEvent(ctx, sessionKey, contextValue.Room.ID, roomdomain.WrapMessageEvent(
		contextValue.Room.ID,
		contextValue.Conversation.ID,
		messageValue,
		messageRoundID,
	))
	return nil
}

func roomInputQueueSourceRoundID(item protocol.InputQueueItem) string {
	if itemID := strings.TrimSpace(item.ID); itemID != "" {
		if strings.TrimSpace(item.SourceMessageID) != "" {
			return itemID
		}
		return "queue_" + itemID
	}
	return strings.TrimSpace(item.SourceMessageID)
}

func roomMessageTargetAgentIDs(value any) []string {
	result := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		result = append(result, typed...)
	case []any:
		for _, item := range typed {
			if agentID, ok := item.(string); ok {
				result = append(result, agentID)
			}
		}
	}
	return mergeRoomMessageTargetAgentIDs(nil, result)
}

func mergeRoomMessageTargetAgentIDs(current []string, incoming []string) []string {
	result := make([]string, 0, len(current)+len(incoming))
	seen := make(map[string]struct{}, len(current)+len(incoming))
	for _, values := range [][]string{current, incoming} {
		for _, agentID := range values {
			agentID = strings.TrimSpace(agentID)
			if agentID == "" {
				continue
			}
			if _, ok := seen[agentID]; ok {
				continue
			}
			seen[agentID] = struct{}{}
			result = append(result, agentID)
		}
	}
	return result
}

// INPUT: Room queue 请求、成员会话、@mention 与当前活跃 root round。
// OUTPUT: 显式 @ / 单成员 / 房主优先，仍无目标时绑定最近活跃 slots 的队列位置。
// POS: Room input queue 入队目标解析入口；与直接 chat 共享显式目标和房主优先语义。
func (s *Service) resolveInputQueueContext(
	ctx context.Context,
	request InputQueueRequest,
) (string, *protocol.ConversationContextAggregate, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(request.SessionKey)
	if err != nil {
		return "", nil, err
	}
	if !protocol.IsRoomSharedSessionKey(sessionKey) {
		return "", nil, errors.New("session_key must be room shared key")
	}
	conversationID := cmp.Or(strings.TrimSpace(request.ConversationID), protocol.ParseRoomConversationID(sessionKey))
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
	if err = requireGroupRoomContext(contextValue); err != nil {
		return "", nil, err
	}
	return sessionKey, contextValue, nil
}

func (s *Service) resolveRoomInputQueuePrimaryLocation(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	content string,
	explicitTargetAgentIDs []string,
) (workspacestore.InputQueueLocation, []string, error) {
	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return workspacestore.InputQueueLocation{}, nil, err
	}
	targetAgentIDs := normalizeExplicitTargetAgentIDs(explicitTargetAgentIDs)
	if len(explicitTargetAgentIDs) > 0 {
		if len(targetAgentIDs) == 0 {
			return workspacestore.InputQueueLocation{}, nil, errors.New("target_agent_ids must not be empty")
		}
		for _, agentID := range targetAgentIDs {
			if !roomdomain.IsMemberAgent(contextValue.Members, agentID) {
				return workspacestore.InputQueueLocation{}, nil, fmt.Errorf("target_agent_id is not a room member: %s", agentID)
			}
		}
	} else {
		targetAgentIDs = roomdomain.ResolveMentionAgentIDs(content, roomdomain.BuildMentionAliases(contextValue))
	}
	if len(targetAgentIDs) == 0 && len(locationsByAgentID) == 1 {
		for agentID := range locationsByAgentID {
			targetAgentIDs = []string{agentID}
		}
	}
	if len(targetAgentIDs) == 0 {
		if hostAgentID, ok := resolveRoomHostDefaultTarget(contextValue, agentNameByIDFromInputLocations(locationsByAgentID)); ok {
			targetAgentIDs = []string{hostAgentID}
		}
	}
	if len(targetAgentIDs) == 0 {
		targetAgentIDs = s.latestActiveRootRoundAgentIDs(
			protocol.BuildRoomSharedSessionKey(contextValue.Conversation.ID),
			contextValue.Conversation.ID,
		)
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

func agentNameByIDFromInputLocations(locations map[string]roomInputQueueLocation) map[string]string {
	result := make(map[string]string, len(locations))
	for agentID := range locations {
		normalizedAgentID := strings.TrimSpace(agentID)
		if normalizedAgentID == "" {
			continue
		}
		result[normalizedAgentID] = normalizedAgentID
	}
	return result
}

func (s *Service) roomInputQueueLocations(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) ([]roomInputQueueLocation, error) {
	locationsByAgentID, err := s.roomInputQueueLocationsByAgent(ctx, contextValue)
	if err != nil {
		return nil, err
	}
	locations := make([]roomInputQueueLocation, 0, len(locationsByAgentID))
	for _, member := range contextValue.Members {
		if member.MemberType != protocol.MemberTypeAgent {
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

func (s *Service) roomInputQueueLocationsByAgent(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
) (map[string]roomInputQueueLocation, error) {
	if contextValue == nil {
		return map[string]roomInputQueueLocation{}, nil
	}
	if contextValue.Room.RoomType == protocol.RoomTypeDM {
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
		if member.MemberType != protocol.MemberTypeAgent || agentID == "" {
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
				OwnerUserID:    contextValue.Room.OwnerUserID,
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

func (s *Service) roomInputQueueItems(ctx context.Context, contextValue *protocol.ConversationContextAggregate) ([]protocol.InputQueueItem, error) {
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

func (s *Service) roomInputQueueEntries(ctx context.Context, contextValue *protocol.ConversationContextAggregate) ([]roomInputQueueEntry, error) {
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

func (s *Service) findRoomInputQueueEntry(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
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

func (s *Service) findAcceptedRoomInputQueueEnqueue(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
	clientMessageID string,
) (roomInputQueueEntry, bool, error) {
	clientMessageID = strings.TrimSpace(clientMessageID)
	if clientMessageID == "" {
		return roomInputQueueEntry{}, false, nil
	}
	locations, err := s.roomInputQueueLocations(ctx, contextValue)
	if err != nil {
		return roomInputQueueEntry{}, false, err
	}
	for _, location := range locations {
		item, accepted, findErr := s.inputQueue.FindAcceptedEnqueue(location.Location, clientMessageID)
		if findErr != nil {
			return roomInputQueueEntry{}, false, findErr
		}
		if accepted {
			return roomInputQueueEntry{Item: item, Location: location.Location}, true, nil
		}
	}
	return roomInputQueueEntry{}, false, nil
}

func (s *Service) deleteRoomInputQueueItem(ctx context.Context, contextValue *protocol.ConversationContextAggregate, itemID string) error {
	entry, ok, err := s.findRoomInputQueueEntry(ctx, contextValue, itemID)
	if err != nil || !ok {
		return err
	}
	if err = rejectGenericControlForBoundQueueItem(entry.Item, "delete"); err != nil {
		return err
	}
	if err = s.revokeRoomQueueAdmission(ctx, entry.Location, entry.Item); err != nil {
		return err
	}
	_, err = s.inputQueue.Delete(entry.Location, itemID)
	return err
}

func (s *Service) reorderRoomInputQueueItems(
	ctx context.Context,
	contextValue *protocol.ConversationContextAggregate,
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
	for _, entry := range entries {
		if _, affected := locationByKey[inputQueueLocationKey(entry.Location)]; !affected {
			continue
		}
		if err = rejectGenericControlForBoundQueueItem(entry.Item, "reorder"); err != nil {
			return err
		}
	}
	for _, location := range locationByKey {
		if _, err = s.inputQueue.Reorder(location, orderedIDs); err != nil {
			return err
		}
	}
	return nil
}

func rejectGenericControlForBoundQueueItem(
	item protocol.InputQueueItem,
	action string,
) error {
	if item.WorkBinding == nil && item.ReviewBinding == nil {
		return nil
	}
	return fmt.Errorf(
		"%w: %s cannot alter an execution-bound queue item; use the WorkGraph lifecycle",
		protocol.ErrInvalidInputQueueCapabilityEnvelope,
		strings.TrimSpace(action),
	)
}
