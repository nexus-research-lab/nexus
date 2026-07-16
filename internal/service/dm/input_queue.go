// INPUT: DM 待发送项、当前运行 round 与队列控制动作。
// OUTPUT: 幂等受理结果、串行派发、round 锚定的 guide 或错过 hook 后的下一轮接力。
// POS: DM 输入队列控制面与派发边界。
package dm

import (
	"context"
	"errors"
	"slices"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// InputQueueRequest 表示 DM 待发送队列控制请求。
type InputQueueRequest struct {
	SessionKey      string
	AgentID         string
	ClientMessageID string
	Action          string
	ItemID          string
	Content         string
	Attachments     []protocol.ChatAttachment
	OrderedIDs      []string
	DeliveryPolicy  protocol.ChatDeliveryPolicy
}

// HandleInputQueue 处理 DM 待发送队列控制消息。
func (s *Service) HandleInputQueue(
	ctx context.Context,
	request InputQueueRequest,
) (protocol.InputQueueMutationResult, error) {
	sessionKey, location, err := s.resolveInputQueueLocation(ctx, request.SessionKey, request.AgentID)
	if err != nil {
		return protocol.InputQueueMutationResult{}, err
	}
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()

	action := strings.TrimSpace(request.Action)
	if action == "" {
		action = "enqueue"
	}
	switch action {
	case "enqueue":
		content := strings.TrimSpace(request.Content)
		attachments := protocol.NormalizeChatAttachments(request.Attachments, request.AgentID)
		if !protocol.HasChatInput(content, attachments) {
			return protocol.InputQueueMutationResult{}, errors.New("content is required")
		}
		clientMessageID := strings.TrimSpace(request.ClientMessageID)
		if clientMessageID == "" {
			// 兼容尚未发送 ACK 关联字段的旧客户端；新客户端必须自行保持该 ID，
			// 才能在受理状态未知时获得跨重试幂等。
			clientMessageID = "legacy_" + workspacestore.NewInputQueueID()
		}
		ownerUserID := authctx.OwnerUserID(ctx)
		enqueueResult, err := s.inputQueue.EnqueueIdempotent(location, protocol.InputQueueItem{
			Scope:          protocol.InputQueueScopeDM,
			SessionKey:     sessionKey,
			AgentID:        inputQueueLocationAgentID(location),
			Source:         protocol.InputQueueSourceUser,
			Content:        content,
			Attachments:    attachments,
			DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy)),
			OwnerUserID:    ownerUserID,
		}, clientMessageID)
		if err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		if !enqueueResult.Duplicate {
			s.broadcastInputQueueSnapshot(ctx, sessionKey, enqueueResult.Items)
			go s.dispatchNextInputQueueItemAtLocation(
				contextWithQueueOwner(context.Background(), ownerUserID),
				sessionKey,
				request.AgentID,
				location,
			)
		}
		return protocol.InputQueueMutationResult{
			Action:    action,
			ItemID:    enqueueResult.Item.ID,
			Duplicate: enqueueResult.Duplicate,
		}, nil
	case "delete":
		if s.hasInFlightInputQueueGuidance(request.ItemID) {
			return protocol.InputQueueMutationResult{}, errors.New("该引导已发送给智能体，不能再删除")
		}
		items, err := s.inputQueue.Delete(location, request.ItemID)
		if err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		return protocol.InputQueueMutationResult{Action: action, ItemID: strings.TrimSpace(request.ItemID)}, nil
	case "reorder":
		for _, itemID := range request.OrderedIDs {
			if s.hasInFlightInputQueueGuidance(itemID) {
				return protocol.InputQueueMutationResult{}, errors.New("已发送给智能体的引导不能重排")
			}
		}
		items, err := s.inputQueue.Reorder(location, request.OrderedIDs)
		if err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		return protocol.InputQueueMutationResult{Action: action}, nil
	case "guide":
		if s.hasInFlightInputQueueGuidance(request.ItemID) {
			return protocol.InputQueueMutationResult{}, errors.New("该引导正在等待智能体确认，不能更改投递方式")
		}
		if err = s.guideInputQueueItem(ctx, sessionKey, location, request.ItemID); err != nil {
			return protocol.InputQueueMutationResult{}, err
		}
		return protocol.InputQueueMutationResult{Action: action, ItemID: strings.TrimSpace(request.ItemID)}, nil
	default:
		return protocol.InputQueueMutationResult{}, errors.New("unsupported input_queue action")
	}
}

// SendInputQueueSnapshot 向当前连接恢复 DM 待发送队列快照。
func (s *Service) SendInputQueueSnapshot(ctx context.Context, sessionKey string, agentID string) error {
	normalizedSessionKey, location, err := s.resolveInputQueueLocation(ctx, sessionKey, agentID)
	if err != nil {
		return err
	}
	items, err := s.inputQueue.Snapshot(location)
	if err != nil {
		return err
	}
	s.broadcastInputQueueSnapshot(ctx, normalizedSessionKey, items)
	go s.dispatchNextInputQueueItemAtLocation(context.Background(), normalizedSessionKey, agentID, location)
	return nil
}

func (s *Service) guideInputQueueItem(
	ctx context.Context,
	sessionKey string,
	location workspacestore.InputQueueLocation,
	itemID string,
) error {
	items, err := s.inputQueue.Snapshot(location)
	if err != nil {
		return err
	}
	var selected *protocol.InputQueueItem
	for _, item := range items {
		if item.ID == strings.TrimSpace(itemID) {
			copyItem := item
			selected = &copyItem
			break
		}
	}
	if selected == nil {
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		return nil
	}
	if protocol.ShouldGuideRunningRound(selected.DeliveryPolicy) {
		items, err = s.inputQueue.UpdateDeliveryPolicy(location, selected.ID, protocol.ChatDeliveryPolicyQueue)
		if err != nil {
			return err
		}
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		go s.dispatchNextInputQueueItemAtLocation(
			contextWithQueueOwner(context.Background(), selected.OwnerUserID),
			sessionKey,
			selected.AgentID,
			location,
		)
		return nil
	}
	runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
	if len(runningRoundIDs) == 0 {
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		return nil
	}
	targetRoundID := strings.TrimSpace(runningRoundIDs[0])
	items, err = s.inputQueue.UpdateDeliveryPolicy(location, selected.ID, protocol.ChatDeliveryPolicyGuide, targetRoundID)
	if err != nil {
		return err
	}
	items, recovered, err := s.recoverStaleInputQueueGuidance(location, selected.ID, targetRoundID, items)
	if err != nil {
		return err
	}
	s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
	if recovered {
		go s.dispatchNextInputQueueItemAtLocation(
			contextWithQueueOwner(context.Background(), selected.OwnerUserID),
			sessionKey,
			selected.AgentID,
			location,
		)
	}
	return nil
}

func (s *Service) recoverStaleInputQueueGuidance(
	location workspacestore.InputQueueLocation,
	itemID string,
	targetRoundID string,
	items []protocol.InputQueueItem,
) ([]protocol.InputQueueItem, bool, error) {
	if slices.Contains(s.runtime.GetRunningRoundIDs(location.SessionKey), strings.TrimSpace(targetRoundID)) {
		return items, false, nil
	}
	recovered, err := s.inputQueue.UpdateDeliveryPolicy(location, itemID, protocol.ChatDeliveryPolicyQueue)
	return recovered, err == nil, err
}

func (s *Service) dispatchNextInputQueueItemAtLocation(
	ctx context.Context,
	normalizedSessionKey string,
	agentID string,
	location workspacestore.InputQueueLocation,
) bool {
	s.inputQueueDispatchMu.Lock()
	defer s.inputQueueDispatchMu.Unlock()

	if strings.TrimSpace(normalizedSessionKey) == "" || len(s.runtime.GetRunningRoundIDs(normalizedSessionKey)) > 0 {
		return false
	}
	item, items, err := s.inputQueue.DispatchFirstDispatchable(location)
	if err != nil {
		s.loggerFor(ctx).Error("弹出 DM 待发送队列失败", "session_key", normalizedSessionKey, "err", err)
		return false
	}
	if item == nil {
		return false
	}
	s.broadcastInputQueueSnapshot(ctx, normalizedSessionKey, items)
	err = s.handleChat(contextWithQueueOwner(ctx, item.OwnerUserID), Request{
		SessionKey:           normalizedSessionKey,
		AgentID:              dmdomain.FirstNonEmpty(item.AgentID, inputQueueLocationAgentID(location)),
		Content:              item.Content,
		Attachments:          item.Attachments,
		RoundID:              inputQueueItemRoundID(*item),
		UserMessageID:        item.SourceMessageID,
		DeliveryPolicy:       protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		BroadcastUserMessage: true,
	})
	if err == nil {
		if len(s.runtime.GetRunningRoundIDs(normalizedSessionKey)) == 0 {
			go s.dispatchNextInputQueueItemAtLocation(
				ctx,
				normalizedSessionKey,
				dmdomain.FirstNonEmpty(item.AgentID, inputQueueLocationAgentID(location)),
				location,
			)
		}
		return true
	}
	s.loggerFor(ctx).Error("派发 DM 待发送队列失败",
		"session_key", normalizedSessionKey,
		"item_id", item.ID,
		"err", err,
	)
	if restored, restoreErr := s.inputQueue.Enqueue(location, *item); restoreErr != nil {
		s.loggerFor(ctx).Error("恢复 DM 待发送队列项失败",
			"session_key", normalizedSessionKey,
			"item_id", item.ID,
			"err", restoreErr,
		)
	} else {
		s.broadcastInputQueueSnapshot(ctx, normalizedSessionKey, restored)
	}
	message := "待发送消息派发失败"
	if clientMessage, ok := protocol.ClientErrorMessage(err); ok {
		message = clientMessage
	}
	s.broadcastEventWithTimeout(ctx, normalizedSessionKey, protocol.NewErrorEvent(normalizedSessionKey, message))
	return false
}

func inputQueueItemRoundID(item protocol.InputQueueItem) string {
	itemID := strings.TrimSpace(item.ID)
	if strings.TrimSpace(item.SourceMessageID) != "" ||
		strings.HasPrefix(itemID, "round_") ||
		strings.HasPrefix(itemID, "queue_") {
		return itemID
	}
	return "queue_" + itemID
}

func (s *Service) releaseUndeliveredInputQueueGuidance(
	ctx context.Context,
	sessionKey string,
	location workspacestore.InputQueueLocation,
	roundID string,
) {
	items, err := s.inputQueue.Snapshot(location)
	if err != nil {
		s.loggerFor(ctx).Warn("读取 DM 未消费引导失败", "session_key", sessionKey, "err", err)
		return
	}
	changed := false
	for _, item := range items {
		if !protocol.ShouldGuideRunningRound(item.DeliveryPolicy) {
			continue
		}
		rootRoundID := strings.TrimSpace(item.RootRoundID)
		if rootRoundID != "" && rootRoundID != strings.TrimSpace(roundID) {
			continue
		}
		items, err = s.inputQueue.UpdateDeliveryPolicy(location, item.ID, protocol.ChatDeliveryPolicyQueue)
		if err != nil {
			s.loggerFor(ctx).Warn("恢复 DM 未消费引导失败", "session_key", sessionKey, "item_id", item.ID, "err", err)
			continue
		}
		changed = true
	}
	if changed {
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
	}
}

func (s *Service) resolveInputQueueLocation(
	ctx context.Context,
	rawSessionKey string,
	requestAgentID string,
) (string, workspacestore.InputQueueLocation, error) {
	sessionKey, err := protocol.RequireStructuredSessionKey(rawSessionKey)
	if err != nil {
		return "", workspacestore.InputQueueLocation{}, err
	}
	parsed := protocol.ParseSessionKey(sessionKey)
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return "", workspacestore.InputQueueLocation{}, ErrRoomSessionNotImplemented
	}
	agentValue, err := s.resolveInputQueueAgent(ctx, parsed, requestAgentID)
	if err != nil {
		return "", workspacestore.InputQueueLocation{}, err
	}
	return sessionKey, workspacestore.InputQueueLocation{
		Scope:         protocol.InputQueueScopeDM,
		WorkspacePath: agentValue.WorkspacePath,
		SessionKey:    sessionKey,
	}, nil
}

func (s *Service) resolveInputQueueAgent(
	ctx context.Context,
	parsed protocol.SessionKey,
	requestAgentID string,
) (*protocol.Agent, error) {
	agentID := dmdomain.FirstNonEmpty(parsed.AgentID, requestAgentID)
	if agentID == "" {
		defaultAgent, err := s.agents.GetDefaultAgent(ctx)
		if err != nil {
			return nil, err
		}
		agentID = defaultAgent.AgentID
	}
	return s.agents.GetAgent(ctx, agentID)
}

func (s *Service) broadcastInputQueueSnapshot(
	ctx context.Context,
	sessionKey string,
	items []protocol.InputQueueItem,
) {
	event := protocol.NewInputQueueEvent(sessionKey, items)
	event.Data["scope"] = string(protocol.InputQueueScopeDM)
	s.broadcastEventWithTimeout(ctx, sessionKey, event)
}

func contextWithQueueOwner(ctx context.Context, ownerUserID string) context.Context {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return ctx
	}
	if _, ok := authctx.CurrentUserID(ctx); ok {
		return ctx
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID: ownerUserID,
		Role:   authctx.RoleOwner,
	})
}

func inputQueueLocationAgentID(location workspacestore.InputQueueLocation) string {
	parsed := protocol.ParseSessionKey(location.SessionKey)
	return strings.TrimSpace(parsed.AgentID)
}
