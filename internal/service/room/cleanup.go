// INPUT: 已提交删除的 Room/Conversation 上下文、成员过滤与 Session artifact 删除协调器。
// OUTPUT: Room 公共 ledger 清理，以及逐成员安装 tombstone 后的 runtime/transcript/artifact 回收。
// POS: Room 删除提交后的外围清理阶段；禁止直接删除 Agent Session 目录。
package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) cleanupConversationArtifacts(
	ctx context.Context,
	contexts []protocol.ConversationContextAggregate,
	deleteSharedLog bool,
	agentFilter map[string]struct{},
) error {
	if s.sessionArtifacts == nil &&
		hasConversationSessionArtifacts(contexts, agentFilter) {
		return ErrSessionArtifactDeletionCoordinatorUnavailable
	}

	errs := make([]error, 0)
	workspaceByOwnerAgent := make(map[string]string)
	cleanupCtx := context.WithoutCancel(ctx)
	for _, contextValue := range contexts {
		ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
		ownerFiles := s.files.ForOwner(ownerUserID)
		if deleteSharedLog {
			if _, err := ownerFiles.DeleteRoomConversation(
				ownerUserID,
				contextValue.Conversation.ID,
			); err != nil {
				errs = append(errs, err)
			}
		}

		seenSessionKeys := make(map[string]struct{})
		for _, sessionValue := range contextValue.Sessions {
			if len(agentFilter) > 0 {
				if _, ok := agentFilter[sessionValue.AgentID]; !ok {
					continue
				}
			}

			sessionKey := protocol.BuildRoomAgentSessionKey(
				contextValue.Conversation.ID,
				sessionValue.AgentID,
				contextValue.Room.RoomType,
			)
			if _, exists := seenSessionKeys[sessionKey]; exists {
				continue
			}
			seenSessionKeys[sessionKey] = struct{}{}

			workspaceKey := ownerUserID + "\x00" + strings.TrimSpace(sessionValue.AgentID)
			workspacePath := workspaceByOwnerAgent[workspaceKey]
			if workspacePath == "" {
				resolvedPath, err := s.resolveAgentWorkspacePath(
					cleanupCtx,
					ownerUserID,
					sessionValue.AgentID,
				)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				workspacePath = resolvedPath
				workspaceByOwnerAgent[workspaceKey] = workspacePath
			}

			if err := s.sessionArtifacts.DeleteSessionArtifacts(
				cleanupCtx,
				ownerUserID,
				workspacePath,
				sessionKey,
				strings.TrimSpace(sessionValue.SDKSessionID),
			); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func hasConversationSessionArtifacts(
	contexts []protocol.ConversationContextAggregate,
	agentFilter map[string]struct{},
) bool {
	for _, contextValue := range contexts {
		for _, sessionValue := range contextValue.Sessions {
			if len(agentFilter) > 0 {
				if _, ok := agentFilter[sessionValue.AgentID]; !ok {
					continue
				}
			}
			return true
		}
	}
	return false
}

func (s *Service) cleanupGoalsForRoomContexts(ctx context.Context, contexts []protocol.ConversationContextAggregate) error {
	if s == nil || s.goals == nil {
		return nil
	}
	conversationIDs := roomContextConversationIDs(contexts)
	if len(conversationIDs) == 0 {
		return nil
	}
	_, err := s.goals.DeleteGoalsForRoomConversations(ctx, conversationIDs)
	return err
}

func (s *Service) cleanupGoalsForRoomMemberContexts(ctx context.Context, contexts []protocol.ConversationContextAggregate, agentID string) error {
	if s == nil || s.goals == nil {
		return nil
	}
	conversationIDs := roomContextConversationIDs(contexts)
	if len(conversationIDs) == 0 {
		return nil
	}
	_, err := s.goals.DeleteGoalsForRoomMember(ctx, agentID, conversationIDs)
	return err
}

func roomContextConversationIDs(contexts []protocol.ConversationContextAggregate) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(contexts))
	for _, contextValue := range contexts {
		conversationID := strings.TrimSpace(contextValue.Conversation.ID)
		if conversationID == "" {
			continue
		}
		if _, ok := seen[conversationID]; ok {
			continue
		}
		seen[conversationID] = struct{}{}
		result = append(result, conversationID)
	}
	return result
}
