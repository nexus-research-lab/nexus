package connectors

import (
	"database/sql"
	"time"
)

// Info 表示连接器列表项。
type Info struct {
	ConnectorID               string   `json:"connector_id"`
	Name                      string   `json:"name"`
	Title                     string   `json:"title"`
	Description               string   `json:"description"`
	Icon                      string   `json:"icon"`
	Category                  string   `json:"category"`
	AuthType                  string   `json:"auth_type"`
	Status                    string   `json:"status"`
	ConnectionState           string   `json:"connection_state"`
	IsConfigured              bool     `json:"is_configured"`
	RequiresExtra             []string `json:"requires_extra,omitempty"`
	ConfigError               *string  `json:"config_error,omitempty"`
	OAuthClientConfigRequired bool     `json:"oauth_client_config_required,omitempty"`
	OAuthClientConfigured     bool     `json:"oauth_client_configured,omitempty"`
}

// Detail 表示连接器详情。
type Detail struct {
	Info
	AuthURL        string          `json:"auth_url,omitempty"`
	TokenURL       string          `json:"token_url,omitempty"`
	Scopes         []string        `json:"scopes"`
	MCPServerURL   string          `json:"mcp_server_url,omitempty"`
	DocsURL        string          `json:"docs_url,omitempty"`
	Features       []string        `json:"features"`
	FeatureDetails []FeatureDetail `json:"feature_details"`
	OAuthClientID  *string         `json:"oauth_client_id,omitempty"`
}

// OAuthClientConfigRequest 表示用户自有 OAuth 应用配置。
type OAuthClientConfigRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// OAuthClientConfig 表示用户已保存的 OAuth 应用配置摘要。
type OAuthClientConfig struct {
	ConnectorID string `json:"connector_id"`
	ClientID    string `json:"client_id,omitempty"`
	Configured  bool   `json:"configured"`
}

// AuthURLResult 表示 OAuth 授权地址。
type AuthURLResult struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

// OAuthCallbackRequest 表示 OAuth 回调请求。
type OAuthCallbackRequest struct {
	Code        string `json:"code"`
	State       string `json:"state"`
	RedirectURI string `json:"redirect_uri"`
}

// DeviceAuthStartResult 表示桌面 Device Flow 的启动信息。
type DeviceAuthStartResult struct {
	ConnectorID             string `json:"connector_id"`
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceAuthPollResult 表示 Device Flow 轮询结果。
type DeviceAuthPollResult struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Connector *Info  `json:"connector,omitempty"`
}

const (
	oauthRedirectKindWeb     = "web"
	oauthRedirectKindDesktop = "desktop"

	deviceAuthStatusPending   = "pending"
	deviceAuthStatusSlowDown  = "slow_down"
	deviceAuthStatusConnected = "connected"
	deviceAuthStatusExpired   = "expired"
	deviceAuthStatusDenied    = "denied"
)

type connectionRecord struct {
	OwnerUserID          string
	ConnectorID          string
	State                string
	Credentials          string
	CredentialsEncrypted sql.NullString
	AuthType             string
	OAuthState           sql.NullString
	OAuthStateExpiresAt  sql.NullTime
}

type stateRow struct {
	OwnerUserID  string
	State        string
	ConnectorID  string
	CodeVerifier string
	RedirectURI  string
	RedirectKind string
	ShopDomain   string
	ExtraJSON    string
	ExpiresAt    time.Time
}
