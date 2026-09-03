package authctx

import (
	"context"
	"strings"
)

const (
	// SystemUserID 表示未启用认证时的本地单用户保底主体。
	SystemUserID = "__system__"

	// RoleOwner 表示单租户默认 owner。
	RoleOwner = "owner"
	// RoleAdmin 表示管理员角色。
	RoleAdmin = "admin"
	// RoleMember 表示普通成员角色。
	RoleMember = "member"

	// AuthMethodPassword 表示密码登录签发的浏览器 Session。
	AuthMethodPassword = "password"
	// AuthMethodLocal 表示桌面端本地免登录身份。
	AuthMethodLocal = "local"
)

// Principal 表示一次已解析的请求身份。
type Principal struct {
	UserID        string  `json:"user_id"`
	ControlUserID string  `json:"control_user_id,omitempty"`
	DeploymentID  string  `json:"deployment_id,omitempty"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"display_name,omitempty"`
	Role          string  `json:"role"`
	Avatar        string  `json:"avatar,omitempty"`
	AuthMethod    string  `json:"auth_method"`
	SessionID     *string `json:"session_id,omitempty"`
}

// State 表示认证域的全局状态摘要。
type State struct {
	SetupRequired        bool `json:"setup_required"`
	SetupEnabled         bool `json:"setup_enabled"`
	AuthRequired         bool `json:"auth_required"`
	PasswordLoginEnabled bool `json:"password_login_enabled"`
}

type principalContextKey struct{}
type stateContextKey struct{}
type interactiveHumanContextKey struct{}
type queuedHumanPrincipalContextKey struct{}

// InteractiveHumanEvidence 是宿主 transport 已验证的人类交互凭据种类。
type InteractiveHumanEvidence struct {
	Source string
}

// QueuedHumanPrincipalBinding is the opaque host-auth identity restored from a
// durable queue admission. SessionID is a database row ID, not a credential.
// Roles are deliberately absent and must be resolved again at tool execution.
type QueuedHumanPrincipalBinding struct {
	UserID     string
	AuthMethod string
	SessionID  string
}

// WithPrincipal 把认证后的主体写入请求上下文。
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	if principal == nil {
		return ctx
	}
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext 读取请求上下文中的主体。
func PrincipalFromContext(ctx context.Context) *Principal {
	principal, _ := ctx.Value(principalContextKey{}).(*Principal)
	return principal
}

// CurrentUserID 从上下文读取当前用户标识。
func CurrentUserID(ctx context.Context) (string, bool) {
	principal := PrincipalFromContext(ctx)
	if principal == nil {
		return "", false
	}
	userID := strings.TrimSpace(principal.UserID)
	if userID == "" {
		return "", false
	}
	return userID, true
}

// OwnerUserID 返回当前请求的 owner 用户标识，未绑定认证主体时回落到本地单用户主体。
func OwnerUserID(ctx context.Context) string {
	if userID, ok := CurrentUserID(ctx); ok {
		return userID
	}
	return SystemUserID
}

// WithState 把认证系统状态写入请求上下文。
func WithState(ctx context.Context, state State) context.Context {
	return context.WithValue(ctx, stateContextKey{}, state)
}

// StateFromContext 读取请求上下文中的认证系统状态。
func StateFromContext(ctx context.Context) (State, bool) {
	state, ok := ctx.Value(stateContextKey{}).(State)
	return state, ok
}

// WithInteractiveHumanEvidence 标记本次请求经过了不可由 Agent runtime
// 取得的宿主交互凭据验证。
func WithInteractiveHumanEvidence(ctx context.Context, source string) context.Context {
	source = strings.TrimSpace(source)
	if source == "" {
		return ctx
	}
	return context.WithValue(ctx, interactiveHumanContextKey{}, InteractiveHumanEvidence{Source: source})
}

// InteractiveHumanEvidenceFromContext 返回宿主验证的人类交互证据。
func InteractiveHumanEvidenceFromContext(ctx context.Context) (InteractiveHumanEvidence, bool) {
	evidence, ok := ctx.Value(interactiveHumanContextKey{}).(InteractiveHumanEvidence)
	return evidence, ok && strings.TrimSpace(evidence.Source) != ""
}

// WithQueuedHumanPrincipalBinding carries a claimed host-DB binding only
// through the service call graph that constructs an in-process MCP server.
// Invalid or runtime-derived identities are ignored.
func WithQueuedHumanPrincipalBinding(
	ctx context.Context,
	binding QueuedHumanPrincipalBinding,
) context.Context {
	binding.UserID = strings.TrimSpace(binding.UserID)
	binding.AuthMethod = strings.TrimSpace(binding.AuthMethod)
	binding.SessionID = strings.TrimSpace(binding.SessionID)
	if !validQueuedHumanPrincipalBinding(binding) {
		return ctx
	}
	return context.WithValue(ctx, queuedHumanPrincipalContextKey{}, binding)
}

// QueuedHumanPrincipalBindingFromContext returns a previously validated,
// database-backed queue principal binding.
func QueuedHumanPrincipalBindingFromContext(
	ctx context.Context,
) (QueuedHumanPrincipalBinding, bool) {
	binding, ok := ctx.Value(queuedHumanPrincipalContextKey{}).(QueuedHumanPrincipalBinding)
	if !ok || !validQueuedHumanPrincipalBinding(binding) {
		return QueuedHumanPrincipalBinding{}, false
	}
	return binding, true
}

// DirectHumanPrincipalBindingFromContext resolves the host-auth identity that
// may be persisted with a direct WebSocket queue admission. Local single-user
// Web mode has no login principal, so its authenticated host boundary is
// represented by the sessionless system owner instead of rejecting the input.
func DirectHumanPrincipalBindingFromContext(
	ctx context.Context,
	ownerUserID string,
) (QueuedHumanPrincipalBinding, bool) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return QueuedHumanPrincipalBinding{}, false
	}
	principal := PrincipalFromContext(ctx)
	if principal == nil {
		if !IsLocalSingleUserControlPlane(ctx, ownerUserID) {
			return QueuedHumanPrincipalBinding{}, false
		}
		return QueuedHumanPrincipalBinding{
			UserID:     SystemUserID,
			AuthMethod: AuthMethodLocal,
		}, true
	}
	binding := QueuedHumanPrincipalBinding{
		UserID:     strings.TrimSpace(principal.UserID),
		AuthMethod: strings.TrimSpace(principal.AuthMethod),
	}
	if principal.SessionID != nil {
		binding.SessionID = strings.TrimSpace(*principal.SessionID)
	}
	if binding.UserID != ownerUserID || !validQueuedHumanPrincipalBinding(binding) {
		return QueuedHumanPrincipalBinding{}, false
	}
	return binding, true
}

func validQueuedHumanPrincipalBinding(binding QueuedHumanPrincipalBinding) bool {
	userID := strings.TrimSpace(binding.UserID)
	authMethod := strings.TrimSpace(binding.AuthMethod)
	sessionID := strings.TrimSpace(binding.SessionID)
	if userID == "" {
		return false
	}
	switch authMethod {
	case AuthMethodPassword:
		return sessionID != ""
	case AuthMethodLocal:
		return userID == SystemUserID && sessionID == ""
	default:
		return false
	}
}

// IsLocalSingleUserControlPlane 只认可桌面本地免登录的 system owner。
// 多用户 owner/admin 仍是人类管理角色，但不能因此让其 Agent runtime
// 继承宿主进程身份或原始 nexusctl。
func IsLocalSingleUserControlPlane(ctx context.Context, ownerUserID string) bool {
	state, ok := StateFromContext(ctx)
	if !ok || state.AuthRequired || strings.TrimSpace(ownerUserID) != SystemUserID {
		return false
	}
	principal := PrincipalFromContext(ctx)
	if principal == nil {
		return true
	}
	return strings.TrimSpace(principal.UserID) == SystemUserID &&
		strings.TrimSpace(principal.Role) == RoleOwner &&
		strings.TrimSpace(principal.AuthMethod) == AuthMethodLocal
}
