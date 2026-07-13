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
	workspaceByAgentID := make(map[string]string)
	for _, contextValue := range contexts {
		if deleteSharedLog {
			if _, err := s.files.DeleteRoomConversation(contextValue.Conversation.ID); err != nil {
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

			workspacePath := workspaceByAgentID[sessionValue.AgentID]
			if workspacePath == "" {
				resolvedPath, err := s.resolveAgentWorkspacePath(ctx, sessionValue.AgentID)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				workspacePath = resolvedPath
				workspaceByAgentID[sessionValue.AgentID] = workspacePath
			}

			if _, err := s.files.DeleteSession(workspacePath, sessionKey); err != nil {
				errs = append(errs, err)
			}
			if s.history != nil && strings.TrimSpace(sessionValue.SDKSessionID) != "" {
				if _, err := s.history.DeleteTranscriptSession(workspacePath, sessionValue.SDKSessionID); err != nil {
					errs = append(errs, err)
				}
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
