package connectors

import "time"

// OAuthClient 表示用户配置的 OAuth 应用凭据。
type OAuthClient struct {
	OwnerUserID  string
	ConnectorID  string
	ClientID     string
	ClientSecret string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
