// INPUT: owner-scoped Room/conversation/session 变更请求。
// OUTPUT: 经 Room-first repository 事务持久化的 conversation 上下文和活动状态。
// POS: Room conversation mutation 的业务编排层；清理发生在数据库提交之后。
package room

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	roomdomain "github.com/nexus-research-lab/nexus/internal/chat/room"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

// ConversationDeletionReconcileError 表示 conversation 数据库删除已提交，但外围清理未完成。
type ConversationDeletionReconcileError struct {
	cause error
}

func (e *ConversationDeletionReconcileError) Error() string {
	return fmt.Sprintf("Conversation 数据已删除，但关联运行态清理需要 reconcile: %v", e.cause)
}

func (e *ConversationDeletionReconcileError) Unwrap() error {
	return e.cause
}

// ConversationDeletionCommitted 判断错误是否发生在 conversation 数据库删除提交之后。
func ConversationDeletionCommitted(err error) bool {
	var committed *ConversationDeletionReconcileError
	return errors.As(err, &committed)
}

// CreateConversation 确保 Room 只有一个尚无用户输入的 draft；标题不改变草稿语义。
func (s *Service) CreateConversation(ctx context.Context, roomID string, request protocol.CreateConversationRequest) (*protocol.ConversationContextAggregate, error) {
	return s.createConversation(ctx, roomID, request, nil)
}

// CreateConversationAtVersion 仅在 Room configuration_version 匹配时创建 conversation。
func (s *Service) CreateConversationAtVersion(
	ctx context.Context,
	roomID string,
	request protocol.CreateConversationRequest,
	expectedConfigurationVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	if expectedConfigurationVersion < 1 {
		return nil, errors.New("expected Room configuration_version 必须大于 0")
	}
	return s.createConversation(ctx, roomID, request, &expectedConfigurationVersion)
}

func (s *Service) createConversation(
	ctx context.Context,
	roomID string,
	request protocol.CreateConversationRequest,
	expectedConfigurationVersion *int64,
) (*protocol.ConversationContextAggregate, error) {
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
		OwnerUserID: authctx.OwnerUserID(ctx),
		RoomID:      roomValue.ID,
		Conversation: protocol.ConversationRecord{
			ID:               conversationID,
			RoomID:           roomValue.ID,
			ConversationType: protocol.ConversationTypeTopic,
			Title:            title,
			IsDraft:          true,
		},
		Sessions:                     roomdomain.BuildSessions(conversationID, agentRefs),
		ExpectedConfigurationVersion: expectedConfigurationVersion,
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
	return s.updateConversation(ctx, roomID, conversationID, request, nil)
}

// UpdateConversationAtVersion 仅在 Room configuration_version 匹配时更新标题。
func (s *Service) UpdateConversationAtVersion(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.UpdateConversationRequest,
	expectedConfigurationVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	if expectedConfigurationVersion < 1 {
		return nil, errors.New("expected Room configuration_version 必须大于 0")
	}
	return s.updateConversation(
		ctx,
		roomID,
		conversationID,
		request,
		&expectedConfigurationVersion,
	)
}

func (s *Service) updateConversation(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.UpdateConversationRequest,
	expectedConfigurationVersion *int64,
) (*protocol.ConversationContextAggregate, error) {
	roomID = strings.TrimSpace(roomID)
	conversationID = strings.TrimSpace(conversationID)
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
	var contextValue *protocol.ConversationContextAggregate
	if expectedConfigurationVersion != nil {
		contextValue, err = s.repository.UpdateConversationAtVersion(
			ctx,
			authctx.OwnerUserID(ctx),
			roomID,
			conversationID,
			title,
			*expectedConfigurationVersion,
		)
	} else {
		contextValue, err = s.repository.UpdateConversation(
			ctx,
			authctx.OwnerUserID(ctx),
			roomID,
			conversationID,
			title,
		)
	}
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
	return s.deleteConversation(ctx, roomID, conversationID, nil)
}

// DeleteConversationAtVersion 仅在 Room configuration_version 匹配时删除 conversation。
func (s *Service) DeleteConversationAtVersion(
	ctx context.Context,
	roomID string,
	conversationID string,
	expectedConfigurationVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	if expectedConfigurationVersion < 1 {
		return nil, errors.New("expected Room configuration_version 必须大于 0")
	}
	return s.deleteConversation(
		ctx,
		roomID,
		conversationID,
		&expectedConfigurationVersion,
	)
}

func (s *Service) deleteConversation(
	ctx context.Context,
	roomID string,
	conversationID string,
	expectedConfigurationVersion *int64,
) (*protocol.ConversationContextAggregate, error) {
	roomID = strings.TrimSpace(roomID)
	conversationID = strings.TrimSpace(conversationID)
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
		RoomID:               roomID,
		TranscriptReferences: transcriptReferences,
	}
	if expectedConfigurationVersion == nil {
		if cleanupErr := s.cleanupCommittedRoomDeletion(ctx, payload, true); cleanupErr != nil {
			return nil, cleanupErr
		}
	}
	var contextValue *protocol.ConversationContextAggregate
	if expectedConfigurationVersion != nil {
		contextValue, err = s.repository.DeleteConversationAtVersion(
			ctx,
			authctx.OwnerUserID(ctx),
			roomID,
			conversationID,
			*expectedConfigurationVersion,
		)
	} else {
		contextValue, err = s.repository.DeleteConversation(
			ctx,
			authctx.OwnerUserID(ctx),
			roomID,
			conversationID,
		)
	}
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrConversationNotFound
	}
	if expectedConfigurationVersion != nil {
		if cleanupErr := s.cleanupCommittedRoomDeletion(ctx, payload, true); cleanupErr != nil {
			return contextValue, &ConversationDeletionReconcileError{cause: cleanupErr}
		}
	}
	return contextValue, nil
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
