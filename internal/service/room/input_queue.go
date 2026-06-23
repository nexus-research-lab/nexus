package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
	Attachments    []protocol.ChatAttachment
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
		attachments := s.normalizeChatAttachments(request.Attachments, "", contextValue.Room.ID, contextValue.Conversation.ID)
		if !protocol.HasChatInput(content, attachments) {
			return errors.New("content is required")
		}
		location, targetAgentIDs, err := s.resolveRoomInputQueuePrimaryLocation(ctx, contextValue, content)
		if err != nil {
			return err
		}
		ownerUserID := authctx.OwnerUserID(ctx)
		if _, err = s.inputQueue.Enqueue(location, protocol.InputQueueItem{
			Scope:          protocol.InputQueueScopeRoom,
			SessionKey:     location.SessionKey,
			RoomID:         contextValue.Room.ID,
			ConversationID: contextValue.Conversation.ID,
			AgentID:        inputQueueLocationAgentID(location),
			TargetAgentIDs: targetAgentIDs,
			Source:         protocol.InputQueueSourceUser,
			Content:        content,
			Attachments:    attachments,
			DeliveryPolicy: protocol.NormalizeChatDeliveryPolicy(string(request.DeliveryPolicy)),
			OwnerUserID:    ownerUserID,
		}); err != nil {
			return err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return err
		}
		go s.dispatchNextInputQueueItem(contextWithQueueOwner(context.Background(), ownerUserID), sessionKey, contextValue.Room.ID, contextValue.Conversation.ID)
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
	if protocol.ShouldGuideRunningRound(entry.Item.DeliveryPolicy) {
		if _, err = s.inputQueue.UpdateDeliveryPolicy(entry.Location, entry.Item.ID, protocol.ChatDeliveryPolicyQueue); err != nil {
			return err
		}
		if err = s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue); err != nil {
			return err
		}
		go s.dispatchNextInputQueueItem(
			contextWithQueueOwner(context.Background(), entry.Item.OwnerUserID),
			sessionKey,
			contextValue.Room.ID,
			contextValue.Conversation.ID,
		)
		return nil
	}
	activeSlot := s.inputQueueGuidanceTargetSlot(sessionKey, contextValue.Conversation.ID, entry)
	if activeSlot == nil {
		return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
	}
	if _, err = s.inputQueue.UpdateDeliveryPolicy(entry.Location, entry.Item.ID, protocol.ChatDeliveryPolicyGuide); err != nil {
		return err
	}
	return s.broadcastRoomInputQueueSnapshot(ctx, sessionKey, contextValue)
}
