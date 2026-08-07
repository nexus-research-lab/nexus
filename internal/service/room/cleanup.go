package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) cleanupConversationArtifacts(
	ctx context.Context,
	contexts []protocol.ConversationContextAggregate,
	transcriptReferences []workspacestore.RoomTranscriptReference,
	deleteSharedLog bool,
	agentFilter map[string]struct{},
) error {
	errs := make([]error, 0)
	workspaceByOwnerAgent := make(map[string]string)
	for _, contextValue := range contexts {
		ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
		ownerFiles := s.files.ForOwner(ownerUserID)
		ownerHistory := s.history.ForOwner(ownerUserID)
		artifacts := make(map[string]*roomSessionArtifacts)
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
			artifact := ensureRoomSessionArtifacts(artifacts, workspacePath, sessionKey)
			for _, transcriptSessionID := range protocol.RoomSessionTranscriptIDs(sessionValue) {
				artifact.transcriptSessionIDs[transcriptSessionID] = struct{}{}
			}
		}
		for _, reference := range transcriptReferences {
			if reference.ConversationID != contextValue.Conversation.ID {
				continue
			}
			if len(agentFilter) > 0 {
				if _, ok := agentFilter[reference.AgentID]; !ok {
					continue
				}
			}
			artifact := ensureRoomSessionArtifacts(
				artifacts,
				reference.WorkspacePath,
				reference.PrivateSessionKey,
			)
			artifact.transcriptSessionIDs[reference.SessionID] = struct{}{}
		}
		for _, artifact := range artifacts {
			transcriptFailed := false
			for transcriptSessionID := range artifact.transcriptSessionIDs {
				if _, err := ownerHistory.DeleteTranscriptSession(
					artifact.workspacePath,
					transcriptSessionID,
				); err != nil {
					errs = append(errs, err)
					transcriptFailed = true
				}
			}
			if transcriptFailed {
				continue
			}
			if _, err := ownerFiles.DeleteSession(artifact.workspacePath, artifact.sessionKey); err != nil {
				errs = append(errs, err)
			}
		}
		if deleteSharedLog && len(errs) == 0 {
			if _, err := ownerFiles.DeleteRoomConversation(
				ownerUserID,
				contextValue.Conversation.ID,
			); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

type roomSessionArtifacts struct {
	sessionKey           string
	transcriptSessionIDs map[string]struct{}
	workspacePath        string
}

func ensureRoomSessionArtifacts(
	items map[string]*roomSessionArtifacts,
	workspacePath string,
	sessionKey string,
) *roomSessionArtifacts {
	workspacePath = strings.TrimSpace(workspacePath)
	sessionKey = strings.TrimSpace(sessionKey)
	key := workspacePath + "\x00" + sessionKey
	if items[key] == nil {
		items[key] = &roomSessionArtifacts{
			sessionKey:           sessionKey,
			transcriptSessionIDs: make(map[string]struct{}),
			workspacePath:        workspacePath,
		}
	}
	return items[key]
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
