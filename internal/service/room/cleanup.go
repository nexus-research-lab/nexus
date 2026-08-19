// INPUT: 已提交删除的 Room/Conversation 上下文、overlay transcript 引用与成员过滤。
// OUTPUT: 带持久 tombstone 的成员 Session 删除、完整 transcript lineage 与公共 ledger 清理。
// POS: Room 删除提交后的外围清理阶段；禁止直接删除 Agent Session 目录。
package room

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type sessionArtifactLineageDeletionCoordinator interface {
	DeleteSessionArtifactsWithTranscripts(
		context.Context,
		string,
		string,
		string,
		[]string,
	) error
}

func (s *Service) cleanupConversationArtifacts(
	ctx context.Context,
	contexts []protocol.ConversationContextAggregate,
	transcriptReferences []workspacestore.RoomTranscriptReference,
	protectedTranscriptSessionIDs []string,
	deleteSharedLog bool,
	agentFilter map[string]struct{},
) error {
	errs := make([]error, 0)
	workspaceByOwnerAgent := make(map[string]string)
	protectedTranscriptIDs := make(map[string]struct{}, len(protectedTranscriptSessionIDs))
	for _, sessionID := range protocol.MergeTranscriptSessionIDs(protectedTranscriptSessionIDs) {
		protectedTranscriptIDs[sessionID] = struct{}{}
	}
	cleanupCtx := context.WithoutCancel(ctx)
	for _, contextValue := range contexts {
		ownerUserID := strings.TrimSpace(contextValue.Room.OwnerUserID)
		ownerFiles := s.files.ForOwner(ownerUserID)
		ownerHistory := s.history.ForOwner(ownerUserID)
		artifacts := make(map[string]*roomSessionArtifacts)
		contextErrorCount := len(errs)

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
			artifact := ensureRoomSessionArtifacts(artifacts, workspacePath, sessionKey)
			artifact.transcriptSessionIDs = protocol.MergeTranscriptSessionIDs(
				artifact.transcriptSessionIDs,
				protocol.RoomSessionCleanupTranscriptIDs(sessionValue),
			)
		}
		for _, reference := range transcriptReferences {
			if strings.TrimSpace(reference.ConversationID) != strings.TrimSpace(contextValue.Conversation.ID) {
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
			artifact.transcriptSessionIDs = protocol.MergeTranscriptSessionIDs(
				artifact.transcriptSessionIDs,
				[]string{reference.SessionID},
			)
		}
		for _, artifact := range artifacts {
			artifact.transcriptSessionIDs = unprotectedTranscriptSessionIDs(
				artifact.transcriptSessionIDs,
				protectedTranscriptIDs,
			)
		}

		if len(artifacts) > 0 && s.sessionArtifacts == nil {
			for _, artifact := range artifacts {
				item, _, findErr := ownerFiles.FindSession(
					[]string{artifact.workspacePath},
					artifact.sessionKey,
				)
				if findErr != nil {
					errs = append(errs, findErr)
					continue
				}
				if item != nil {
					errs = append(errs, ErrSessionArtifactDeletionCoordinatorUnavailable)
					continue
				}
				for _, transcriptSessionID := range artifact.transcriptSessionIDs {
					if _, err := ownerHistory.DeleteTranscriptSession(
						artifact.workspacePath,
						transcriptSessionID,
					); err != nil {
						errs = append(errs, err)
					}
				}
			}
		} else {
			for _, artifact := range artifacts {
				if artifact.workspacePath == "" || artifact.sessionKey == "" {
					continue
				}
				if lineageCoordinator, ok := s.sessionArtifacts.(sessionArtifactLineageDeletionCoordinator); ok {
					if err := lineageCoordinator.DeleteSessionArtifactsWithTranscripts(
						cleanupCtx,
						ownerUserID,
						artifact.workspacePath,
						artifact.sessionKey,
						artifact.transcriptSessionIDs,
					); err != nil {
						errs = append(errs, err)
					}
					continue
				}

				cleanupSessionID := ""
				if len(artifact.transcriptSessionIDs) > 0 {
					cleanupSessionID = artifact.transcriptSessionIDs[0]
				}
				if err := s.sessionArtifacts.DeleteSessionArtifacts(
					cleanupCtx,
					ownerUserID,
					artifact.workspacePath,
					artifact.sessionKey,
					cleanupSessionID,
				); err != nil {
					errs = append(errs, err)
					continue
				}
				if len(artifact.transcriptSessionIDs) > 1 {
					for _, transcriptSessionID := range artifact.transcriptSessionIDs[1:] {
						if _, err := ownerHistory.DeleteTranscriptSession(
							artifact.workspacePath,
							transcriptSessionID,
						); err != nil {
							errs = append(errs, err)
						}
					}
				}
			}
		}
		if deleteSharedLog && len(errs) == contextErrorCount {
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

func unprotectedTranscriptSessionIDs(
	values []string,
	protected map[string]struct{},
) []string {
	result := make([]string, 0, len(values))
	for _, sessionID := range protocol.MergeTranscriptSessionIDs(values) {
		if _, exists := protected[sessionID]; exists {
			continue
		}
		result = append(result, sessionID)
	}
	return result
}

type roomSessionArtifacts struct {
	sessionKey           string
	transcriptSessionIDs []string
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
			sessionKey:    sessionKey,
			workspacePath: workspacePath,
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
