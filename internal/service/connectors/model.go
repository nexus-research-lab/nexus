package connectors

import (
	"database/sql"
	"time"
)

const (
	// ConnectorKindCatalog 表示 Nexus 内置连接器目录项。
	ConnectorKindCatalog = "connector"
	// ConnectorKindCustomMCP 表示用户创建的自定义 MCP server。
	ConnectorKindCustomMCP = "custom_mcp"
)

// Info 表示连接器列表项。
type Info struct {
	ConnectorID               string   `json:"connector_id"`
	Kind                      string   `json:"kind"`
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
	SupportsDeviceAuth        bool     `json:"supports_device_auth,omitempty"`
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

// ConfigurationState 是单个 Connector 的 owner-scoped 持久配置版本。
// 它只暴露凭据是否存在，不返回可恢复的秘密。
type ConfigurationState struct {
	ConnectorID           string `json:"connector_id"`
	ConfigurationVersion  int64  `json:"configuration_version"`
	ConnectionExists      bool   `json:"connection_exists"`
	ConnectionState       string `json:"connection_state,omitempty"`
	ConnectionAuthType    string `json:"connection_auth_type,omitempty"`
	ConnectionConfigured  bool   `json:"connection_credentials_configured"`
	OAuthClientExists     bool   `json:"oauth_client_exists"`
	OAuthClientID         string `json:"oauth_client_id,omitempty"`
	OAuthClientConfigured bool   `json:"oauth_client_configured"`
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

// DeviceAuthStartMode 表示调用方显式选择的飞书连接方式。
type DeviceAuthStartMode string

const (
	DeviceAuthStartModeOfficialQR        DeviceAuthStartMode = "official_qr"
	DeviceAuthStartModeManualCredentials DeviceAuthStartMode = "manual_credentials"
)

// DeviceAuthStartResult 表示 Device Flow 或前置应用选择/创建阶段的启动信息。
type DeviceAuthStartResult struct {
	ConnectorID             string `json:"connector_id"`
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	Stage                   string `json:"stage,omitempty"`
}

// DeviceAuthPollResult 表示 Device Flow 轮询结果。
type DeviceAuthPollResult struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Connector *Info                  `json:"connector,omitempty"`
	Next      *DeviceAuthStartResult `json:"next,omitempty"`
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
	ConfigurationVersion int64
}

type stateRow struct {
	OwnerUserID   string
	State         string
	ConnectorID   string
	CodeVerifier  string
	RedirectURI   string
	RedirectKind  string
	ShopDomain    string
	ExtraJSON     string
	ControlFlowID string
	ExpiresAt     time.Time
}
