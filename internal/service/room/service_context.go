package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetConversationContext 暴露 Room conversation 聚合，供 automation 做目标成员校验。
func (s *RealtimeService) GetConversationContext(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	if s.rooms == nil {
		return nil, errors.New("room service is not configured")
	}
	return s.rooms.GetConversationContext(ctx, strings.TrimSpace(conversationID))
}
