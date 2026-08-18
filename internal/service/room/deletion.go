// INPUT: owner-scoped Room 删除目标、可选 configuration_version 与删除前固化的 artifact 引用。
// OUTPUT: 数据库优先撤销身份，并对 runtime、Goal、Task、transcript 和 Session 产物做可审计清理。
// POS: Room、Conversation 与成员删除的提交后 reconcile 边界。
package room

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// DeletionReconcileError 表示 Room 数据库删除已经提交，但外围清理未完全成功。
type DeletionReconcileError struct {
	cause error
}

func (e *DeletionReconcileError) Error() string {
	return fmt.Sprintf("Room 数据已删除，但关联运行态清理需要 reconcile: %v", e.cause)
}

func (e *DeletionReconcileError) Unwrap() error {
	return e.cause
}

// RoomDeletionCommitted 判断删除错误是否发生在数据库提交之后。
func RoomDeletionCommitted(err error) bool {
	var committed *DeletionReconcileError
	return errors.As(err, &committed)
}

// RoomMemberDeletionReconcileError 表示成员身份已撤销但外围清理未完成。
type RoomMemberDeletionReconcileError struct {
	cause error
}

func (e *RoomMemberDeletionReconcileError) Error() string {
	return fmt.Sprintf("Room 成员已移除，但关联运行态清理需要 reconcile: %v", e.cause)
}

func (e *RoomMemberDeletionReconcileError) Unwrap() error {
	return e.cause
}

// RoomMemberDeletionCommitted 判断成员删除错误是否发生在数据库提交之后。
func RoomMemberDeletionCommitted(err error) bool {
	var committed *RoomMemberDeletionReconcileError
	return errors.As(err, &committed)
}

type roomDeletionPayload struct {
	AgentIDs                      []string
	AgentID                       string
	Contexts                      []protocol.ConversationContextAggregate
	ConversationID                string
	RoomID                        string
	TranscriptReferences          []workspacestore.RoomTranscriptReference
	ProtectedTranscriptSessionIDs []string
}

func (s *Service) captureRoomTranscriptReferences(
	contexts []protocol.ConversationContextAggregate,
) ([]workspacestore.RoomTranscriptReference, error) {
	seen := make(map[string]struct{})
	result := make([]workspacestore.RoomTranscriptReference, 0)
	for _, contextValue := range contexts {
		ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
		conversationID := strings.TrimSpace(contextValue.Conversation.ID)
		key := ownerUserID + "\x00" + conversationID
		if ownerUserID == "" || conversationID == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items, err := s.roomHistory.ListTranscriptReferences(ownerUserID, conversationID)
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
	}
	return result, nil
}

func (s *Service) cleanupRoomDeletionReferences(
	ctx context.Context,
	payload roomDeletionPayload,
	includeShared bool,
) error {
	if s.deletion == nil {
		return nil
	}
	return s.deletion.CleanupSessionReferences(
		ctx,
		authctx.OwnerUserID(ctx),
		roomRuntimeSessionKeys(payload.Contexts, includeShared, roomAgentFilter(payload.AgentIDs)),
	)
}

func roomAgentFilter(agentIDs []string) map[string]struct{} {
	if len(agentIDs) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(agentIDs))
	for _, agentID := range agentIDs {
		agentID = strings.TrimSpace(agentID)
		if agentID != "" {
			result[agentID] = struct{}{}
		}
	}
	return result
}

func (s *Service) cleanupCommittedRoomDeletion(
	ctx context.Context,
	payload roomDeletionPayload,
	includeShared bool,
) error {
	cleanupCtx := context.WithoutCancel(ctx)
	filter := roomAgentFilter(payload.AgentIDs)
	errValues := []error{
		wrapRoomDeletionCleanup("关闭 runtime session", s.closeConversationRuntimeSessions(
			cleanupCtx,
			payload.Contexts,
			includeShared,
			filter,
		)),
		wrapRoomDeletionCleanup("清理 Session 引用", s.cleanupRoomDeletionReferences(
			cleanupCtx,
			payload,
			includeShared,
		)),
	}
	if payload.AgentID == "" {
		errValues = append(errValues, wrapRoomDeletionCleanup(
			"清理 Goal",
			s.cleanupGoalsForRoomContexts(cleanupCtx, payload.Contexts),
		))
	} else {
		errValues = append(errValues, wrapRoomDeletionCleanup(
			"清理成员 Goal",
			s.cleanupGoalsForRoomMemberContexts(cleanupCtx, payload.Contexts, payload.AgentID),
		))
	}
	errValues = append(errValues, wrapRoomDeletionCleanup(
		"清理 Session 与 transcript artifact",
		s.cleanupConversationArtifacts(
			cleanupCtx,
			payload.Contexts,
			payload.TranscriptReferences,
			payload.ProtectedTranscriptSessionIDs,
			includeShared,
			filter,
		),
	))
	return errors.Join(errValues...)
}

// DeleteRoom 删除房间，并在数据库提交后清理所有 conversation 运行态与文件。
func (s *Service) DeleteRoom(ctx context.Context, roomID string) error {
	return s.deleteRoom(ctx, roomID, nil)
}

// DeleteRoomAtVersion 仅在 configuration_version 仍等于计划版本时删除房间。
func (s *Service) DeleteRoomAtVersion(
	ctx context.Context,
	roomID string,
	expectedConfigurationVersion int64,
) error {
	if expectedConfigurationVersion < 1 {
		return errors.New("expected Room configuration_version 必须大于 0")
	}
	return s.deleteRoom(ctx, roomID, &expectedConfigurationVersion)
}

func (s *Service) deleteRoom(
	ctx context.Context,
	roomID string,
	expectedConfigurationVersion *int64,
) error {
	roomID = strings.TrimSpace(roomID)
	roomContexts, err := s.GetRoomContexts(ctx, roomID)
	if err != nil {
		return err
	}
	transcriptReferences, err := s.captureRoomTranscriptReferences(roomContexts)
	if err != nil {
		return err
	}

	ownerUserID := authctx.OwnerUserID(ctx)
	payload := roomDeletionPayload{
		Contexts:             roomContexts,
		RoomID:               roomID,
		TranscriptReferences: transcriptReferences,
	}
	if expectedConfigurationVersion == nil {
		if cleanupErr := s.cleanupCommittedRoomDeletion(ctx, payload, true); cleanupErr != nil {
			return cleanupErr
		}
	}
	var deleted bool
	if expectedConfigurationVersion != nil {
		deleted, err = s.repository.DeleteRoomAtVersion(
			ctx,
			ownerUserID,
			roomID,
			*expectedConfigurationVersion,
		)
	} else {
		deleted, err = s.repository.DeleteRoom(ctx, ownerUserID, roomID)
	}
	if err != nil {
		return err
	}
	if !deleted {
		return ErrRoomNotFound
	}
	if expectedConfigurationVersion != nil {
		if cleanupErr := s.cleanupCommittedRoomDeletion(ctx, payload, true); cleanupErr != nil {
			return &DeletionReconcileError{cause: cleanupErr}
		}
	}
	return nil
}

func wrapRoomDeletionCleanup(stage string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", stage, err)
}
