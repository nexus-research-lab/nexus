package authctx

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
	// AuthMethodBearer 表示 ACCESS_TOKEN 的 Bearer 身份。
	AuthMethodBearer = "bearer"
)

// Principal 表示一次已解析的请求身份。
type Principal struct {
	UserID      string  `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name,omitempty"`
	Role        string  `json:"role"`
	Avatar      string  `json:"avatar,omitempty"`
	AuthMethod  string  `json:"auth_method"`
	SessionID   *string `json:"session_id,omitempty"`
}

// State 表示认证域的全局状态摘要。
type State struct {
	SetupRequired        bool `json:"setup_required"`
	AuthRequired         bool `json:"auth_required"`
	PasswordLoginEnabled bool `json:"password_login_enabled"`
	AccessTokenEnabled   bool `json:"access_token_enabled"`
	UserCount            int  `json:"user_count"`
	PasswordUserCount    int  `json:"password_user_count"`
}
