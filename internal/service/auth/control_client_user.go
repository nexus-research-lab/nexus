// INPUT: 已验证的 Control 密码 Session Principal 与本地 owner 绑定。
// OUTPUT: 当前用户的即时人类确认、角色校验与固定 audience Relay 短令牌。
// POS: Nexus 消费 Control 人类身份的唯一业务边界。
package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

const relayUserPrincipalAudience = "nexus-relay-user"

// ExchangeRelayUserToken 用当前已验证的密码 Session 换取 Relay 短令牌。
// audience 固定在 Nexus 内部，调用方不能改写 Relay 令牌用途。
func (a *ControlAuthority) ExchangeRelayUserToken(
	ctx context.Context,
	principal *Principal,
) (string, error) {
	if principal == nil || strings.TrimSpace(principal.AuthMethod) != AuthMethodPassword {
		return "", errors.New("Relay 令牌必须来自已验证的密码 Session")
	}
	controlUserID := strings.TrimSpace(principal.ControlUserID)
	deploymentID := strings.TrimSpace(principal.DeploymentID)
	sessionID := ""
	if principal.SessionID != nil {
		sessionID = strings.TrimSpace(*principal.SessionID)
	}
	if controlUserID == "" || deploymentID == "" || sessionID == "" {
		return "", errors.New("Relay 令牌缺少 Control 用户、Deployment 或 Session 身份")
	}

	var response struct {
		PrincipalToken string `json:"principal_token"`
	}
	if err := a.call(ctx, http.MethodPost, "/internal/humans/verify", map[string]string{
		"user_id":    controlUserID,
		"session_id": sessionID,
		"audience":   relayUserPrincipalAudience,
	}, &response); err != nil {
		return "", mapControlError(err)
	}
	token := strings.TrimSpace(response.PrincipalToken)
	if token == "" || len(token) > 16*1024 {
		return "", errors.New("Control 返回的 Relay Principal token 无效")
	}
	return token, nil
}

func (a *ControlAuthority) VerifyInteractiveHuman(
	ctx context.Context,
	principal *Principal,
) (*Principal, error) {
	if principal == nil || strings.TrimSpace(principal.AuthMethod) != AuthMethodPassword {
		return nil, errors.New("Control 人类确认必须来自密码 Session")
	}
	sessionID := ""
	if principal.SessionID != nil {
		sessionID = strings.TrimSpace(*principal.SessionID)
	}
	return a.VerifyBoundInteractiveHuman(ctx, principal.UserID, principal.AuthMethod, sessionID)
}

func (a *ControlAuthority) VerifyBoundInteractiveHuman(
	ctx context.Context,
	localOwnerKey string,
	authMethod string,
	sessionID string,
) (*Principal, error) {
	if strings.TrimSpace(authMethod) != AuthMethodPassword || strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("Control 人类确认必须绑定密码 Session")
	}
	binding, err := a.bindings.controlIdentity(ctx, localOwnerKey)
	if err != nil {
		return nil, err
	}
	var response struct {
		PrincipalToken string `json:"principal_token"`
	}
	err = a.call(ctx, http.MethodPost, "/internal/humans/verify", map[string]string{
		"user_id":    binding.ControlUserID,
		"session_id": sessionID,
		"audience":   a.verifier.audience,
	}, &response)
	if err != nil {
		return nil, mapControlError(err)
	}
	claims, err := a.verifier.verify(response.PrincipalToken)
	if err != nil {
		return nil, err
	}
	value := claims.principal()
	if value.UserID != binding.ControlUserID ||
		value.DeploymentID != binding.DeploymentID ||
		value.SessionID != strings.TrimSpace(sessionID) {
		return nil, errors.New("Control 人类 Principal 与本地绑定不匹配")
	}
	return projectControlPrincipal(value, binding.LocalOwnerKey), nil
}

func (a *ControlAuthority) AcquireBoundInteractiveHumanLease(
	ctx context.Context,
	localOwnerKey string,
	authMethod string,
	sessionID string,
) (*Principal, func(), error) {
	a.humanSessionMu.RLock()
	var once sync.Once
	release := func() { once.Do(a.humanSessionMu.RUnlock) }
	principal, err := a.VerifyBoundInteractiveHuman(ctx, localOwnerKey, authMethod, sessionID)
	if err != nil {
		release()
		return nil, nil, err
	}
	// ponytail: 首版所有浏览器认证流都经当前 Nexus gateway，因此本地租约能与
	// Logout 串行；Control 直连开放给其他写入口时，升级为 Control lease receipt。
	return principal, release, nil
}

func (a *ControlAuthority) ResolveActivePrincipalRole(
	ctx context.Context,
	localOwnerKey string,
) (string, error) {
	if strings.TrimSpace(localOwnerKey) == authctx.SystemUserID {
		return "", ErrUserNotFound
	}
	binding, err := a.bindings.controlIdentity(ctx, localOwnerKey)
	if err != nil {
		return "", err
	}
	return a.controlRole(ctx, binding.ControlUserID)
}

func (a *ControlAuthority) controlRole(ctx context.Context, controlUserID string) (string, error) {
	var response struct {
		Role string `json:"role"`
	}
	err := a.call(
		ctx,
		http.MethodGet,
		"/internal/users/"+url.PathEscape(controlUserID)+"/role",
		nil,
		&response,
	)
	if err != nil {
		return "", mapControlError(err)
	}
	switch response.Role {
	case RoleOwner, RoleAdmin, RoleMember:
		return response.Role, nil
	default:
		return "", errors.New("Control 返回了无效角色")
	}
}
