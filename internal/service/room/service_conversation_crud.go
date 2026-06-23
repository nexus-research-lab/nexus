package room

import (
	"context"
	"errors"
	"strings"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// CreateConversation 创建 room 话题。
func (s *Service) CreateConversation(ctx context.Context, roomID string, request protocol.CreateConversationRequest) (*protocol.ConversationContextAggregate, error) {
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	roomValue := contexts[0].Room

	agentIDs := roomdomain.ListAgentIDs(contexts[0].Members)
	agentRefs, err := s.loadAgentRefs(ctx, agentIDs)
	if err != nil {
		return nil, err
	}

	nextTitle := roomdomain.NormalizeOptionalText(request.Title)
	if nextTitle == "" {
		nextTitle = roomdomain.BuildNextConversationTitle(roomValue.Name, contexts)
	}

	conversationID := roomdomain.NewEntityID()
	contextValue, err := s.repository.CreateConversation(ctx, roomrepo.CreateConversationBundle{
		RoomID: roomValue.ID,
		Conversation: protocol.ConversationRecord{
			ID:               conversationID,
			RoomID:           roomValue.ID,
			ConversationType: protocol.ConversationTypeTopic,
			Title:            nextTitle,
		},
		Sessions: roomdomain.BuildSessions(conversationID, agentRefs),
	})
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}

// UpdateConversation 更新 room 话题标题。
func (s *Service) UpdateConversation(ctx context.Context, roomID string, conversationID string, request protocol.UpdateConversationRequest) (*protocol.ConversationContextAggregate, error) {
	title := roomdomain.NormalizeOptionalText(request.Title)
	if title == "" {
		return nil, errors.New("对话标题不能为空")
	}
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if !roomdomain.HasConversation(contexts, conversationID) {
		return nil, ErrConversationNotFound
	}
	contextValue, err := s.repository.UpdateConversation(
		ctx,
		authctx.OwnerUserID(ctx),
		strings.TrimSpace(roomID),
		strings.TrimSpace(conversationID),
		title,
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	return contextValue, nil
}

// UpdateConversationTitle 以最小输入更新对话标题，供跨领域服务复用。
func (s *Service) UpdateConversationTitle(
	ctx context.Context,
	roomID string,
	conversationID string,
	title string,
) (*protocol.ConversationContextAggregate, error) {
	return s.UpdateConversation(ctx, roomID, conversationID, protocol.UpdateConversationRequest{Title: title})
}

// DeleteConversation 删除 room 话题并返回回退上下文。
func (s *Service) DeleteConversation(ctx context.Context, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if len(contexts) <= 1 {
		return nil, errors.New("Room 至少保留一个对话")
	}
	target, ok := roomdomain.FindConversation(contexts, conversationID)
	if !ok {
		return nil, ErrConversationNotFound
	}
	if target.ConversationType != protocol.ConversationTypeTopic {
		return nil, errors.New("主对话不支持删除")
	}
	targetContext, ok := roomdomain.FindConversationContext(contexts, conversationID)
	if !ok {
		return nil, ErrConversationNotFound
	}
	contextValue, err := s.repository.DeleteConversation(
		ctx,
		authctx.OwnerUserID(ctx),
		strings.TrimSpace(roomID),
		strings.TrimSpace(conversationID),
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	runtimeErr := s.closeConversationRuntimeSessions(ctx, []protocol.ConversationContextAggregate{targetContext}, true, nil)
	artifactErr := s.cleanupConversationArtifacts(ctx, []protocol.ConversationContextAggregate{targetContext}, true, nil)
	goalErr := s.cleanupGoalsForRoomContexts(ctx, []protocol.ConversationContextAggregate{targetContext})
	return contextValue, errors.Join(runtimeErr, artifactErr, goalErr)
}

// UpdateSessionSDKSessionID 更新房间会话记录中的 SDK session_id。
func (s *Service) UpdateSessionSDKSessionID(ctx context.Context, sessionID string, sdkSessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	sdkSessionID = strings.TrimSpace(sdkSessionID)
	if sessionID == "" || sdkSessionID == "" {
		return nil
	}
	return s.repository.UpdateSessionSDKSessionID(ctx, sessionID, sdkSessionID)
}
