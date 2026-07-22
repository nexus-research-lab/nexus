// INPUT: Room conversation ID 与调用方身份。
// OUTPUT: 用户可见或系统恢复使用的 Room conversation 聚合。
// POS: 实时编排读取持久化 Room 上下文的唯一适配边界。
package realtime

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetConversationContext 暴露 Room conversation 聚合，供 automation 做目标成员校验。
func (s *Service) GetConversationContext(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	if s.rooms == nil {
		return nil, errors.New("room service is not configured")
	}
	return s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
}

func (s *Service) internalConversationContext(
	ctx context.Context,
	conversationID string,
	internal bool,
) (context.Context, *protocol.ConversationContextAggregate, error) {
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
