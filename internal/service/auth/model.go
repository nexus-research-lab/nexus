package auth

import (
	"errors"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

var (
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	ErrUserNotFound       = errors.New("user not found")
)

const (
	// SystemUserID 表示未启用认证时的本地单用户保底主体。
	SystemUserID = authctx.SystemUserID

	// RoleOwner 表示单租户默认 owner。
	RoleOwner = authctx.RoleOwner
	// RoleAdmin 表示管理员角色。
	RoleAdmin = authctx.RoleAdmin
	// RoleMember 表示普通成员角色。
	RoleMember = authctx.RoleMember

	// UserStatusActive 表示用户处于可登录状态。
	UserStatusActive = "active"
	// UserStatusDisabled 表示用户已禁用。
	UserStatusDisabled = "disabled"

	// AuthMethodPassword 表示密码登录签发的浏览器 Session。
	AuthMethodPassword = authctx.AuthMethodPassword
	// AuthMethodLocal 表示桌面端本地免登录身份。
	AuthMethodLocal = authctx.AuthMethodLocal
)

type Principal = authctx.Principal
type State = authctx.State

// StatusPayload 表示前端依赖的登录状态响应。
type StatusPayload struct {
	AuthRequired         bool    `json:"auth_required"`
	PasswordLoginEnabled bool    `json:"password_login_enabled"`
	Authenticated        bool    `json:"authenticated"`
	Username             *string `json:"username"`
	UserID               *string `json:"user_id,omitempty"`
	DisplayName          *string `json:"display_name,omitempty"`
	Role                 *string `json:"role,omitempty"`
	Avatar               *string `json:"avatar,omitempty"`
	AuthMethod           *string `json:"auth_method,omitempty"`
	SetupRequired        bool    `json:"setup_required,omitempty"`
	SetupEnabled         bool    `json:"setup_enabled"`
}
