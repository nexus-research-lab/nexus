package session

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CreateSession 创建或幂等返回普通 Agent 会话。
func (s *Service) CreateSession(ctx context.Context, request CreateRequest) (*protocol.Session, error) {
	sessionKey, parsed, err := s.requireSessionKey(request.SessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, fmt.Errorf("%w: 共享 room session 不支持通过 Session API 创建", ErrSessionMutationUnsupported)
	}
	if request.AgentID != "" && request.AgentID != parsed.AgentID {
		return nil, errors.New("agent_id 与 session_key 不一致")
	}

	existing, err := s.GetSession(ctx, sessionKey)
	if err == nil && existing != nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}

	agentValue, err := s.agentService.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	created, err := s.files.UpsertSession(agentValue.WorkspacePath, normalizeSession(protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      parsed.AgentID,
		ChannelType:  protocol.NormalizeStoredChannelType(parsed.Channel),
		ChatType:     protocol.NormalizeSessionChatType(parsed.ChatType),
		Status:       "closed",
		CreatedAt:    now,
		LastActivity: now,
		Title:        cmp.Or(strings.TrimSpace(request.Title), "New Chat"),
		MessageCount: 0,
		Options:      map[string]any{},
		IsActive:     false,
	}))
	if err != nil {
		return nil, err
	}
	s.notifyDirectoryChanged(ctx, "session_created", *created)
	return created, nil
}

// UpdateSession 更新普通 Agent 会话标题。
func (s *Service) UpdateSession(ctx context.Context, rawSessionKey string, request UpdateRequest) (*protocol.Session, error) {
	item, workspacePath, parsed, err := s.loadMutableWorkspaceSession(ctx, rawSessionKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrSessionNotFound
	}
	next := closePersistedSessionMeta(normalizeSession(*item))
	if request.Title != nil {
		next.Title = cmp.Or(strings.TrimSpace(*request.Title), "New Chat")
	}
	if parsed.AgentID != "" {
		next.AgentID = parsed.AgentID
	}
	updated, err := s.files.UpsertSession(workspacePath, next)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		projected := s.applyRuntimeStateToSession(next)
		s.notifyDirectoryChanged(ctx, "session_updated", projected)
		return &projected, nil
	}
	projected := s.applyRuntimeStateToSession(*updated)
	s.notifyDirectoryChanged(ctx, "session_updated", projected)
	return &projected, nil
}

// UpdateSessionTitle 以最小输入更新会话标题，供跨领域服务复用。
func (s *Service) UpdateSessionTitle(ctx context.Context, rawSessionKey string, title string) (*protocol.Session, error) {
	return s.UpdateSession(ctx, rawSessionKey, UpdateRequest{Title: &title})
}

// DeleteSession 删除普通 Agent 会话目录。
func (s *Service) DeleteSession(ctx context.Context, rawSessionKey string) error {
	sessionKey, _, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return err
	}
	item, workspacePath, _, err := s.loadMutableWorkspaceSession(ctx, sessionKey)
	if err != nil {
		return err
	}
	if workspacePath == "" {
		return ErrSessionNotFound
	}
	deleted, err := s.files.DeleteSession(workspacePath, sessionKey)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrSessionNotFound
	}
	if item != nil && item.SessionID != nil {
		if _, err := s.history.DeleteTranscriptSession(workspacePath, strings.TrimSpace(*item.SessionID)); err != nil {
			return err
		}
	}
	if item != nil {
		s.notifyDirectoryChanged(ctx, "session_deleted", *item)
	}
	return nil
}

func (s *Service) notifyDirectoryChanged(ctx context.Context, reason string, session protocol.Session) {
	if s.notifier == nil {
		return
	}
	s.notifier.NotifyDirectoryChanged(ctx, strings.TrimSpace(reason), session)
}

func (s *Service) loadMutableWorkspaceSession(ctx context.Context, rawSessionKey string) (*protocol.Session, string, protocol.SessionKey, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, "", protocol.SessionKey{}, err
	}
	if parsed.Kind != protocol.SessionKeyKindAgent {
		return nil, "", parsed, fmt.Errorf("%w: 共享 room session 不支持通过 Session API 修改", ErrSessionMutationUnsupported)
	}

	roomSession, err := s.repository.GetRoomSessionByKey(ctx, authctx.OwnerUserID(ctx), parsed)
	if err != nil {
		return nil, "", parsed, err
	}
	if roomSession != nil {
		return nil, "", parsed, fmt.Errorf("%w: Room 成员会话必须通过 room/conversation 语义修改", ErrSessionMutationUnsupported)
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, "", parsed, err
	}
	item, workspacePath, err := s.files.FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, "", parsed, err
	}
	return item, workspacePath, parsed, nil
}
