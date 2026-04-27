package auth

import "time"

const (
	// UserStatusActive 表示 users 表中的活跃用户。
	UserStatusActive = "active"
)

// State 表示认证持久化状态摘要。
type State struct {
	SetupRequired        bool
	AuthRequired         bool
	PasswordLoginEnabled bool
	AccessTokenEnabled   bool
	UserCount            int
	PasswordUserCount    int
}

// UserRecord 表示 users 表的一行记录。
type UserRecord struct {
	UserID      string
	Username    string
	DisplayName string
	Role        string
	Status      string
	Avatar      string
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PasswordCredential 表示 auth_password_credentials 表的一行记录。
type PasswordCredential struct {
	CredentialID      string
	UserID            string
	PasswordHash      string
	PasswordAlgo      string
	PasswordUpdatedAt time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SessionRecord 表示 auth_sessions 表的一行记录。
type SessionRecord struct {
	SessionID        string
	UserID           string
	SessionTokenHash string
	AuthMethod       string
	ExpiresAt        time.Time
	LastSeenAt       time.Time
	ClientIP         string
	UserAgent        string
	RevokedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
