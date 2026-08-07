// INPUT: 删除前的 Room 聚合、overlay transcript 引用与 owner 作用域。
// OUTPUT: 可跨失败重放的 Room、Conversation、成员完整删除流程。
// POS: Room SQL 主记录与 runtime、Goal、任务、transcript 文件之间的持久事务边界。
package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	deletionsvc "github.com/nexus-research-lab/nexus/internal/service/deletion"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type roomDeletionPayload struct {
	AgentIDs             []string                                 `json:"agent_ids,omitempty"`
	AgentID              string                                   `json:"agent_id,omitempty"`
	Contexts             []protocol.ConversationContextAggregate  `json:"contexts"`
	ConversationID       string                                   `json:"conversation_id,omitempty"`
	FallbackConversation string                                   `json:"fallback_conversation_id,omitempty"`
	RoomID               string                                   `json:"room_id"`
	TranscriptReferences []workspacestore.RoomTranscriptReference `json:"transcript_references,omitempty"`
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

func (s *Service) ensureRoomDeletionJob(
	ctx context.Context,
	kind deletionsvc.Kind,
	targetID string,
	payload roomDeletionPayload,
) (deletionsvc.Job, error) {
	if s.deletion == nil {
		return deletionsvc.Job{}, nil
	}
	return s.deletion.Ensure(
		ctx,
		authctx.OwnerUserID(ctx),
		kind,
		targetID,
		payload,
	)
}

func (s *Service) loadRoomDeletionJob(
	ctx context.Context,
	kind deletionsvc.Kind,
	targetID string,
) (*deletionsvc.Job, roomDeletionPayload, error) {
	if s.deletion == nil {
		return nil, roomDeletionPayload{}, nil
	}
	job, err := s.deletion.Load(ctx, authctx.OwnerUserID(ctx), kind, targetID)
	if err != nil || job == nil {
		return job, roomDeletionPayload{}, err
	}
	var payload roomDeletionPayload
	if err = deletionsvc.DecodePayload(*job, &payload); err != nil {
		return job, payload, s.deletion.Fail(ctx, *job, err)
	}
	bindRoomDeletionOwner(&payload, job.OwnerUserID)
	return job, payload, nil
}

func bindRoomDeletionOwner(payload *roomDeletionPayload, ownerUserID string) {
	if payload == nil {
		return
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	for index := range payload.Contexts {
		payload.Contexts[index].Room.OwnerUserID = ownerUserID
	}
}

func (s *Service) failRoomDeletion(
	ctx context.Context,
	job deletionsvc.Job,
	err error,
) error {
	if s.deletion == nil || job.ID == "" {
		return err
	}
	return s.deletion.Fail(ctx, job, err)
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

func (s *Service) finishRoomDeletion(
	ctx context.Context,
	job deletionsvc.Job,
) error {
	if s.deletion == nil || job.ID == "" {
		return nil
	}
	if err := s.deletion.Complete(ctx, job); err != nil {
		return s.deletion.Fail(ctx, job, err)
	}
	return nil
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

func fallbackConversationID(
	contexts []protocol.ConversationContextAggregate,
	targetID string,
) string {
	fallbackID := ""
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == targetID {
			continue
		}
		if fallbackID == "" {
			fallbackID = contextValue.Conversation.ID
		}
		if contextValue.Conversation.ConversationType == protocol.ConversationTypeMain ||
			contextValue.Conversation.ConversationType == protocol.ConversationTypeDM {
			return contextValue.Conversation.ID
		}
	}
	return fallbackID
}

func (s *Service) applyRoomDeletion(
	ctx context.Context,
	job deletionsvc.Job,
	payload roomDeletionPayload,
) error {
	if err := s.closeConversationRuntimeSessions(ctx, payload.Contexts, true, nil); err != nil {
		return s.failRoomDeletion(ctx, job, err)
	}
	if err := s.cleanupRoomDeletionReferences(ctx, payload, true); err != nil {
		return s.failRoomDeletion(ctx, job, err)
	}
	if err := s.cleanupGoalsForRoomContexts(ctx, payload.Contexts); err != nil {
		return s.failRoomDeletion(ctx, job, err)
	}
	deleted, err := s.repository.DeleteRoom(ctx, authctx.OwnerUserID(ctx), payload.RoomID)
	if err != nil {
		return s.failRoomDeletion(ctx, job, err)
	}
	if !deleted && job.ID == "" {
		return ErrRoomNotFound
	}
	if err = s.cleanupConversationArtifacts(
		ctx,
		payload.Contexts,
		payload.TranscriptReferences,
		true,
		nil,
	); err != nil {
		return s.failRoomDeletion(ctx, job, err)
	}
	return s.finishRoomDeletion(ctx, job)
}

func (s *Service) applyConversationDeletion(
	ctx context.Context,
	job deletionsvc.Job,
	payload roomDeletionPayload,
) (*protocol.ConversationContextAggregate, error) {
	if err := s.closeConversationRuntimeSessions(ctx, payload.Contexts, true, nil); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if err := s.cleanupRoomDeletionReferences(ctx, payload, true); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if err := s.cleanupGoalsForRoomContexts(ctx, payload.Contexts); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	fallback, err := s.repository.DeleteConversation(
		ctx,
		authctx.OwnerUserID(ctx),
		payload.RoomID,
		payload.ConversationID,
	)
	if err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if err = s.cleanupConversationArtifacts(
		ctx,
		payload.Contexts,
		payload.TranscriptReferences,
		true,
		nil,
	); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if fallback == nil && payload.FallbackConversation != "" {
		fallback, err = s.GetConversationContext(ctx, payload.FallbackConversation)
		if err != nil {
			return nil, s.failRoomDeletion(ctx, job, err)
		}
	}
	if fallback == nil {
		return nil, s.failRoomDeletion(ctx, job, ErrConversationNotFound)
	}
	if err = s.finishRoomDeletion(ctx, job); err != nil {
		return nil, err
	}
	return fallback, nil
}

func (s *Service) applyRoomMemberDeletion(
	ctx context.Context,
	job deletionsvc.Job,
	payload roomDeletionPayload,
) (*protocol.ConversationContextAggregate, error) {
	filter := roomAgentFilter(payload.AgentIDs)
	if err := s.closeConversationRuntimeSessions(ctx, payload.Contexts, false, filter); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if err := s.cleanupRoomDeletionReferences(ctx, payload, false); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if err := s.cleanupGoalsForRoomMemberContexts(ctx, payload.Contexts, payload.AgentID); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	contextValue, err := s.repository.RemoveRoomMember(
		ctx,
		authctx.OwnerUserID(ctx),
		payload.RoomID,
		payload.AgentID,
	)
	if err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if err = s.cleanupConversationArtifacts(
		ctx,
		payload.Contexts,
		payload.TranscriptReferences,
		false,
		filter,
	); err != nil {
		return nil, s.failRoomDeletion(ctx, job, err)
	}
	if contextValue == nil {
		roomValue, getErr := s.GetRoom(ctx, payload.RoomID)
		if getErr != nil {
			return nil, s.failRoomDeletion(ctx, job, getErr)
		}
		if roomValue != nil {
			contexts, contextsErr := s.GetRoomContexts(ctx, payload.RoomID)
			if contextsErr != nil {
				return nil, s.failRoomDeletion(ctx, job, contextsErr)
			}
			if len(contexts) > 0 {
				contextValue = &contexts[0]
			}
		}
	}
	if contextValue == nil {
		return nil, s.failRoomDeletion(ctx, job, ErrRoomNotFound)
	}
	if err = s.finishRoomDeletion(ctx, job); err != nil {
		return nil, err
	}
	return contextValue, nil
}

// ReconcilePendingDeletions 重放 Room 域未完成的持久删除任务。
func (s *Service) ReconcilePendingDeletions(ctx context.Context) error {
	if s == nil || s.deletion == nil {
		return nil
	}
	jobs, err := s.deletion.ListPending(
		ctx,
		deletionsvc.KindRoom,
		deletionsvc.KindConversation,
		deletionsvc.KindRoomMember,
	)
	if err != nil {
		return err
	}
	errList := make([]error, 0)
	for _, job := range jobs {
		var payload roomDeletionPayload
		if decodeErr := deletionsvc.DecodePayload(job, &payload); decodeErr != nil {
			errList = append(errList, s.deletion.Fail(ctx, job, decodeErr))
			continue
		}
		bindRoomDeletionOwner(&payload, job.OwnerUserID)
		ownerCtx := authctx.WithPrincipal(ctx, &authctx.Principal{
			UserID: job.OwnerUserID,
			Role:   authctx.RoleOwner,
		})
		switch job.Kind {
		case deletionsvc.KindRoom:
			err = s.applyRoomDeletion(ownerCtx, job, payload)
		case deletionsvc.KindConversation:
			_, err = s.applyConversationDeletion(ownerCtx, job, payload)
		case deletionsvc.KindRoomMember:
			_, err = s.applyRoomMemberDeletion(ownerCtx, job, payload)
		}
		if err != nil {
			errList = append(errList, err)
		}
	}
	return errors.Join(errList...)
}
