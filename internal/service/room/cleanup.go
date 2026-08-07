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
	errs := make([]error, 0)
	workspaceByOwnerAgent := make(map[string]string)
	for _, contextValue := range contexts {
		ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
		ownerFiles := s.files.ForOwner(ownerUserID)
		ownerHistory := s.history.ForOwner(ownerUserID)
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
					ctx,
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

			if ownerHistory != nil && strings.TrimSpace(sessionValue.SDKSessionID) != "" {
				if _, err := ownerHistory.DeleteTranscriptSession(workspacePath, sessionValue.SDKSessionID); err != nil {
					errs = append(errs, err)
					// 保留带 sdk_session_id 的 session meta，后续修复仍能精确重试。
					continue
				}
			}
			if _, err := ownerFiles.DeleteSession(workspacePath, sessionKey); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
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
