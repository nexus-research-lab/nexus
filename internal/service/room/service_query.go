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

// GetConversationContextForSystem 供内部系统续跑在没有请求主体时恢复 Room owner。
func (s *Service) GetConversationContextForSystem(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	contextValue, err := s.repository.GetConversationContextForSystem(ctx, strings.TrimSpace(conversationID))
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

func (s *RealtimeService) internalConversationContext(ctx context.Context, conversationID string, internal bool) (context.Context, *protocol.ConversationContextAggregate, error) {
	if s.rooms == nil {
		return ctx, nil, errors.New("room service is not configured")
	}
	if !internal {
		contextValue, err := s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
		return ctx, contextValue, err
	}
	if _, ok := authctx.CurrentUserID(ctx); ok {
		contextValue, err := s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
		return ctx, contextValue, err
	}
	contextValue, err := s.rooms.GetConversationContextForSystem(ctx, strings.TrimSpace(conversationID))
	if err != nil || contextValue == nil {
		return ctx, contextValue, err
	}
	ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
	if ownerUserID == "" {
		return ctx, contextValue, nil
	}
	return authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	}), contextValue, nil
}
