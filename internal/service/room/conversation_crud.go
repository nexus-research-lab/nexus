package room

import (
	"context"
	"errors"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// CreateConversation 确保 Room 只有一个尚无用户输入的 draft；标题不改变草稿语义。
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

	title := roomdomain.NormalizeOptionalText(request.Title)

	conversationID := roomdomain.NewEntityID()
	contextValue, err := s.repository.CreateConversation(ctx, roomrepo.CreateConversationBundle{
		RoomID: roomValue.ID,
		Conversation: protocol.ConversationRecord{
			ID:               conversationID,
			RoomID:           roomValue.ID,
			ConversationType: protocol.ConversationTypeTopic,
			Title:            title,
			IsDraft:          true,
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

// DeleteConversation 删除 room 对话并返回回退上下文。
func (s *Service) DeleteConversation(ctx context.Context, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	roomID = strings.TrimSpace(roomID)
	conversationID = strings.TrimSpace(conversationID)
	if job, payload, err := s.loadRoomDeletionJob(
		ctx,
		deletionsvc.KindConversation,
		conversationID,
	); err != nil {
		return nil, err
	} else if job != nil {
		return s.applyConversationDeletion(ctx, *job, payload)
	}
	contexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if len(contexts) <= 1 {
		return nil, errors.New("Room 至少保留一个对话")
	}
	targetContext, ok := roomdomain.FindConversationContext(contexts, conversationID)
	if !ok {
		return nil, ErrConversationNotFound
	}
	targetContexts := []protocol.ConversationContextAggregate{targetContext}
	transcriptReferences, err := s.captureRoomTranscriptReferences(targetContexts)
	if err != nil {
		return nil, err
	}
	payload := roomDeletionPayload{
		Contexts:             targetContexts,
		ConversationID:       conversationID,
		FallbackConversation: fallbackConversationID(contexts, conversationID),
		RoomID:               roomID,
		TranscriptReferences: transcriptReferences,
	}
	job, err := s.ensureRoomDeletionJob(
		ctx,
		deletionsvc.KindConversation,
		conversationID,
		payload,
	)
	if err != nil {
		return nil, err
	}
	return s.applyConversationDeletion(ctx, job, payload)
}

// UpdateSessionSDKSessionID 更新房间会话记录中的 SDK session_id。
func (s *Service) UpdateSessionSDKSessionID(ctx context.Context, sessionID string, sdkSessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	sdkSessionID = strings.TrimSpace(sdkSessionID)
	if sessionID == "" {
		return nil
	}
	return s.repository.UpdateSessionSDKSessionID(ctx, sessionID, sdkSessionID)
}

// TouchConversationActivity 更新 conversation 级最近活动时间。
func (s *Service) TouchConversationActivity(ctx context.Context, conversationID string, activityAt time.Time) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	if activityAt.IsZero() {
		activityAt = time.Now().UTC()
	}
	return s.repository.TouchConversationActivity(ctx, conversationID, activityAt.UTC())
}

// MarkConversationStarted 在首条真实用户输入落盘后消费 conversation draft。
func (s *Service) MarkConversationStarted(ctx context.Context, conversationID string, activityAt time.Time) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	if activityAt.IsZero() {
		activityAt = time.Now().UTC()
	}
	return s.repository.MarkConversationStarted(ctx, conversationID, activityAt.UTC())
}
