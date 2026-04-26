package chat

import (
	"context"
	"errors"
	"strings"

	agent3 "github.com/nexus-research-lab/nexus/internal/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// InputQueueRequest 表示 DM 待发送队列控制请求。
type InputQueueRequest struct {
	SessionKey     string
	AgentID        string
	Action         string
	ItemID         string
	Content        string
	OrderedIDs     []string
	DeliveryPolicy protocol.ChatDeliveryPolicy
}

// HandleInputQueue 处理 DM 待发送队列控制消息。
func (s *Service) HandleInputQueue(ctx context.Context, request InputQueueRequest) error {
	sessionKey, location, err := s.resolveInputQueueLocation(ctx, request.SessionKey, request.AgentID)
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
		items, err := s.inputQueue.Enqueue(location, protocol.InputQueueItem{
			Scope:          protocol.InputQueueScopeDM,
			SessionKey:     sessionKey,
			AgentID:        inputQueueLocationAgentID(location),
			Source:         protocol.InputQueueSourceUser,
			Content:        content,
			DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy)),
			OwnerUserID:    ownerUserIDFromContext(ctx),
		})
		if err != nil {
			return err
		}
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		go s.dispatchNextInputQueueItem(contextWithQueueOwner(context.Background(), ownerUserIDFromContext(ctx)), sessionKey, request.AgentID)
		return nil
	case "delete":
		items, err := s.inputQueue.Delete(location, request.ItemID)
		if err != nil {
			return err
		}
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		return nil
	case "reorder":
		items, err := s.inputQueue.Reorder(location, request.OrderedIDs)
		if err != nil {
			return err
		}
		s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
		return nil
	case "guide":
		return s.guideInputQueueItem(ctx, sessionKey, location, request.ItemID)
	default:
		return errors.New("unsupported input_queue action")
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
	go s.dispatchNextInputQueueItem(context.Background(), normalizedSessionKey, agentID)
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
	items, err = s.inputQueue.Delete(location, selected.ID)
	if err != nil {
		return err
	}
	s.broadcastInputQueueSnapshot(ctx, sessionKey, items)
	return s.HandleChat(contextWithQueueOwner(ctx, selected.OwnerUserID), Request{
		SessionKey:           sessionKey,
		AgentID:              firstNonEmpty(selected.AgentID, inputQueueLocationAgentID(location)),
		Content:              selected.Content,
		RoundID:              "queue_" + selected.ID,
		ReqID:                "queue_" + selected.ID,
		DeliveryPolicy:       protocol.ChatDeliveryPolicyGuide,
		BroadcastUserMessage: true,
	})
}

func (s *Service) dispatchNextInputQueueItem(ctx context.Context, sessionKey string, agentID string) {
	if strings.TrimSpace(sessionKey) == "" || len(s.runtime.GetRunningRoundIDs(sessionKey)) > 0 {
		return
	}
	normalizedSessionKey, location, err := s.resolveInputQueueLocation(ctx, sessionKey, agentID)
	if err != nil {
		s.loggerFor(ctx).Warn("解析 DM 待发送队列位置失败", "session_key", sessionKey, "err", err)
		return
	}
	item, items, err := s.inputQueue.DispatchNext(location)
	if err != nil {
		s.loggerFor(ctx).Error("弹出 DM 待发送队列失败", "session_key", normalizedSessionKey, "err", err)
		return
	}
	if item == nil {
		return
	}
	s.broadcastInputQueueSnapshot(ctx, normalizedSessionKey, items)
	err = s.HandleChat(contextWithQueueOwner(ctx, item.OwnerUserID), Request{
		SessionKey:           normalizedSessionKey,
		AgentID:              firstNonEmpty(item.AgentID, inputQueueLocationAgentID(location)),
		Content:              item.Content,
		RoundID:              "queue_" + item.ID,
		ReqID:                "queue_" + item.ID,
		DeliveryPolicy:       protocol.NormalizeChatDeliveryPolicy(string(item.DeliveryPolicy)),
		BroadcastUserMessage: true,
	})
	if err == nil {
		if len(s.runtime.GetRunningRoundIDs(normalizedSessionKey)) == 0 {
			go s.dispatchNextInputQueueItem(ctx, normalizedSessionKey, firstNonEmpty(item.AgentID, inputQueueLocationAgentID(location)))
		}
		return
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
	s.permission.BroadcastEvent(ctx, normalizedSessionKey, protocol.NewErrorEvent(normalizedSessionKey, "待发送消息派发失败"))
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
		return "", workspacestore.InputQueueLocation{}, ErrRoomChatNotImplemented
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
) (*agent3.Agent, error) {
	agentID := firstNonEmpty(parsed.AgentID, requestAgentID)
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
	s.permission.BroadcastEvent(ctx, sessionKey, event)
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

func inputQueueLocationAgentID(location workspacestore.InputQueueLocation) string {
	parsed := protocol.ParseSessionKey(location.SessionKey)
	return strings.TrimSpace(parsed.AgentID)
}
