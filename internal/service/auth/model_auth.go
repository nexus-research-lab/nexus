package auth

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
	// AuthMethodBearer 表示 ACCESS_TOKEN 的 Bearer 身份。
	AuthMethodBearer = authctx.AuthMethodBearer
)

// User 表示认证域中的用户实体。
type User struct {
	UserID      string     `json:"user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	Avatar      string     `json:"avatar,omitempty"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

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
	AccessTokenEnabled   bool    `json:"access_token_enabled,omitempty"`
}

// LoginInput 表示密码登录请求。
type LoginInput struct {
	Username  string
	Password  string
	ClientIP  string
	UserAgent string
}

// LoginResult 表示密码登录结果。
type LoginResult struct {
	SessionToken string
	Status       StatusPayload
}

// InitOwnerInput 表示初始化 owner 的输入。
type InitOwnerInput struct {
	Username    string
	DisplayName string
	Password    string
}

// CreateUserInput 表示创建普通用户的输入。
type CreateUserInput struct {
	Username    string
	DisplayName string
	Password    string
	Role        string
}

// ResetPasswordInput 表示重置密码请求。
type ResetPasswordInput struct {
	UserID   string
	Username string
	Password string
}

// ChangePasswordInput 表示当前用户主动修改密码请求。
type ChangePasswordInput struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

// UpdateProfileInput 表示当前用户资料更新请求。
type UpdateProfileInput struct {
	UserID string
	Avatar *string
}
