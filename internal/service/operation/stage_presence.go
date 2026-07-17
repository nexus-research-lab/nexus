// INPUT: 舞台前端上报的会话键、浏览器实例 ID 与当前时间。
// OUTPUT: 可跨共享 Room key 和成员 runtime key 匹配的短期在线状态。
// POS: operation service 的舞台在线真相源；工具路由不得把组件挂载误判为舞台已打开。
package operation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const defaultStagePresenceLifetime = 90 * time.Second

var ErrInvalidStagePresence = errors.New("invalid operation stage presence")

// StagePresence 描述一个仍有效的舞台浏览器实例。
type StagePresence struct {
	SessionKey string `json:"session_key"`
	ClientID   string `json:"client_id"`
	Active     bool   `json:"active"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

// TouchStagePresence 刷新一个显式打开的舞台实例。
func (s *Service) TouchStagePresence(
	ctx context.Context,
	sessionKey string,
	clientID string,
) (*StagePresence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	scope, normalizedSessionKey, normalizedClientID, err := normalizeStagePresenceIdentity(sessionKey, clientID)
	if err != nil {
		return nil, err
	}

	now := s.currentTime()
	expiresAt := now.Add(s.presenceLifetime)
	s.presenceMu.Lock()
	s.pruneExpiredPresenceLocked(now)
	clients := s.presenceByScope[scope]
	if clients == nil {
		clients = make(map[string]time.Time)
		s.presenceByScope[scope] = clients
	}
	clients[normalizedClientID] = expiresAt
	s.presenceMu.Unlock()

	return &StagePresence{
		SessionKey: normalizedSessionKey,
		ClientID:   normalizedClientID,
		Active:     true,
		ExpiresAt:  expiresAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

// CloseStagePresence 只关闭当前浏览器实例，不影响同一会话的其他舞台窗口。
func (s *Service) CloseStagePresence(
	ctx context.Context,
	sessionKey string,
	clientID string,
) (*StagePresence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	scope, normalizedSessionKey, normalizedClientID, err := normalizeStagePresenceIdentity(sessionKey, clientID)
	if err != nil {
		return nil, err
	}

	s.presenceMu.Lock()
	if clients := s.presenceByScope[scope]; clients != nil {
		delete(clients, normalizedClientID)
		if len(clients) == 0 {
			delete(s.presenceByScope, scope)
		}
	}
	s.presenceMu.Unlock()

	return &StagePresence{
		SessionKey: normalizedSessionKey,
		ClientID:   normalizedClientID,
		Active:     false,
	}, nil
}

// IsStageActive 报告 runtime 会话当前是否有可见舞台实例。
func (s *Service) IsStageActive(sessionKey string) bool {
	scope, err := normalizeStagePresenceScope(sessionKey)
	if err != nil {
		return false
	}
	now := s.currentTime()
	s.presenceMu.Lock()
	defer s.presenceMu.Unlock()
	s.pruneExpiredPresenceLocked(now)
	return len(s.presenceByScope[scope]) > 0
}

func (s *Service) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Service) pruneExpiredPresenceLocked(now time.Time) {
	for scope, clients := range s.presenceByScope {
		for clientID, expiresAt := range clients {
			if !expiresAt.After(now) {
				delete(clients, clientID)
			}
		}
		if len(clients) == 0 {
			delete(s.presenceByScope, scope)
		}
	}
}

func normalizeStagePresenceIdentity(sessionKey string, clientID string) (string, string, string, error) {
	normalizedSessionKey := strings.TrimSpace(sessionKey)
	normalizedClientID := strings.TrimSpace(clientID)
	if normalizedClientID == "" || len(normalizedClientID) > 128 {
		return "", "", "", ErrInvalidStagePresence
	}
	scope, err := normalizeStagePresenceScope(normalizedSessionKey)
	if err != nil {
		return "", "", "", err
	}
	return scope, normalizedSessionKey, normalizedClientID, nil
}

func normalizeStagePresenceScope(sessionKey string) (string, error) {
	normalized := strings.TrimSpace(sessionKey)
	if normalized == "" || len(normalized) > 512 {
		return "", ErrInvalidStagePresence
	}
	parsed := protocol.ParseSessionKey(normalized)
	if parsed.IsStructured && strings.TrimSpace(parsed.ConversationID) != "" {
		return "conversation:" + strings.TrimSpace(parsed.ConversationID), nil
	}
	if parsed.IsStructured &&
		parsed.Kind == protocol.SessionKeyKindAgent &&
		strings.EqualFold(strings.TrimSpace(parsed.ChatType), "group") &&
		strings.TrimSpace(parsed.Ref) != "" {
		return "conversation:" + strings.TrimSpace(parsed.Ref), nil
	}
	return "session:" + normalized, nil
}
