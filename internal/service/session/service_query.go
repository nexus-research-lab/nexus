package session

import (
	"context"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ListSessions 列出全部会话视图。
func (s *Service) ListSessions(ctx context.Context) ([]protocol.Session, error) {
	fileSessions, err := s.listWorkspaceSessions(ctx, "")
	if err != nil {
		return nil, err
	}
	roomSessions, err := s.repository.ListRoomSessions(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return nil, err
	}
	roomSessions = s.applyRuntimeStateToSessions(roomSessions)
	return mergeSessions(fileSessions, roomSessions), nil
}

// ListAgentSessions 列出指定 Agent 的全部会话。
func (s *Service) ListAgentSessions(ctx context.Context, agentID string) ([]protocol.Session, error) {
	agentValue, err := s.agentService.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	fileSessions, err := s.files.ListSessions(agentValue.WorkspacePath)
	if err != nil {
		return nil, err
	}
	filteredFileSessions := make([]protocol.Session, 0, len(fileSessions))
	for _, item := range fileSessions {
		if item.AgentID != agentID {
			continue
		}
		reconciled, reconcileErr := s.reconcileWorkspaceSessionRuntimeState(agentValue.WorkspacePath, item)
		if reconcileErr != nil {
			return nil, reconcileErr
		}
		if shouldHideWorkspaceSession(reconciled) {
			continue
		}
		filteredFileSessions = append(filteredFileSessions, reconciled)
	}

	roomSessions, err := s.repository.ListRoomSessionsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	roomSessions = s.applyRuntimeStateToSessions(roomSessions)
	return mergeSessions(filteredFileSessions, roomSessions), nil
}

// GetSession 读取指定 session。
func (s *Service) GetSession(ctx context.Context, rawSessionKey string) (*protocol.Session, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return nil, ErrSessionNotFound
	}

	roomSession, err := s.repository.GetRoomSessionByKey(ctx, authctx.OwnerUserID(ctx), parsed)
	if err != nil {
		return nil, err
	}
	if roomSession != nil {
		normalized := s.applyRuntimeStateToSession(*roomSession)
		return &normalized, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	item, workspacePath, err := s.files.FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	normalized, err := s.reconcileWorkspaceSessionRuntimeState(workspacePath, *item)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}
