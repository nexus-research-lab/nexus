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
	return s.projectExternalSessionIdentities(ctx, mergeSessions(fileSessions, roomSessions))
}

// ListMutableSessions 只列出 owner workspace 中由 Session 域管理的 Agent 会话。
func (s *Service) ListMutableSessions(ctx context.Context) ([]protocol.Session, error) {
	items, err := s.listMutableWorkspaceSessions(ctx)
	if err != nil {
		return nil, err
	}
	return s.projectExternalSessionIdentities(ctx, items)
}

// ListAgentSessions 列出指定 Agent 的全部会话。
func (s *Service) ListAgentSessions(ctx context.Context, agentID string) ([]protocol.Session, error) {
	agentValue, err := s.agentService.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	fileSessions, err := s.ownerFiles(ctx).ListSessions(agentValue.WorkspacePath)
	if err != nil {
		return nil, err
	}
	filteredFileSessions := make([]protocol.Session, 0, len(fileSessions))
	for _, item := range fileSessions {
		if item.AgentID != agentID {
			continue
		}
		reconciled, reconcileErr := s.reconcileWorkspaceSessionRuntimeState(
			ctx,
			agentValue.WorkspacePath,
			item,
		)
		if reconcileErr != nil {
			return nil, reconcileErr
		}
		if protocol.IsRoomSharedSessionKey(reconciled.SessionKey) {
			continue
		}
		filteredFileSessions = append(filteredFileSessions, reconciled)
	}

	roomSessions, err := s.repository.ListRoomSessionsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	roomSessions = s.applyRuntimeStateToSessions(roomSessions)
	return s.projectExternalSessionIdentities(
		ctx,
		mergeSessions(filteredFileSessions, roomSessions),
	)
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
		workspacePaths, resolveErr := s.resolveWorkspacePaths(ctx, parsed.AgentID)
		if resolveErr != nil {
			return nil, resolveErr
		}
		workspacePath := resolveHistoryWorkspacePath(workspacePaths, parsed)
		hydrated, hydrateErr := s.hydrateRoomHistorySession(
			ctx,
			workspacePath,
			sessionKey,
			*roomSession,
		)
		if hydrateErr != nil {
			return nil, hydrateErr
		}
		normalized := s.applyRuntimeStateToSession(*hydrated)
		projected, projectErr := s.projectExternalSessionIdentity(ctx, normalized)
		if projectErr != nil {
			return nil, projectErr
		}
		return &projected, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	item, workspacePath, err := s.ownerFiles(ctx).FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	normalized, err := s.reconcileWorkspaceSessionRuntimeState(
		ctx,
		workspacePath,
		*item,
	)
	if err != nil {
		return nil, err
	}
	projected, err := s.projectExternalSessionIdentity(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return &projected, nil
}

// GetMutableSession 读取 Session 域可变的 Agent workspace 会话。
func (s *Service) GetMutableSession(
	ctx context.Context,
	rawSessionKey string,
) (*protocol.Session, error) {
	item, _, _, err := s.loadMutableWorkspaceSession(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	normalized := s.applyRuntimeStateToSession(*item)
	projected, err := s.projectExternalSessionIdentity(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return &projected, nil
}
