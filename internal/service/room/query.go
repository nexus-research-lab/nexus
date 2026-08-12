package room

import (
	"context"
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

// GetRoomContexts 读取房间全部上下文，并用 canonical Room/workspace 历史补全消息计数。
func (s *Service) GetRoomContexts(ctx context.Context, roomID string) ([]protocol.ConversationContextAggregate, error) {
	contexts, err := s.repository.GetRoomContexts(ctx, authctx.OwnerUserID(ctx), strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	if len(contexts) == 0 {
		return nil, ErrRoomNotFound
	}
	if err = s.hydrateConversationMessageCounts(contexts); err != nil {
		return nil, err
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
	if err = s.hydrateConversationMessageCount(contextValue); err != nil {
		return nil, err
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
	if err = s.hydrateConversationMessageCount(contextValue); err != nil {
		return nil, err
	}
	return contextValue, nil
}

func (s *Service) hydrateConversationMessageCounts(
	contexts []protocol.ConversationContextAggregate,
) error {
	for index := range contexts {
		if err := s.hydrateConversationMessageCount(&contexts[index]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) hydrateConversationMessageCount(
	contextValue *protocol.ConversationContextAggregate,
) error {
	count, err := s.canonicalConversationMessageCount(contextValue)
	if err != nil {
		return err
	}
	if contextValue != nil && count > contextValue.Conversation.MessageCount {
		contextValue.Conversation.MessageCount = count
	}
	return nil
}

func (s *Service) canonicalConversationMessageCount(
	contextValue *protocol.ConversationContextAggregate,
) (int, error) {
	if contextValue == nil {
		return 0, nil
	}
	if contextValue.Room.RoomType != protocol.RoomTypeDM {
		return s.roomHistory.MessageCount(
			contextValue.Room.OwnerUserID,
			contextValue.Conversation.ID,
		)
	}
	for _, sessionValue := range contextValue.Sessions {
		if !sessionValue.IsPrimary || strings.TrimSpace(sessionValue.AgentID) == "" {
			continue
		}
		agentValue := findConversationMemberAgent(contextValue.MemberAgents, sessionValue.AgentID)
		if agentValue == nil || strings.TrimSpace(agentValue.WorkspacePath) == "" {
			continue
		}
		sessionKey := protocol.BuildRoomAgentSessionKey(
			contextValue.Conversation.ID,
			sessionValue.AgentID,
			protocol.RoomTypeDM,
		)
		fileSession, _, err := s.files.ForOwner(contextValue.Room.OwnerUserID).FindSession(
			[]string{agentValue.WorkspacePath},
			sessionKey,
		)
		if err != nil {
			return 0, err
		}
		if fileSession != nil {
			return fileSession.MessageCount, nil
		}
	}
	return 0, nil
}

func findConversationMemberAgent(items []protocol.Agent, agentID string) *protocol.Agent {
	for index := range items {
		if strings.TrimSpace(items[index].AgentID) == strings.TrimSpace(agentID) {
			return &items[index]
		}
	}
	return nil
}
