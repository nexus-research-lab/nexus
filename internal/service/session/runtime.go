package session

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) activeRoundIDs(sessionKey string) []string {
	if s.runtime == nil {
		return nil
	}
	return s.runtime.GetRunningRoundIDs(sessionKey)
}

func (s *Service) applyRuntimeStateToSessions(items []protocol.Session) []protocol.Session {
	result := make([]protocol.Session, 0, len(items))
	for _, item := range items {
		result = append(result, s.applyRuntimeStateToSession(item))
	}
	return result
}

func (s *Service) applyRuntimeStateToSession(item protocol.Session) protocol.Session {
	normalized := normalizeSession(item)
	if s.runtime == nil {
		return normalized
	}
	if len(s.activeRoundIDs(normalized.SessionKey)) == 0 {
		normalized.Status = "closed"
		normalized.IsActive = false
		return normalized
	}
	normalized.Status = "active"
	normalized.IsActive = true
	return normalized
}

func (s *Service) reconcileWorkspaceSessionRuntimeState(
	workspacePath string,
	item protocol.Session,
) (protocol.Session, error) {
	normalized := normalizeSession(item)
	if s.runtime == nil || strings.TrimSpace(workspacePath) == "" {
		return normalized, nil
	}
	reconciled := s.applyRuntimeStateToSession(normalized)
	if reconciled.IsActive || (normalized.Status == reconciled.Status && normalized.IsActive == reconciled.IsActive) {
		return reconciled, nil
	}
	updated, err := s.files.UpsertSession(workspacePath, closePersistedSessionMeta(reconciled))
	if err != nil {
		return protocol.Session{}, err
	}
	if updated == nil {
		return reconciled, nil
	}
	return normalizeSession(*updated), nil
}
