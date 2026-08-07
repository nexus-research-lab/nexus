// INPUT: 删除前的 Room 聚合、overlay transcript 引用与 owner 作用域。
// OUTPUT: 已清理 runtime、Goal、任务、transcript 文件和 SQL 主记录的删除结果。
// POS: Room、Conversation 与成员删除的统一收口边界。
package room

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type roomDeletionPayload struct {
	AgentIDs             []string
	AgentID              string
	Contexts             []protocol.ConversationContextAggregate
	ConversationID       string
	RoomID               string
	TranscriptReferences []workspacestore.RoomTranscriptReference
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

func (s *Service) applyRoomDeletion(
	ctx context.Context,
	payload roomDeletionPayload,
) error {
	if err := s.closeConversationRuntimeSessions(ctx, payload.Contexts, true, nil); err != nil {
		return err
	}
	if err := s.cleanupRoomDeletionReferences(ctx, payload, true); err != nil {
		return err
	}
	if err := s.cleanupGoalsForRoomContexts(ctx, payload.Contexts); err != nil {
		return err
	}
	if err := s.cleanupConversationArtifacts(
		ctx,
		payload.Contexts,
		payload.TranscriptReferences,
		true,
		nil,
	); err != nil {
		return err
	}
	deleted, err := s.repository.DeleteRoom(ctx, authctx.OwnerUserID(ctx), payload.RoomID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrRoomNotFound
	}
	return nil
}

func (s *Service) applyConversationDeletion(
	ctx context.Context,
	payload roomDeletionPayload,
) (*protocol.ConversationContextAggregate, error) {
	if err := s.closeConversationRuntimeSessions(ctx, payload.Contexts, true, nil); err != nil {
		return nil, err
	}
	if err := s.cleanupRoomDeletionReferences(ctx, payload, true); err != nil {
		return nil, err
	}
	if err := s.cleanupGoalsForRoomContexts(ctx, payload.Contexts); err != nil {
		return nil, err
	}
	if err := s.cleanupConversationArtifacts(
		ctx,
		payload.Contexts,
		payload.TranscriptReferences,
		true,
		nil,
	); err != nil {
		return nil, err
	}
	fallback, err := s.repository.DeleteConversation(
		ctx,
		authctx.OwnerUserID(ctx),
		payload.RoomID,
		payload.ConversationID,
	)
	if err != nil {
		return nil, err
	}
	if fallback == nil {
		return nil, ErrConversationNotFound
	}
	return fallback, nil
}

func (s *Service) applyRoomMemberDeletion(
	ctx context.Context,
	payload roomDeletionPayload,
) (*protocol.ConversationContextAggregate, error) {
	filter := roomAgentFilter(payload.AgentIDs)
	if err := s.closeConversationRuntimeSessions(ctx, payload.Contexts, false, filter); err != nil {
		return nil, err
	}
	if err := s.cleanupRoomDeletionReferences(ctx, payload, false); err != nil {
		return nil, err
	}
	if err := s.cleanupGoalsForRoomMemberContexts(ctx, payload.Contexts, payload.AgentID); err != nil {
		return nil, err
	}
	if err := s.cleanupConversationArtifacts(
		ctx,
		payload.Contexts,
		payload.TranscriptReferences,
		false,
		filter,
	); err != nil {
		return nil, err
	}
	contextValue, err := s.repository.RemoveRoomMember(
		ctx,
		authctx.OwnerUserID(ctx),
		payload.RoomID,
		payload.AgentID,
	)
	if err != nil {
		return nil, err
	}
	if contextValue == nil {
		return nil, ErrRoomNotFound
	}
	return contextValue, nil
}
