package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListRooms 列出最近房间。
func (s *Service) ListRooms(ctx context.Context, limit int) ([]protocol.RoomAggregate, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.repository.ListRecentRooms(ctx, authctx.OwnerUserID(ctx), limit)
}

// GetRoom 读取单个房间。
func (s *Service) GetRoom(ctx context.Context, roomID string) (*protocol.RoomAggregate, error) {
	roomValue, err := s.repository.GetRoom(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	if roomValue == nil {
		return nil, ErrRoomNotFound
	}
	return roomValue, nil
}

// GetRoomContexts 读取房间全部上下文。
func (s *Service) GetRoomContexts(ctx context.Context, roomID string) ([]protocol.ConversationContextAggregate, error) {
	contexts, err := s.repository.GetRoomContexts(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	if len(contexts) == 0 {
		return nil, ErrRoomNotFound
	}
	return contexts, nil
}

// GetConversationContext 按 conversation_id 读取单条房间上下文。
func (s *Service) GetConversationContext(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	contextValue, err := s.repository.GetConversationContext(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(conversationID))
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	return contextValue, nil
}

// GetConversationContext 暴露 Room conversation 聚合，供 automation 做目标成员校验。
func (s *RealtimeService) GetConversationContext(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	if s.rooms == nil {
		return nil, errors.New("room service is not configured")
	}
	return s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
}
