// INPUT: owner、频道与进程内扫码会话索引。
// OUTPUT: exact login 查询，以及唯一未绑定 Web 扫码会话的只读对账结果。
// POS: Channel 登录会话存储边界；读取不启动平台注册、不生成 login_id，也不修复异常索引。
package channels

import (
	"context"
	"strings"
	"time"
)

// GetCurrentChannelLogin returns the one active browser-started login for the
// exact owner and Channel. An absent session is not evidence that a preceding
// start was rejected; callers must keep the write result unproven. Corrupt,
// ambiguous, or conversationally bound state fails closed.
func (s *ControlService) GetCurrentChannelLogin(
	_ context.Context,
	ownerUserID string,
	channelType string,
) (*ChannelLoginView, error) {
	ownerUserID = normalizeChannelOwnerUserID(ownerUserID)
	channelType = normalizeIMChannelType(channelType)
	catalog, ok := channelCatalogByType(channelType)
	if !ok {
		return nil, ErrChannelNotFound
	}
	if !catalog.SupportsQRCode {
		return nil, ErrChannelLoginUnsupported
	}

	store := s.effectiveChannelLoginStore()
	activeKey := channelLoginActiveKey(ownerUserID, channelType)
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(time.Now())

	activeID := store.active[activeKey]
	type activeChannelLogin struct {
		loginID string
		session *channelLoginSession
		view    ChannelLoginView
	}
	candidates := make([]activeChannelLogin, 0, 1)
	for loginID, session := range store.sessions {
		if session == nil ||
			session.ownerUserID != ownerUserID ||
			session.channelType != channelType {
			continue
		}
		view := session.snapshot()
		if !channelLoginIsActive(view.Status) {
			continue
		}
		candidates = append(candidates, activeChannelLogin{
			loginID: loginID,
			session: session,
			view:    view,
		})
	}

	if len(candidates) == 0 {
		if activeID != "" {
			return nil, ErrChannelLoginState
		}
		return nil, ErrChannelLoginNotFound
	}
	if len(candidates) != 1 {
		return nil, ErrChannelLoginState
	}
	candidate := candidates[0]
	if activeID == "" ||
		activeID != candidate.loginID ||
		candidate.view.LoginID != candidate.loginID ||
		normalizeIMChannelType(candidate.view.ChannelType) != channelType ||
		candidate.session.activeKey != activeKey ||
		strings.TrimSpace(candidate.session.authorizationBinding) != "" {
		return nil, ErrChannelLoginState
	}
	return &candidate.view, nil
}

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
