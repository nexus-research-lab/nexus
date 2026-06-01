package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/connectors/providers"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/storage"
	connectorstore "github.com/nexus-research-lab/nexus/internal/storage/connectors"
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

// Service 提供连接器目录、授权与状态能力。
type Service struct {
	config     config.Config
	db         *sql.DB
	driver     string
	httpClient *http.Client
}

// NewService 创建连接器服务。
func NewService(cfg config.Config, db *sql.DB) *Service {
	driver := storage.NormalizeSQLDriver(cfg.DatabaseDriver)
	return &Service{
		config: cfg,
		db:     db,
		driver: driver,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// ListConnectors 列出连接器目录。
func (s *Service) ListConnectors(ctx context.Context, ownerUserID string, query string, category string, status string) ([]Info, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	states, err := s.listConnectionStates(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	configErrors := s.listOAuthConfigErrors(ctx, ownerUserID)
	needle := strings.ToLower(strings.TrimSpace(query))
	items := make([]Info, 0, len(connectorCatalog))
	for _, entry := range connectorCatalog {
		if category != "" && entry.Category != category {
			continue
		}
		if status != "" && entry.Status != status {
			continue
		}
		if needle != "" && !connectorMatches(entry, needle) {
			continue
		}
		items = append(items, s.toInfoWithConfigError(entry, connectorFirstNonEmpty(states[entry.ConnectorID], "disconnected"), configErrors[entry.ConnectorID]))
	}
	return items, nil
}

// GetConnectorDetail 返回单个连接器详情。
func (s *Service) GetConnectorDetail(ctx context.Context, ownerUserID string, connectorID string) (*Detail, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("connector not found")
	}
	state, err := s.connectionState(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	detail := s.toDetail(ctx, ownerUserID, entry, connectorFirstNonEmpty(state, "disconnected"))
	return &detail, nil
}

// GetConnectedCount 返回当前用户已连接数量。
func (s *Service) GetConnectedCount(ctx context.Context, ownerUserID string) (int, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	query := fmt.Sprintf(
		"SELECT COUNT(1) FROM connector_connections WHERE owner_user_id = %s AND state = 'connected'",
		s.bind(1),
	)
	var count int
	if err := s.db.QueryRowContext(ctx, query, ownerUserID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetOAuthClientConfig 返回用户自有 OAuth 应用配置摘要，不返回 Secret。
func (s *Service) GetOAuthClientConfig(ctx context.Context, ownerUserID string, connectorID string) (*OAuthClientConfig, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	return s.oauthClientConfig(ctx, ownerUserID, entry)
}

// SaveOAuthClientConfig 保存用户自有 OAuth 应用配置。
func (s *Service) SaveOAuthClientConfig(ctx context.Context, ownerUserID string, connectorID string, request OAuthClientConfigRequest) (*Info, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if !entry.UserOAuthClient {
		return nil, errors.New("当前连接器不支持用户自定义 OAuth 应用")
	}
	clientID := strings.TrimSpace(request.ClientID)
	clientSecret := strings.TrimSpace(request.ClientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, errors.New("OAuth Client ID / Secret 不能为空")
	}
	store, err := s.oauthClientStore()
	if err != nil {
		return nil, err
	}
	if err = store.Upsert(ctx, connectorstore.OAuthClient{
		OwnerUserID:  ownerUserID,
		ConnectorID:  entry.ConnectorID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}); err != nil {
		return nil, err
	}
	state, err := s.connectionState(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, connectorFirstNonEmpty(state, "disconnected"))
	return &info, nil
}

// DeleteOAuthClientConfig 删除用户自有 OAuth 应用配置，并断开依赖该配置的连接。
func (s *Service) DeleteOAuthClientConfig(ctx context.Context, ownerUserID string, connectorID string) (*Info, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if !entry.UserOAuthClient {
		return nil, errors.New("当前连接器不支持用户自定义 OAuth 应用")
	}
	store, err := s.oauthClientStore()
	if err != nil {
		return nil, err
	}
	if err = store.Delete(ctx, ownerUserID, entry.ConnectorID); err != nil {
		return nil, err
	}
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "disconnected",
		Credentials: "",
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "disconnected")
	return &info, nil
}

// ListActiveConnections 列出当前用户已连接 connector。
func (s *Service) ListActiveConnections(ctx context.Context, ownerUserID string) ([]connectordomain.ConnectionSnapshot, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	query := fmt.Sprintf(
		"SELECT owner_user_id, connector_id, credentials, credentials_encrypted, auth_type FROM connector_connections WHERE owner_user_id = %s AND state = 'connected'",
		s.bind(1),
	)
	rows, err := s.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []connectordomain.ConnectionSnapshot{}
	for rows.Next() {
		var record connectionRecord
		if err = rows.Scan(
			&record.OwnerUserID,
			&record.ConnectorID,
			&record.Credentials,
			&record.CredentialsEncrypted,
			&record.AuthType,
		); err != nil {
			return nil, err
		}
		item, err := s.connectionSnapshotFromRecord(record)
		if err != nil {
			return nil, err
		}
		if item != nil {
			result = append(result, *item)
		}
	}
	return result, rows.Err()
}

// LoadActiveConnection 读取已连接 connector 的 token 快照。
func (s *Service) LoadActiveConnection(ctx context.Context, ownerUserID, connectorID string) (*connectordomain.ConnectionSnapshot, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	query := fmt.Sprintf(
		"SELECT owner_user_id, connector_id, credentials, credentials_encrypted, auth_type FROM connector_connections WHERE owner_user_id = %s AND connector_id = %s AND state = 'connected'",
		s.bind(1),
		s.bind(2),
	)
	var record connectionRecord
	err := s.db.QueryRowContext(ctx, query, ownerUserID, strings.TrimSpace(connectorID)).Scan(
		&record.OwnerUserID,
		&record.ConnectorID,
		&record.Credentials,
		&record.CredentialsEncrypted,
		&record.AuthType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record.OwnerUserID = ownerUserID
	record, err = s.refreshActiveConnectionIfNeeded(ctx, ownerUserID, record)
	if err != nil {
		return nil, err
	}
	return s.connectionSnapshotFromRecord(record)
}

func (s *Service) connectionSnapshotFromRecord(record connectionRecord) (*connectordomain.ConnectionSnapshot, error) {
	entry, ok := getConnector(record.ConnectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	payload, err := s.connectionCredentialsPayload(record)
	if err != nil {
		return nil, err
	}
	parsed, err := credentialMapFromPayload(payload)
	if err != nil {
		return nil, err
	}
	token := connectorFirstNonEmpty(parsed["access_token"], parsed["token"], parsed["bearer_token"], parsed["api_key"])
	if token == "" {
		return nil, errors.New("connector 未获取到 access token")
	}
	delete(parsed, "access_token")
	delete(parsed, "token")
	delete(parsed, "bearer_token")
	delete(parsed, "api_key")
	shop := connectorFirstNonEmpty(parsed["shop"], parsed["shop_domain"])
	return &connectordomain.ConnectionSnapshot{
		ConnectorID: record.ConnectorID,
		AuthType:    record.AuthType,
		APIBaseURL:  entry.APIBaseURL,
		AccessToken: token,
		ShopDomain:  shop,
		Extra:       parsed,
	}, nil
}

func (s *Service) refreshActiveConnectionIfNeeded(ctx context.Context, ownerUserID string, record connectionRecord) (connectionRecord, error) {
	if record.ConnectorID != "feishu-docx" {
		return record, nil
	}
	payload, err := s.connectionCredentialsPayload(record)
	if err != nil {
		return record, err
	}
	current, err := credentialMapFromPayload(payload)
	if err != nil {
		return record, err
	}
	if !credentialNeedsRefresh(current) {
		return record, nil
	}
	refreshToken := strings.TrimSpace(current["refresh_token"])
	if refreshToken == "" {
		return record, nil
	}
	provider, err := providers.Get(record.ConnectorID)
	if err != nil {
		return record, err
	}
	refreshProvider, ok := provider.(providers.RefreshTokenProvider)
	if !ok {
		return record, nil
	}
	clientID, clientSecret, err := s.oauthCredentials(ctx, ownerUserID, record.ConnectorID)
	if err != nil {
		return record, err
	}
	payload, err = refreshProvider.RefreshToken(ctx, s.httpClient, providers.TokenRefreshRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	})
	if err != nil {
		return record, err
	}
	updated, err := credentialMapFromPayload([]byte(normalizeOAuthPayload(payload)))
	if err != nil {
		return record, err
	}
	for key, value := range current {
		if _, exists := updated[key]; !exists {
			updated[key] = value
		}
	}
	encoded, err := json.Marshal(updated)
	if err != nil {
		return record, err
	}
	record.Credentials = string(encoded)
	record.CredentialsEncrypted = sql.NullString{}
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: record.ConnectorID,
		State:       "connected",
		Credentials: record.Credentials,
		AuthType:    record.AuthType,
	}); err != nil {
		return record, err
	}
	return record, nil
}

func credentialNeedsRefresh(credentials map[string]string) bool {
	expiresAtRaw := strings.TrimSpace(credentials["expires_at"])
	if expiresAtRaw == "" {
		return false
	}
	expiresAt, err := strconv.ParseFloat(expiresAtRaw, 64)
	if err != nil {
		return false
	}
	return time.Unix(int64(expiresAt), 0).Before(time.Now().Add(5 * time.Minute))
}

// GetCategories 返回连接器分类映射。
func (s *Service) GetCategories() map[string]string {
	result := make(map[string]string, len(categoryLabels))
	for key, value := range categoryLabels {
		result[key] = value
	}
	return result
}

// RequiredExtraKeys 返回连接器授权时允许透传的额外参数。
func (s *Service) RequiredExtraKeys(connectorID string) []string {
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil
	}
	providerID := connectorFirstNonEmpty(entry.Provider, entry.ConnectorID)
	provider, err := providers.Get(providerID)
	if err != nil {
		return append([]string{}, entry.RequiresExtra...)
	}
	return append([]string{}, provider.RequiredExtraKeys()...)
}

// GetAuthURL 生成 OAuth 授权地址。
func (s *Service) GetAuthURL(ctx context.Context, ownerUserID string, connectorID string, redirectURI string, extras map[string]string) (*AuthURLResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	if err := s.purgeExpiredStates(ctx); err != nil {
		return nil, err
	}
	providerID := connectorFirstNonEmpty(entry.Provider, entry.ConnectorID)
	provider, err := providers.Get(providerID)
	if err != nil {
		return nil, err
	}
	normalizedExtras := normalizeExtras(extras)
	for _, key := range provider.RequiredExtraKeys() {
		if strings.TrimSpace(normalizedExtras[key]) == "" {
			return nil, fmt.Errorf("%s 参数缺失", key)
		}
	}
	clientID, _, configErr := s.oauthCredentials(ctx, ownerUserID, entry.ConnectorID)
	if configErr != nil {
		return nil, configErr
	}
	resolvedRedirectURI := strings.TrimSpace(redirectURI)
	if resolvedRedirectURI == "" {
		resolvedRedirectURI = s.config.ConnectorOAuthRedirectURI
	}
	if err := s.validateRedirectURI(resolvedRedirectURI); err != nil {
		return nil, err
	}
	redirectKind := oauthRedirectKind(resolvedRedirectURI)
	var verifier string
	var challenge string
	if provider.RequiresPKCE() {
		verifier, challenge, err = providers.GeneratePKCE()
		if err != nil {
			return nil, err
		}
	}
	state, err := providers.RandomState()
	if err != nil {
		return nil, err
	}
	extraJSON, err := json.Marshal(normalizedExtras)
	if err != nil {
		return nil, err
	}
	if err = s.insertState(ctx, stateRow{
		OwnerUserID:  ownerUserID,
		State:        state,
		ConnectorID:  entry.ConnectorID,
		CodeVerifier: verifier,
		RedirectURI:  resolvedRedirectURI,
		RedirectKind: redirectKind,
		ShopDomain:   normalizedExtras["shop"],
		ExtraJSON:    string(extraJSON),
		ExpiresAt:    time.Now().Add(s.oauthStateTTL()),
	}); err != nil {
		return nil, err
	}
	authURL, err := provider.BuildAuthURL(ctx, providers.AuthRequest{
		ClientID:     clientID,
		RedirectURI:  resolvedRedirectURI,
		Scopes:       entry.Scopes,
		State:        state,
		CodeVerifier: challenge,
		Extra:        normalizedExtras,
	})
	if err != nil {
		return nil, err
	}
	return &AuthURLResult{
		AuthURL: authURL,
		State:   state,
	}, nil
}

// CompleteOAuthCallback 完成 OAuth token 交换。
func (s *Service) CompleteOAuthCallback(ctx context.Context, ownerUserID string, request OAuthCallbackRequest) (*Info, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	stateValue := strings.TrimSpace(request.State)
	state, err := s.consumeState(ctx, ownerUserID, stateValue)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, errors.New("OAuth state 无效或已过期")
	}
	if state.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("OAuth state 无效或已过期")
	}
	entry, ok := getConnector(state.ConnectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	requestRedirectURI := strings.TrimSpace(request.RedirectURI)
	if requestRedirectURI != "" && state.RedirectURI != "" && requestRedirectURI != state.RedirectURI {
		return nil, errors.New("redirect URI 不匹配")
	}
	if err := s.validateRedirectURI(connectorFirstNonEmpty(requestRedirectURI, state.RedirectURI)); err != nil {
		return nil, err
	}
	extra, err := state.extra()
	if err != nil {
		return nil, err
	}
	providerID := connectorFirstNonEmpty(entry.Provider, entry.ConnectorID)
	provider, err := providers.Get(providerID)
	if err != nil {
		return nil, err
	}
	clientID, clientSecret, configErr := s.oauthCredentials(ctx, ownerUserID, entry.ConnectorID)
	if configErr != nil {
		return nil, configErr
	}
	payload, err := provider.ExchangeToken(ctx, s.httpClient, providers.TokenRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  state.RedirectURI,
		Code:         strings.TrimSpace(request.Code),
		CodeVerifier: state.CodeVerifier,
		Extra:        extra,
	})
	if err != nil {
		return nil, err
	}
	credentials := mergeCredentialExtras(normalizeOAuthPayload(payload), extra)
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: credentials,
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &info, nil
}

// StartDeviceAuth 启动支持桌面公共客户端的 OAuth Device Flow。
func (s *Service) StartDeviceAuth(ctx context.Context, ownerUserID string, connectorID string) (*DeviceAuthStartResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	clientID, err := s.oauthPublicClientID(ctx, ownerUserID, entry.ConnectorID, entry.Title)
	if err != nil {
		return nil, err
	}
	response, err := provider.RequestDeviceCode(ctx, s.httpClient, providers.DeviceCodeRequest{
		ClientID: clientID,
		Scopes:   entry.Scopes,
	})
	if err != nil {
		return nil, friendlyDeviceAuthError(err)
	}
	return &DeviceAuthStartResult{
		ConnectorID:             entry.ConnectorID,
		DeviceCode:              response.DeviceCode,
		UserCode:                response.UserCode,
		VerificationURI:         response.VerificationURI,
		VerificationURIComplete: response.VerificationURIComplete,
		ExpiresIn:               response.ExpiresIn,
		Interval:                response.Interval,
	}, nil
}

// PollDeviceAuth 轮询 OAuth Device Flow，并在成功后保存连接凭证。
func (s *Service) PollDeviceAuth(ctx context.Context, ownerUserID string, connectorID string, deviceCode string) (*DeviceAuthPollResult, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	if strings.TrimSpace(deviceCode) == "" {
		return nil, errors.New("device_code 不能为空")
	}
	provider, err := s.deviceProvider(entry)
	if err != nil {
		return nil, err
	}
	clientID, err := s.oauthPublicClientID(ctx, ownerUserID, entry.ConnectorID, entry.Title)
	if err != nil {
		return nil, err
	}
	payload, err := provider.ExchangeDeviceToken(ctx, s.httpClient, providers.DeviceTokenRequest{
		ClientID:   clientID,
		DeviceCode: deviceCode,
	})
	if err != nil {
		status := deviceAuthStatusFromError(err)
		if status != "" {
			return &DeviceAuthPollResult{
				Status:  status,
				Message: deviceAuthMessage(status),
			}, nil
		}
		return nil, friendlyDeviceAuthError(err)
	}
	credentials := normalizeOAuthPayload(payload)
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: credentials,
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &DeviceAuthPollResult{
		Status:    deviceAuthStatusConnected,
		Connector: &info,
	}, nil
}

// Connect 使用显式凭证直接连接。
func (s *Service) Connect(ctx context.Context, ownerUserID string, connectorID string, credentials map[string]string) (*Info, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	if entry.AuthType == "oauth2" {
		return nil, errors.New("OAuth2 连接器请先调用 auth-url 完成授权")
	}
	normalizedCredentials, err := normalizeDirectCredentials(entry, credentials)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(normalizedCredentials)
	if err != nil {
		return nil, err
	}
	if err = s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: string(payload),
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "connected")
	return &info, nil
}

func normalizeDirectCredentials(entry CatalogEntry, raw map[string]string) (map[string]string, error) {
	normalized := make(map[string]string, len(raw))
	for key, value := range raw {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			normalized[key] = value
		}
	}
	switch entry.AuthType {
	case "api_key":
		apiKey := connectorFirstNonEmpty(normalized["api_key"], normalized["key"])
		if apiKey == "" {
			return nil, fmt.Errorf("%s API Key 不能为空", entry.Title)
		}
		return map[string]string{"api_key": apiKey}, nil
	case "token":
		token := connectorFirstNonEmpty(normalized["token"], normalized["access_token"], normalized["bearer_token"])
		if token == "" {
			return nil, fmt.Errorf("%s Token 不能为空", entry.Title)
		}
		return map[string]string{"token": token}, nil
	case "none":
		return map[string]string{}, nil
	default:
		return normalized, nil
	}
}

// Disconnect 断开连接器。
func (s *Service) Disconnect(ctx context.Context, ownerUserID string, connectorID string) (*Info, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if err := s.upsertConnection(ctx, connectionRecord{
		OwnerUserID: ownerUserID,
		ConnectorID: entry.ConnectorID,
		State:       "disconnected",
		Credentials: "",
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, ownerUserID, entry, "disconnected")
	return &info, nil
}

func (s *Service) toInfo(ctx context.Context, ownerUserID string, entry CatalogEntry, connectionState string) Info {
	configError := s.oauthConfigError(ctx, ownerUserID, entry.ConnectorID, entry.AuthType, entry.Status)
	return s.toInfoWithConfigError(entry, connectionState, configError)
}

func (s *Service) toInfoWithConfigError(entry CatalogEntry, connectionState string, configError string) Info {
	var configErrorPtr *string
	if configError != "" {
		configErrorPtr = &configError
	}
	return Info{
		ConnectorID:               entry.ConnectorID,
		Name:                      entry.Name,
		Title:                     entry.Title,
		Description:               entry.Description,
		Icon:                      entry.Icon,
		Category:                  entry.Category,
		AuthType:                  entry.AuthType,
		Status:                    entry.Status,
		ConnectionState:           connectionState,
		IsConfigured:              configError == "",
		RequiresExtra:             append([]string{}, entry.RequiresExtra...),
		ConfigError:               configErrorPtr,
		OAuthClientConfigRequired: entry.UserOAuthClient,
		OAuthClientConfigured:     entry.UserOAuthClient && configError == "",
	}
}

func (s *Service) toDetail(ctx context.Context, ownerUserID string, entry CatalogEntry, connectionState string) Detail {
	info := s.toInfo(ctx, ownerUserID, entry, connectionState)
	var oauthClientID *string
	if config, err := s.oauthClientConfig(ctx, ownerUserID, entry); err == nil && config != nil && config.ClientID != "" {
		oauthClientID = &config.ClientID
	}
	return Detail{
		Info:           info,
		AuthURL:        entry.AuthURL,
		TokenURL:       entry.TokenURL,
		Scopes:         append([]string{}, entry.Scopes...),
		MCPServerURL:   entry.MCPServerURL,
		DocsURL:        entry.DocsURL,
		Features:       append([]string{}, entry.Features...),
		FeatureDetails: connectorFeatureDetailsFor(entry),
		OAuthClientID:  oauthClientID,
	}
}

func (s *Service) listConnectionStates(ctx context.Context, ownerUserID string) (map[string]string, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	query := fmt.Sprintf(
		"SELECT connector_id, state FROM connector_connections WHERE owner_user_id = %s",
		s.bind(1),
	)
	rows, err := s.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var connectorID string
		var state string
		if err = rows.Scan(&connectorID, &state); err != nil {
			return nil, err
		}
		result[connectorID] = state
	}
	return result, rows.Err()
}

func (s *Service) connectionState(ctx context.Context, ownerUserID string, connectorID string) (string, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	query := fmt.Sprintf(
		"SELECT state FROM connector_connections WHERE owner_user_id = %s AND connector_id = %s LIMIT 1",
		s.bind(1),
		s.bind(2),
	)
	var state string
	err := s.db.QueryRowContext(ctx, query, ownerUserID, strings.TrimSpace(connectorID)).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return state, nil
}

func (s *Service) insertState(ctx context.Context, row stateRow) error {
	ownerUserID := normalizeConnectorOwnerUserID(ctx, row.OwnerUserID)
	query := fmt.Sprintf(
		"INSERT INTO connector_oauth_states (owner_user_id, state, connector_id, code_verifier, redirect_uri, redirect_kind, shop_domain, extra_json, expires_at) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)",
		s.bind(1),
		s.bind(2),
		s.bind(3),
		s.bind(4),
		s.bind(5),
		s.bind(6),
		s.bind(7),
		s.bind(8),
		s.bind(9),
	)
	_, err := s.db.ExecContext(
		ctx,
		query,
		ownerUserID,
		row.State,
		row.ConnectorID,
		emptyStringAsNil(row.CodeVerifier),
		row.RedirectURI,
		connectorFirstNonEmpty(row.RedirectKind, oauthRedirectKind(row.RedirectURI)),
		emptyStringAsNil(row.ShopDomain),
		emptyStringAsNil(row.ExtraJSON),
		row.ExpiresAt,
	)
	return err
}

func (s *Service) consumeState(ctx context.Context, ownerUserID string, state string) (*stateRow, error) {
	if strings.TrimSpace(state) == "" {
		return nil, nil
	}
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	query := fmt.Sprintf(
		"DELETE FROM connector_oauth_states WHERE owner_user_id = %s AND state = %s RETURNING owner_user_id, state, connector_id, code_verifier, redirect_uri, redirect_kind, shop_domain, extra_json, expires_at",
		s.bind(1),
		s.bind(2),
	)
	var row stateRow
	var codeVerifier sql.NullString
	var redirectKind sql.NullString
	var shopDomain sql.NullString
	var extraJSON sql.NullString
	err := s.db.QueryRowContext(ctx, query, ownerUserID, strings.TrimSpace(state)).Scan(
		&row.OwnerUserID,
		&row.State,
		&row.ConnectorID,
		&codeVerifier,
		&row.RedirectURI,
		&redirectKind,
		&shopDomain,
		&extraJSON,
		&row.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.CodeVerifier = codeVerifier.String
	row.RedirectKind = connectorFirstNonEmpty(redirectKind.String, oauthRedirectKind(row.RedirectURI))
	row.ShopDomain = shopDomain.String
	row.ExtraJSON = extraJSON.String
	return &row, nil
}

func (s *Service) purgeExpiredStates(ctx context.Context) error {
	query := fmt.Sprintf("DELETE FROM connector_oauth_states WHERE expires_at < %s", s.bind(1))
	_, err := s.db.ExecContext(ctx, query, time.Now())
	return err
}

func (s *Service) upsertConnection(ctx context.Context, record connectionRecord) error {
	record.OwnerUserID = normalizeConnectorOwnerUserID(ctx, record.OwnerUserID)
	if err := s.encryptConnectionCredentials(&record); err != nil {
		return err
	}
	if s.driver == "pgx" {
		query := `
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, credentials, credentials_encrypted, auth_type, oauth_state, oauth_state_expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (owner_user_id, connector_id) DO UPDATE SET
    state = EXCLUDED.state,
    credentials = EXCLUDED.credentials,
    credentials_encrypted = EXCLUDED.credentials_encrypted,
    auth_type = EXCLUDED.auth_type,
    oauth_state = EXCLUDED.oauth_state,
    oauth_state_expires_at = EXCLUDED.oauth_state_expires_at,
    updated_at = CURRENT_TIMESTAMP`
		_, err := s.db.ExecContext(
			ctx,
			query,
			record.OwnerUserID,
			record.ConnectorID,
			record.State,
			record.Credentials,
			nullString(record.CredentialsEncrypted),
			record.AuthType,
			nil,
			nil,
		)
		return err
	}
	query := `
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, credentials, credentials_encrypted, auth_type, oauth_state, oauth_state_expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(owner_user_id, connector_id) DO UPDATE SET
    state = excluded.state,
    credentials = excluded.credentials,
    credentials_encrypted = excluded.credentials_encrypted,
    auth_type = excluded.auth_type,
    oauth_state = excluded.oauth_state,
    oauth_state_expires_at = excluded.oauth_state_expires_at,
    updated_at = CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(
		ctx,
		query,
		record.OwnerUserID,
		record.ConnectorID,
		record.State,
		record.Credentials,
		nullString(record.CredentialsEncrypted),
		record.AuthType,
		nil,
		nil,
	)
	return err
}

func (s *Service) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func normalizeConnectorOwnerUserID(ctx context.Context, ownerUserID string) string {
	trimmed := strings.TrimSpace(ownerUserID)
	if trimmed != "" {
		return trimmed
	}
	return authctx.OwnerUserID(ctx)
}

func (s *Service) oauthStateTTL() time.Duration {
	if s.config.ConnectorOAuthStateTTLSeconds <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(s.config.ConnectorOAuthStateTTLSeconds) * time.Second
}

func (s *Service) validateRedirectURI(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("redirect URI 格式不正确")
	}
	for _, allowed := range s.config.ConnectorOAuthAllowedOrigins {
		allowedURL, err := url.Parse(strings.TrimSpace(allowed))
		if err != nil || allowedURL.Scheme == "" || allowedURL.Host == "" {
			continue
		}
		if parsed.Scheme == allowedURL.Scheme && parsed.Host == allowedURL.Host && strings.HasPrefix(parsed.Path, allowedURL.Path) {
			return nil
		}
	}
	return errors.New("redirect URI 不在允许列表中")
}

func oauthRedirectKind(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return oauthRedirectKindWeb
	}
	if strings.EqualFold(parsed.Scheme, "nexus") {
		return oauthRedirectKindDesktop
	}
	return oauthRedirectKindWeb
}

func (s *Service) encryptConnectionCredentials(record *connectionRecord) error {
	if strings.TrimSpace(record.Credentials) == "" {
		record.CredentialsEncrypted = sql.NullString{}
		return nil
	}
	key, err := credentials.DecodeKey(s.config.ConnectorCredentialsKey)
	if err != nil {
		return fmt.Errorf("CONNECTOR_CREDENTIALS_KEY 未配置或无效，无法加密 connector credentials: %w", err)
	}
	encrypted, err := credentials.EncryptPayload(key, []byte(record.Credentials))
	if err != nil {
		return err
	}
	record.Credentials = "__encrypted__"
	record.CredentialsEncrypted = sql.NullString{String: encrypted, Valid: true}
	return nil
}

func (s *Service) connectionCredentialsPayload(record connectionRecord) ([]byte, error) {
	if record.CredentialsEncrypted.Valid && strings.TrimSpace(record.CredentialsEncrypted.String) != "" {
		key, err := credentials.DecodeKey(s.config.ConnectorCredentialsKey)
		if err != nil {
			return nil, err
		}
		return credentials.DecryptPayload(key, record.CredentialsEncrypted.String)
	}
	return []byte(record.Credentials), nil
}

func (s *Service) deviceProvider(entry CatalogEntry) (providers.DeviceProvider, error) {
	providerID := connectorFirstNonEmpty(entry.Provider, entry.ConnectorID)
	provider, err := providers.Get(providerID)
	if err != nil {
		return nil, err
	}
	deviceProvider, ok := provider.(providers.DeviceProvider)
	if !ok {
		return nil, errors.New("连接器不支持 Device Flow")
	}
	return deviceProvider, nil
}

func (s *Service) oauthPublicClientID(ctx context.Context, ownerUserID string, connectorID string, _ string) (string, error) {
	if connectorID == "github" && s.isDesktopMode() {
		return requireOAuthClientID(s.config.ConnectorGitHubClientID, "GitHub")
	}
	clientID, _, err := s.oauthCredentials(ctx, ownerUserID, connectorID)
	if err == nil {
		return clientID, nil
	}
	return "", err
}

func (s *Service) oauthCredentials(ctx context.Context, ownerUserID string, connectorID string) (string, string, error) {
	entry, ok := getConnector(connectorID)
	if ok && entry.UserOAuthClient {
		return s.userOAuthCredentials(ctx, ownerUserID, entry)
	}
	return s.defaultOAuthCredentials(connectorID)
}

func (s *Service) userOAuthCredentials(ctx context.Context, ownerUserID string, entry CatalogEntry) (string, string, error) {
	if strings.TrimSpace(ownerUserID) == "" {
		return "", "", fmt.Errorf("%s OAuth Client ID / Secret 未配置，请先在连接器详情中配置自己的 OAuth 应用", entry.Title)
	}
	store, err := s.oauthClientStore()
	if err != nil {
		return "", "", err
	}
	client, err := store.Get(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return "", "", err
	}
	if client == nil || strings.TrimSpace(client.ClientID) == "" || strings.TrimSpace(client.ClientSecret) == "" {
		return "", "", fmt.Errorf("%s OAuth Client ID / Secret 未配置，请先在连接器详情中配置自己的 OAuth 应用", entry.Title)
	}
	return strings.TrimSpace(client.ClientID), strings.TrimSpace(client.ClientSecret), nil
}

func (s *Service) oauthClientConfig(ctx context.Context, ownerUserID string, entry CatalogEntry) (*OAuthClientConfig, error) {
	if !entry.UserOAuthClient {
		return nil, nil
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return &OAuthClientConfig{ConnectorID: entry.ConnectorID}, nil
	}
	store, err := s.oauthClientStore()
	if err != nil {
		return nil, err
	}
	client, err := store.Get(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return &OAuthClientConfig{ConnectorID: entry.ConnectorID}, nil
	}
	return &OAuthClientConfig{
		ConnectorID: entry.ConnectorID,
		ClientID:    strings.TrimSpace(client.ClientID),
		Configured:  strings.TrimSpace(client.ClientID) != "" && strings.TrimSpace(client.ClientSecret) != "",
	}, nil
}

func (s *Service) oauthClientStore() (*connectorstore.OAuthClientStore, error) {
	key, err := credentials.DecodeKey(s.config.ConnectorCredentialsKey)
	if err != nil {
		return nil, err
	}
	return connectorstore.NewOAuthClientStore(s.db, s.driver, key), nil
}

func (s *Service) defaultOAuthCredentials(connectorID string) (string, string, error) {
	switch connectorID {
	case "github":
		return requireOAuthCredentials(s.config.ConnectorGitHubClientID, s.config.ConnectorGitHubClientSecret, "GitHub")
	case "gmail":
		return requireOAuthCredentials(s.config.ConnectorGoogleClientID, s.config.ConnectorGoogleClientSecret, "Google")
	case "linkedin":
		return requireOAuthCredentials(s.config.ConnectorLinkedInClientID, s.config.ConnectorLinkedInClientSecret, "LinkedIn")
	case "x-twitter":
		return requireOAuthCredentials(s.config.ConnectorTwitterClientID, s.config.ConnectorTwitterClientSecret, "X")
	case "instagram":
		return requireOAuthCredentials(s.config.ConnectorInstagramClientID, s.config.ConnectorInstagramClientSecret, "Instagram")
	case "shopify":
		return requireOAuthCredentials(s.config.ConnectorShopifyClientID, s.config.ConnectorShopifyClientSecret, "Shopify")
	default:
		return "", "", errors.New("当前连接器未配置 OAuth 凭证")
	}
}

func (s *Service) oauthConfigError(ctx context.Context, ownerUserID string, connectorID string, authType string, status string) string {
	if authType != "oauth2" || status != "available" {
		return ""
	}
	if connectorID == "github" && s.isDesktopMode() {
		_, err := s.oauthPublicClientID(ctx, ownerUserID, connectorID, "GitHub")
		if err != nil {
			return err.Error()
		}
		return ""
	}
	_, _, err := s.oauthCredentials(ctx, ownerUserID, connectorID)
	if err != nil {
		return err.Error()
	}
	return ""
}

func (s *Service) listOAuthConfigErrors(ctx context.Context, ownerUserID string) map[string]string {
	result := map[string]string{}
	for _, entry := range connectorCatalog {
		if entry.AuthType != "oauth2" || entry.Status != "available" {
			continue
		}
		var err error
		if entry.ConnectorID == "github" && s.isDesktopMode() {
			_, err = requireOAuthClientID(s.config.ConnectorGitHubClientID, "GitHub")
		} else if entry.UserOAuthClient {
			_, _, err = s.userOAuthCredentials(ctx, ownerUserID, entry)
		} else {
			_, _, err = s.defaultOAuthCredentials(entry.ConnectorID)
		}
		if err != nil {
			result[entry.ConnectorID] = err.Error()
		}
	}
	return result
}

func (s *Service) isDesktopMode() bool {
	return strings.EqualFold(strings.TrimSpace(s.config.AppMode), "desktop")
}

func getConnector(connectorID string) (CatalogEntry, bool) {
	for _, entry := range connectorCatalog {
		if entry.ConnectorID == strings.TrimSpace(connectorID) {
			return entry, true
		}
	}
	return CatalogEntry{}, false
}

func connectorMatches(entry CatalogEntry, query string) bool {
	fields := []string{
		strings.ToLower(entry.ConnectorID),
		strings.ToLower(entry.Name),
		strings.ToLower(entry.Title),
		strings.ToLower(entry.Description),
		strings.ToLower(strings.Join(entry.Features, " ")),
	}
	for _, field := range fields {
		if strings.Contains(field, query) {
			return true
		}
	}
	return false
}

func nullString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func emptyStringAsNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func requireOAuthCredentials(clientID string, clientSecret string, label string) (string, string, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return "", "", fmt.Errorf("%s OAuth Client ID / Secret 未配置", label)
	}
	return clientID, clientSecret, nil
}

func requireOAuthClientID(clientID string, label string) (string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", fmt.Errorf("%s OAuth Client ID 未配置", label)
	}
	return strings.TrimSpace(clientID), nil
}

func deviceAuthStatusFromError(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "authorization_pending"):
		return deviceAuthStatusPending
	case strings.Contains(message, "slow_down"):
		return deviceAuthStatusSlowDown
	case strings.Contains(message, "expired_token"), strings.Contains(message, "token_expired"):
		return deviceAuthStatusExpired
	case strings.Contains(message, "access_denied"):
		return deviceAuthStatusDenied
	default:
		return ""
	}
}

func deviceAuthMessage(status string) string {
	switch status {
	case deviceAuthStatusPending:
		return "等待 GitHub 授权确认"
	case deviceAuthStatusSlowDown:
		return "GitHub 要求降低轮询频率"
	case deviceAuthStatusExpired:
		return "GitHub 授权码已过期"
	case deviceAuthStatusDenied:
		return "用户取消了 GitHub 授权"
	default:
		return ""
	}
}

func friendlyDeviceAuthError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "device_flow_disabled") {
		return errors.New("GitHub OAuth App 未启用 Device Flow，请在 GitHub Developer settings 的 OAuth App 设置中启用 Device Flow 后重试")
	}
	return err
}

func normalizeOAuthPayload(payload []byte) string {
	normalized, err := credentialMapFromPayload(payload)
	if err != nil {
		return string(payload)
	}
	addCredentialExpiresAt(normalized, "expires_in", "expires_at")
	addCredentialExpiresAt(normalized, "refresh_expires_in", "refresh_expires_at")
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return string(payload)
	}
	return string(encoded)
}

func credentialMapFromPayload(payload []byte) (map[string]string, error) {
	if json.Valid(payload) {
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			return nil, err
		}
		if data, ok := raw["data"].(map[string]any); ok {
			raw = data
		}
		normalized := map[string]string{}
		for key, value := range raw {
			if key == "" || value == nil {
				continue
			}
			normalized[key] = credentialScalarString(value)
		}
		return normalized, nil
	}
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return nil, err
	}
	normalized := map[string]string{}
	for key, value := range values {
		normalized[key] = strings.Join(value, ",")
	}
	return normalized, nil
}

func credentialScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func addCredentialExpiresAt(credentials map[string]string, durationKey string, targetKey string) {
	if strings.TrimSpace(credentials[targetKey]) != "" {
		return
	}
	expiresInRaw := strings.TrimSpace(credentials[durationKey])
	if expiresInRaw == "" {
		return
	}
	expiresIn, err := strconv.ParseFloat(expiresInRaw, 64)
	if err != nil || expiresIn <= 0 {
		return
	}
	credentials[targetKey] = strconv.FormatInt(time.Now().Add(time.Duration(expiresIn)*time.Second).Unix(), 10)
}

func mergeCredentialExtras(credentials string, extra map[string]string) string {
	if len(extra) == 0 || !json.Valid([]byte(credentials)) {
		return credentials
	}
	parsed := map[string]string{}
	if err := json.Unmarshal([]byte(credentials), &parsed); err != nil {
		return credentials
	}
	for key, value := range extra {
		if strings.TrimSpace(value) == "" {
			continue
		}
		parsed[key] = value
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return credentials
	}
	return string(encoded)
}

func normalizeExtras(extras map[string]string) map[string]string {
	normalized := map[string]string{}
	for key, value := range extras {
		normalized[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return normalized
}

func (row stateRow) extra() (map[string]string, error) {
	result := map[string]string{}
	if strings.TrimSpace(row.ExtraJSON) != "" {
		if err := json.Unmarshal([]byte(row.ExtraJSON), &result); err != nil {
			return nil, errors.New("OAuth state extra 参数格式不正确")
		}
	}
	if result["shop"] == "" && strings.TrimSpace(row.ShopDomain) != "" {
		result["shop"] = row.ShopDomain
	}
	return normalizeExtras(result), nil
}

func connectorFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
