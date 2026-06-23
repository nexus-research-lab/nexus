package session

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// GetSessionMessages 读取 session 历史消息。
func (s *Service) GetSessionMessages(ctx context.Context, rawSessionKey string) ([]protocol.Message, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		return s.roomHistory.ReadMessages(parsed.ConversationID, s.activeRoundIDs(sessionKey))
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	sessionValue, workspacePath, err := s.loadHistorySession(ctx, workspacePaths, parsed, sessionKey)
	if err != nil {
		return nil, err
	}
	if sessionValue == nil {
		return nil, ErrSessionNotFound
	}
	return s.history.ReadMessages(workspacePath, *sessionValue, s.activeRoundIDs(sessionKey))
}

// GetSessionMessagesPage 分页读取 session 历史消息。
func (s *Service) GetSessionMessagesPage(
	ctx context.Context,
	rawSessionKey string,
	request MessagePageRequest,
) (*protocol.MessagePage, error) {
	sessionKey, parsed, err := s.requireSessionKey(rawSessionKey)
	if err != nil {
		return nil, err
	}
	if parsed.Kind == protocol.SessionKeyKindRoom {
		page, err := s.roomHistory.ReadMessagesPage(
			parsed.ConversationID,
			s.activeRoundIDs(sessionKey),
			request.Limit,
			request.BeforeRoundID,
			request.BeforeRoundTimestamp,
		)
		if err != nil {
			return nil, err
		}
		return &page, nil
	}

	workspacePaths, err := s.resolveWorkspacePaths(ctx, parsed.AgentID)
	if err != nil {
		return nil, err
	}
	sessionValue, workspacePath, err := s.loadHistorySession(ctx, workspacePaths, parsed, sessionKey)
	if err != nil {
		return nil, err
	}
	if sessionValue == nil {
		return nil, ErrSessionNotFound
	}
	page, err := s.history.ReadMessagesPage(
		workspacePath,
		*sessionValue,
		s.activeRoundIDs(sessionKey),
		request.Limit,
		request.BeforeRoundID,
		request.BeforeRoundTimestamp,
	)
	if err != nil {
		return nil, err
	}
	return &page, nil
}

func (s *Service) loadHistorySession(
	ctx context.Context,
	workspacePaths []string,
	parsed protocol.SessionKey,
	sessionKey string,
) (*protocol.Session, string, error) {
	roomSession, err := s.repository.GetRoomSessionByKey(ctx, authctx.OwnerUserID(ctx), parsed)
	if err != nil {
		return nil, "", err
	}
	if roomSession != nil {
		workspacePath := resolveHistoryWorkspacePath(workspacePaths, parsed)
		hydrated, hydrateErr := s.hydrateRoomHistorySession(ctx, workspacePath, sessionKey, *roomSession)
		if hydrateErr != nil {
			return nil, "", hydrateErr
		}
		return hydrated, workspacePath, nil
	}

	item, workspacePath, err := s.files.FindSession(workspacePaths, sessionKey)
	if err != nil {
		return nil, "", err
	}
	return item, workspacePath, nil
}

func resolveHistoryWorkspacePath(workspacePaths []string, parsed protocol.SessionKey) string {
	for _, workspacePath := range workspacePaths {
		if filepath.Base(workspacePath) == parsed.AgentID {
			return workspacePath
		}
	}
	if len(workspacePaths) > 0 {
		return workspacePaths[0]
	}
	return ""
}

func (s *Service) hydrateRoomHistorySession(
	ctx context.Context,
	workspacePath string,
	sessionKey string,
	roomSession protocol.Session,
) (*protocol.Session, error) {
	if workspacePath == "" {
		return &roomSession, nil
	}

	fileSession, _, err := s.files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		return nil, err
	}
	if fileSession == nil {
		return &roomSession, nil
	}

	merged := roomSession
	roomSessionID := strings.TrimSpace(stringPointerValue(roomSession.SessionID))
	fileSessionID := strings.TrimSpace(stringPointerValue(fileSession.SessionID))
	if roomSessionID == "" && fileSessionID != "" {
		merged.SessionID = fileSession.SessionID
		if merged.RoomSessionID != nil && strings.TrimSpace(*merged.RoomSessionID) != "" {
			if updateErr := s.repository.UpdateRoomSessionSDKSessionID(
				ctx,
				strings.TrimSpace(*merged.RoomSessionID),
				fileSessionID,
			); updateErr != nil {
				return nil, updateErr
			}
		}
	}
	return &merged, nil
}
