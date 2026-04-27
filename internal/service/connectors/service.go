package connectors

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/service/connectors/providers"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// Info 表示连接器列表项。
type Info struct {
	ConnectorID     string   `json:"connector_id"`
	Name            string   `json:"name"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Icon            string   `json:"icon"`
	Category        string   `json:"category"`
	AuthType        string   `json:"auth_type"`
	Status          string   `json:"status"`
	ConnectionState string   `json:"connection_state"`
	IsConfigured    bool     `json:"is_configured"`
	RequiresExtra   []string `json:"requires_extra,omitempty"`
	ConfigError     *string  `json:"config_error,omitempty"`
}

// Detail 表示连接器详情。
type Detail struct {
	Info
	AuthURL      string   `json:"auth_url,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	Scopes       []string `json:"scopes"`
	MCPServerURL string   `json:"mcp_server_url,omitempty"`
	DocsURL      string   `json:"docs_url,omitempty"`
	Features     []string `json:"features"`
}

// OAuthClientView 是前端可见的 OAuth 应用配置摘要，不包含 secret 明文。
type OAuthClientView struct {
	ConnectorID     string    `json:"connector_id"`
	ClientID        string    `json:"client_id"`
	HasClientSecret bool      `json:"has_client_secret"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ConnectionSnapshot 是 Agent MCP 调用 connector REST API 所需的连接快照。
type ConnectionSnapshot struct {
	ConnectorID string            `json:"connector_id"`
	AuthType    string            `json:"auth_type"`
	APIBaseURL  string            `json:"api_base_url"`
	AccessToken string            `json:"-"`
	ShopDomain  string            `json:"shop_domain,omitempty"`
	Extra       map[string]string `json:"extra,omitempty"`
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

type connectionRecord struct {
	ConnectorID          string
	State                string
	Credentials          string
	CredentialsEncrypted sql.NullString
	AuthType             string
	OAuthState           sql.NullString
	OAuthStateExpiresAt  sql.NullTime
}

type stateRow struct {
	State        string
	ConnectorID  string
	CodeVerifier string
	RedirectURI  string
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
	clients    *oauthClientStore
}

// NewService 创建连接器服务。
func NewService(cfg config.Config, db *sql.DB) *Service {
	driver := storage.NormalizeSQLDriver(cfg.DatabaseDriver)
	var clients *oauthClientStore
	if key, err := decodeCredentialKey(cfg.ConnectorCredentialsKey); err == nil {
		clients = newOAuthClientStore(db, driver, key)
	} else if strings.TrimSpace(cfg.ConnectorCredentialsKey) != "" {
		fmt.Fprintln(os.Stderr, "WARNING: CONNECTOR_CREDENTIALS_KEY 解析失败，OAuth client DB 配置将不可用")
	}
	return &Service{
		config:  cfg,
		db:      db,
		driver:  driver,
		clients: clients,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// ListConnectors 列出连接器目录。
func (s *Service) ListConnectors(ctx context.Context, ownerUserID string, query string, category string, status string) ([]Info, error) {
	states, err := s.listConnectionStates(ctx)
	if err != nil {
		return nil, err
	}
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
		items = append(items, s.toInfo(ctx, ownerUserID, entry, connectorFirstNonEmpty(states[entry.ConnectorID], "disconnected")))
	}
	return items, nil
}

// GetConnectorDetail 返回单个连接器详情。
func (s *Service) GetConnectorDetail(ctx context.Context, ownerUserID string, connectorID string) (*Detail, error) {
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("connector not found")
	}
	states, err := s.listConnectionStates(ctx)
	if err != nil {
		return nil, err
	}
	detail := s.toDetail(ctx, ownerUserID, entry, connectorFirstNonEmpty(states[entry.ConnectorID], "disconnected"))
	return &detail, nil
}

// GetConnectedCount 返回已连接数量。
func (s *Service) GetConnectedCount(ctx context.Context) (int, error) {
	query := "SELECT COUNT(1) FROM connector_connections WHERE state = 'connected'"
	var count int
	if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ListActiveConnections 列出已连接 connector，暂时保留 ownerUserID 签名供后续 user scope 使用。
func (s *Service) ListActiveConnections(ctx context.Context, ownerUserID string) ([]ConnectionSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT connector_id FROM connector_connections WHERE state = 'connected'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ConnectionSnapshot{}
	for rows.Next() {
		var connectorID string
		if err = rows.Scan(&connectorID); err != nil {
			return nil, err
		}
		item, err := s.LoadActiveConnection(ctx, ownerUserID, connectorID)
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
func (s *Service) LoadActiveConnection(ctx context.Context, ownerUserID, connectorID string) (*ConnectionSnapshot, error) {
	// TODO(connector-user-scope): 追加 owner_user_id 过滤 —— 见 Phase 3。
	query := fmt.Sprintf(
		"SELECT connector_id, credentials, credentials_encrypted, auth_type FROM connector_connections WHERE connector_id = %s AND state = 'connected'",
		s.bind(1),
	)
	var record connectionRecord
	err := s.db.QueryRowContext(ctx, query, strings.TrimSpace(connectorID)).Scan(
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
	entry, ok := getConnector(record.ConnectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	payload, err := s.connectionCredentialsPayload(record)
	if err != nil {
		return nil, err
	}
	parsed := map[string]string{}
	if err = json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}
	token := connectorFirstNonEmpty(parsed["access_token"], parsed["token"], parsed["bearer_token"])
	if token == "" {
		return nil, errors.New("connector 未获取到 access token")
	}
	delete(parsed, "access_token")
	delete(parsed, "token")
	delete(parsed, "bearer_token")
	shop := connectorFirstNonEmpty(parsed["shop"], parsed["shop_domain"])
	return &ConnectionSnapshot{
		ConnectorID: record.ConnectorID,
		AuthType:    record.AuthType,
		APIBaseURL:  entry.APIBaseURL,
		AccessToken: token,
		ShopDomain:  shop,
		Extra:       parsed,
	}, nil
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
		State:        state,
		ConnectorID:  entry.ConnectorID,
		CodeVerifier: verifier,
		RedirectURI:  resolvedRedirectURI,
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
	stateValue := strings.TrimSpace(request.State)
	state, err := s.consumeState(ctx, stateValue)
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

// Connect 使用显式凭证直接连接。
func (s *Service) Connect(ctx context.Context, connectorID string, credentials map[string]string) (*Info, error) {
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
	payload, err := json.Marshal(credentials)
	if err != nil {
		return nil, err
	}
	if err = s.upsertConnection(ctx, connectionRecord{
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: string(payload),
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, "", entry, "connected")
	return &info, nil
}

// Disconnect 断开连接器。
func (s *Service) Disconnect(ctx context.Context, connectorID string) (*Info, error) {
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if err := s.upsertConnection(ctx, connectionRecord{
		ConnectorID: entry.ConnectorID,
		State:       "disconnected",
		Credentials: "",
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(ctx, "", entry, "disconnected")
	return &info, nil
}

func (s *Service) toInfo(ctx context.Context, ownerUserID string, entry CatalogEntry, connectionState string) Info {
	configError := s.oauthConfigError(ctx, ownerUserID, entry.ConnectorID, entry.AuthType, entry.Status)
	var configErrorPtr *string
	if configError != "" {
		configErrorPtr = &configError
	}
	return Info{
		ConnectorID:     entry.ConnectorID,
		Name:            entry.Name,
		Title:           entry.Title,
		Description:     entry.Description,
		Icon:            entry.Icon,
		Category:        entry.Category,
		AuthType:        entry.AuthType,
		Status:          entry.Status,
		ConnectionState: connectionState,
		IsConfigured:    configError == "",
		RequiresExtra:   append([]string{}, entry.RequiresExtra...),
		ConfigError:     configErrorPtr,
	}
}

func (s *Service) toDetail(ctx context.Context, ownerUserID string, entry CatalogEntry, connectionState string) Detail {
	info := s.toInfo(ctx, ownerUserID, entry, connectionState)
	return Detail{
		Info:         info,
		AuthURL:      entry.AuthURL,
		TokenURL:     entry.TokenURL,
		Scopes:       append([]string{}, entry.Scopes...),
		MCPServerURL: entry.MCPServerURL,
		DocsURL:      entry.DocsURL,
		Features:     append([]string{}, entry.Features...),
	}
}

func (s *Service) listConnectionStates(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT connector_id, state FROM connector_connections")
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

func (s *Service) insertState(ctx context.Context, row stateRow) error {
	query := fmt.Sprintf(
		"INSERT INTO connector_oauth_states (state, connector_id, code_verifier, redirect_uri, shop_domain, extra_json, expires_at) VALUES (%s, %s, %s, %s, %s, %s, %s)",
		s.bind(1),
		s.bind(2),
		s.bind(3),
		s.bind(4),
		s.bind(5),
		s.bind(6),
		s.bind(7),
	)
	_, err := s.db.ExecContext(
		ctx,
		query,
		row.State,
		row.ConnectorID,
		emptyStringAsNil(row.CodeVerifier),
		row.RedirectURI,
		emptyStringAsNil(row.ShopDomain),
		emptyStringAsNil(row.ExtraJSON),
		row.ExpiresAt,
	)
	return err
}

func (s *Service) consumeState(ctx context.Context, state string) (*stateRow, error) {
	if strings.TrimSpace(state) == "" {
		return nil, nil
	}
	query := fmt.Sprintf(
		"DELETE FROM connector_oauth_states WHERE state = %s RETURNING state, connector_id, code_verifier, redirect_uri, shop_domain, extra_json, expires_at",
		s.bind(1),
	)
	var row stateRow
	var codeVerifier sql.NullString
	var shopDomain sql.NullString
	var extraJSON sql.NullString
	err := s.db.QueryRowContext(ctx, query, strings.TrimSpace(state)).Scan(
		&row.State,
		&row.ConnectorID,
		&codeVerifier,
		&row.RedirectURI,
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
	if err := s.encryptConnectionCredentials(&record); err != nil {
		return err
	}
	if s.driver == "pgx" {
		query := `
INSERT INTO connector_connections (
    connector_id, state, credentials, credentials_encrypted, auth_type, oauth_state, oauth_state_expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (connector_id) DO UPDATE SET
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
    connector_id, state, credentials, credentials_encrypted, auth_type, oauth_state, oauth_state_expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(connector_id) DO UPDATE SET
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

func (s *Service) encryptConnectionCredentials(record *connectionRecord) error {
	if strings.TrimSpace(record.Credentials) == "" {
		record.CredentialsEncrypted = sql.NullString{}
		return nil
	}
	key, err := decodeCredentialKey(s.config.ConnectorCredentialsKey)
	if err != nil {
		if s.config.Debug {
			fmt.Fprintln(os.Stderr, "WARNING: CONNECTOR_CREDENTIALS_KEY 未配置，connector credentials 将以明文保存")
			return nil
		}
		return err
	}
	encrypted, err := encryptCredentialPayload(key, []byte(record.Credentials))
	if err != nil {
		return err
	}
	record.Credentials = "__encrypted__"
	record.CredentialsEncrypted = sql.NullString{String: encrypted, Valid: true}
	return nil
}

func (s *Service) connectionCredentialsPayload(record connectionRecord) ([]byte, error) {
	if record.CredentialsEncrypted.Valid && strings.TrimSpace(record.CredentialsEncrypted.String) != "" {
		key, err := decodeCredentialKey(s.config.ConnectorCredentialsKey)
		if err != nil {
			return nil, err
		}
		return decryptCredentialPayload(key, record.CredentialsEncrypted.String)
	}
	return []byte(record.Credentials), nil
}

func (s *Service) GetOAuthClient(ctx context.Context, ownerUserID, connectorID string) (*OAuthClientView, error) {
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.AuthType != "oauth2" {
		return nil, errors.New("连接器不支持 OAuth")
	}
	if s.clients == nil {
		return nil, nil
	}
	record, err := s.clients.Get(ctx, ownerUserID, entry.ConnectorID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	return &OAuthClientView{
		ConnectorID:     record.ConnectorID,
		ClientID:        record.ClientID,
		HasClientSecret: strings.TrimSpace(record.ClientSecret) != "",
		UpdatedAt:       record.UpdatedAt,
	}, nil
}

func (s *Service) UpsertOAuthClient(ctx context.Context, ownerUserID, connectorID, clientID, clientSecret string) error {
	entry, ok := getConnector(connectorID)
	if !ok {
		return errors.New("未知连接器")
	}
	if entry.AuthType != "oauth2" {
		return errors.New("连接器不支持 OAuth")
	}
	if entry.Status != "available" {
		return errors.New("连接器暂不可用")
	}
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return errors.New("OAuth Client ID / Secret 不能为空")
	}
	if s.clients == nil {
		return errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 OAuth 应用凭据")
	}
	return s.clients.Upsert(ctx, OAuthClient{
		OwnerUserID:  ownerUserID,
		ConnectorID:  entry.ConnectorID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
}

func (s *Service) DeleteOAuthClient(ctx context.Context, ownerUserID, connectorID string) error {
	entry, ok := getConnector(connectorID)
	if !ok {
		return errors.New("未知连接器")
	}
	if entry.AuthType != "oauth2" {
		return errors.New("连接器不支持 OAuth")
	}
	if s.clients == nil {
		return nil
	}
	return s.clients.Delete(ctx, ownerUserID, entry.ConnectorID)
}

func (s *Service) oauthCredentials(ctx context.Context, ownerUserID string, connectorID string) (string, string, error) {
	if s.clients != nil {
		record, err := s.clients.Get(ctx, ownerUserID, connectorID)
		if err != nil {
			return "", "", err
		}
		if record != nil {
			return requireOAuthCredentials(record.ClientID, record.ClientSecret, connectorID)
		}
	}
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
	_, _, err := s.oauthCredentials(ctx, ownerUserID, connectorID)
	if err != nil {
		return err.Error()
	}
	return ""
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

func normalizeOAuthPayload(payload []byte) string {
	if json.Valid(payload) {
		return string(payload)
	}
	values, err := url.ParseQuery(string(payload))
	if err != nil {
		return string(payload)
	}
	normalized := map[string]string{}
	for key, value := range values {
		normalized[key] = strings.Join(value, ",")
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return string(payload)
	}
	return string(encoded)
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

func decodeCredentialKey(raw string) ([]byte, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(key) != 32 {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 必须是 32 字节 base64")
	}
	return key, nil
}

func encryptCredentialPayload(key []byte, payload []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, payload, nil)
	encoded := append(nonce, ciphertext...)
	return "v1:" + base64.StdEncoding.EncodeToString(encoded), nil
}

func decryptCredentialPayload(key []byte, payload string) ([]byte, error) {
	encoded := strings.TrimPrefix(strings.TrimSpace(payload), "v1:")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("connector credentials payload 格式不正确")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func connectorFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
