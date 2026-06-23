package channels

import (
	"strings"
	"time"
)

func (s *ControlService) getChannelLoginSession(ownerUserID string, channelType string, loginID string) (*channelLoginSession, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	loginID = strings.TrimSpace(loginID)
	if loginID == "" {
		return nil, ErrChannelLoginNotFound
	}
	store := s.effectiveChannelLoginStore()
	store.mu.Lock()
	session := store.sessions[loginID]
	store.mu.Unlock()
	if session == nil || session.ownerUserID != ownerUserID || session.channelType != channelType {
		return nil, ErrChannelLoginNotFound
	}
	return session, nil
}

func (s *ControlService) finishChannelLoginSession(session *channelLoginSession) {
	store := s.effectiveChannelLoginStore()
	store.mu.Lock()
	if store.active[session.activeKey] == session.view.LoginID {
		delete(store.active, session.activeKey)
	}
	store.mu.Unlock()
}

func (s *ControlService) effectiveChannelLoginStore() *channelLoginStore {
	if s.loginStore == nil {
		s.loginStore = newChannelLoginStore()
	}
	return s.loginStore
}

func (s *channelLoginStore) pruneLocked(now time.Time) {
	for loginID, session := range s.sessions {
		view := session.snapshot()
		if channelLoginIsActive(view.Status) {
			continue
		}
		if view.FinishedAt != nil && now.Sub(*view.FinishedAt) > 10*time.Minute {
			delete(s.sessions, loginID)
			if s.active[session.activeKey] == loginID {
				delete(s.active, session.activeKey)
			}
		}
	}
}

func channelLoginActiveKey(ownerUserID string, channelType string) string {
	return strings.TrimSpace(ownerUserID) + "\x00" + normalizeIMChannelType(channelType)
}

func channelLoginIsActive(status string) bool {
	switch strings.TrimSpace(status) {
	case ChannelLoginStatusRunning, ChannelLoginStatusVerifyCodeRequired:
		return true
	default:
		return false
	}
}
