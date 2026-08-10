// INPUT: WebSocket 握手时解析的 principal 与当前 transport 人类交互证据。
// OUTPUT: 从数据库或桌面 token 重新核验后的 active human principal。
// POS: 高风险配置批准的人类在场与角色新鲜度边界。
package auth

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

// VerifyInteractiveHuman 拒绝 bearer、无凭据本地 Web 和已失效的浏览器 session。
func (s *Service) VerifyInteractiveHuman(
	ctx context.Context,
	principal *Principal,
) (*Principal, error) {
	if s == nil || principal == nil {
		return nil, errors.New("interactive human principal is required")
	}
	switch strings.TrimSpace(principal.AuthMethod) {
	case AuthMethodLocal:
		evidence, ok := authctx.InteractiveHumanEvidenceFromContext(ctx)
		if !ok || evidence.Source != "desktop_session_token" ||
			strings.TrimSpace(principal.UserID) != SystemUserID {
			return nil, errors.New("desktop human-presence evidence is required")
		}
		return s.VerifyBoundInteractiveHuman(
			ctx,
			principal.UserID,
			principal.AuthMethod,
			"",
		)
	case AuthMethodPassword:
		sessionID := ""
		if principal.SessionID != nil {
			sessionID = strings.TrimSpace(*principal.SessionID)
		}
		return s.VerifyBoundInteractiveHuman(
			ctx,
			principal.UserID,
			principal.AuthMethod,
			sessionID,
		)
	default:
		return nil, errors.New("bearer and non-interactive principals cannot approve configuration changes")
	}
}

// VerifyBoundInteractiveHuman 重新核验 durable flow 固定的人类登录身份。
// 它只接受 password session 或本地桌面 owner；不会把 bearer/runtime
// principal 升级为真人。调用者必须先从可信 flow/transport 固定这些字段。
func (s *Service) VerifyBoundInteractiveHuman(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*Principal, error) {
	principal, release, err := s.AcquireBoundInteractiveHumanLease(
		ctx,
		userID,
		authMethod,
		sessionID,
	)
	if release != nil {
		release()
	}
	return principal, err
}

// AcquireBoundInteractiveHumanLease 在重新核验 durable 人类身份后持有
// 认证读租约。调用者必须在高风险写入、热重载和写后核验全部结束后 release；
// Logout 会等待该租约，从而不会在核验与凭据提交之间竞态撤销同一登录。
func (s *Service) AcquireBoundInteractiveHumanLease(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*Principal, func(), error) {
	if s == nil {
		return nil, nil, errors.New("interactive human verifier is required")
	}
	s.humanSessionMu.RLock()
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(s.humanSessionMu.RUnlock)
	}
	principal, err := s.verifyBoundInteractiveHuman(
		ctx,
		userID,
		authMethod,
		sessionID,
	)
	if err != nil {
		release()
		return nil, nil, err
	}
	return principal, release, nil
}

func (s *Service) verifyBoundInteractiveHuman(
	ctx context.Context,
	userID string,
	authMethod string,
	sessionID string,
) (*Principal, error) {
	userID = strings.TrimSpace(userID)
	authMethod = strings.TrimSpace(authMethod)
	sessionID = strings.TrimSpace(sessionID)
	switch authMethod {
	case AuthMethodLocal:
		if sessionID != "" ||
			!s.desktopAuthBypassEnabled() ||
			userID != SystemUserID {
			return nil, errors.New("desktop human identity is no longer active")
		}
		return s.desktopLocalPrincipal(ctx)
	case AuthMethodPassword:
		if sessionID == "" {
			return nil, errors.New("password session identity is required")
		}
		identity, err := s.repository.GetActiveSessionIdentityByID(
			ctx,
			sessionID,
			s.now(),
		)
		if err != nil {
			return nil, err
		}
		if identity == nil ||
			identity.Status != UserStatusActive ||
			identity.AuthMethod != AuthMethodPassword ||
			identity.UserID != userID {
			return nil, errors.New("password session is expired, revoked, or inactive")
		}
		resolvedSessionID := identity.SessionID
		return &Principal{
			UserID: identity.UserID, Username: identity.Username,
			DisplayName: identity.DisplayName, Role: identity.Role,
			Avatar: identity.Avatar, AuthMethod: AuthMethodPassword,
			SessionID: &resolvedSessionID,
		}, nil
	default:
		return nil, errors.New("bound human identity must use password or local desktop authentication")
	}
}

// ResolveActivePrincipalRole 重新读取用户 active 状态与当前角色。
func (s *Service) ResolveActivePrincipalRole(
	ctx context.Context,
	userID string,
) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", ErrUserNotFound
	}
	if userID == SystemUserID && s.desktopAuthBypassEnabled() {
		user, err := s.localDesktopUser(ctx)
		if err != nil {
			return "", err
		}
		return user.Role, nil
	}
	user, err := s.userByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil || user.Status != UserStatusActive {
		return "", ErrUserNotFound
	}
	return user.Role, nil
}
