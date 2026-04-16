// =====================================================
// @File   ：service.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package connectors

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Info 表示连接器列表项。
type Info struct {
	ConnectorID     string  `json:"connector_id"`
	Name            string  `json:"name"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Icon            string  `json:"icon"`
	Category        string  `json:"category"`
	AuthType        string  `json:"auth_type"`
	Status          string  `json:"status"`
	ConnectionState string  `json:"connection_state"`
	IsConfigured    bool    `json:"is_configured"`
	ConfigError     *string `json:"config_error,omitempty"`
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
	ConnectorID         string
	State               string
	Credentials         string
	AuthType            string
	OAuthState          sql.NullString
	OAuthStateExpiresAt sql.NullTime
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
	return &Service{
		config: cfg,
		db:     db,
		driver: protocol.NormalizeSQLDriver(cfg.DatabaseDriver),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// ListConnectors 列出连接器目录。
func (s *Service) ListConnectors(ctx context.Context, query string, category string, status string) ([]Info, error) {
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
		items = append(items, s.toInfo(entry, connectorFirstNonEmpty(states[entry.ConnectorID], "disconnected")))
	}
	return items, nil
}

// GetConnectorDetail 返回单个连接器详情。
func (s *Service) GetConnectorDetail(ctx context.Context, connectorID string) (*Detail, error) {
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("connector not found")
	}
	states, err := s.listConnectionStates(ctx)
	if err != nil {
		return nil, err
	}
	detail := s.toDetail(entry, connectorFirstNonEmpty(states[entry.ConnectorID], "disconnected"))
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

// GetCategories 返回连接器分类映射。
func (s *Service) GetCategories() map[string]string {
	result := make(map[string]string, len(categoryLabels))
	for key, value := range categoryLabels {
		result[key] = value
	}
	return result
}

// GetAuthURL 生成 OAuth 授权地址。
func (s *Service) GetAuthURL(ctx context.Context, connectorID string, redirectURI string) (*AuthURLResult, error) {
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if entry.Status != "available" {
		return nil, errors.New("连接器暂不可用")
	}
	clientID, _, configErr := s.oauthCredentials(entry.ConnectorID)
	if configErr != nil {
		return nil, configErr
	}
	resolvedRedirectURI := strings.TrimSpace(redirectURI)
	if resolvedRedirectURI == "" {
		resolvedRedirectURI = s.config.ConnectorOAuthRedirectURI
	}
	if strings.Contains(entry.AuthURL, "{shop}") {
		return nil, errors.New("Shopify 需要店铺域名，当前 Go 版暂未支持")
	}
	state, err := randomStateToken()
	if err != nil {
		return nil, err
	}
	if err = s.upsertConnection(ctx, connectionRecord{
		ConnectorID:         entry.ConnectorID,
		State:               "disconnected",
		Credentials:         "",
		AuthType:            entry.AuthType,
		OAuthState:          sql.NullString{String: state, Valid: true},
		OAuthStateExpiresAt: sql.NullTime{Time: time.Now().Add(10 * time.Minute), Valid: true},
	}); err != nil {
		return nil, err
	}
	authURL, err := url.Parse(entry.AuthURL)
	if err != nil {
		return nil, err
	}
	params := authURL.Query()
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", resolvedRedirectURI)
	params.Set("state", state)
	if len(entry.Scopes) > 0 {
		params.Set("scope", strings.Join(entry.Scopes, " "))
	}
	if entry.ConnectorID == "gmail" {
		params.Set("access_type", "offline")
		params.Set("prompt", "consent")
	}
	authURL.RawQuery = params.Encode()
	return &AuthURLResult{
		AuthURL: authURL.String(),
		State:   state,
	}, nil
}

// CompleteOAuthCallback 完成 OAuth token 交换。
func (s *Service) CompleteOAuthCallback(ctx context.Context, request OAuthCallbackRequest) (*Info, error) {
	connection, err := s.getConnectionByOAuthState(ctx, strings.TrimSpace(request.State))
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, errors.New("OAuth state 无效或已过期")
	}
	if connection.OAuthStateExpiresAt.Valid && connection.OAuthStateExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("OAuth state 已过期，请重新发起授权")
	}
	entry, ok := getConnector(connection.ConnectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}
	if strings.Contains(entry.TokenURL, "{shop}") {
		return nil, errors.New("Shopify OAuth token exchange 当前未实现")
	}
	clientID, clientSecret, configErr := s.oauthCredentials(entry.ConnectorID)
	if configErr != nil {
		return nil, configErr
	}
	redirectURI := strings.TrimSpace(request.RedirectURI)
	if redirectURI == "" {
		redirectURI = s.config.ConnectorOAuthRedirectURI
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", strings.TrimSpace(request.Code))
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, entry.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")
	response, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("OAuth token exchange 失败: %s", strings.TrimSpace(string(payload)))
	}
	credentials := normalizeOAuthPayload(payload)
	if err = s.upsertConnection(ctx, connectionRecord{
		ConnectorID: entry.ConnectorID,
		State:       "connected",
		Credentials: credentials,
		AuthType:    entry.AuthType,
	}); err != nil {
		return nil, err
	}
	info := s.toInfo(entry, "connected")
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
	info := s.toInfo(entry, "connected")
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
	info := s.toInfo(entry, "disconnected")
	return &info, nil
}

func (s *Service) toInfo(entry CatalogEntry, connectionState string) Info {
	configError := s.oauthConfigError(entry.ConnectorID, entry.AuthType, entry.Status)
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
		ConfigError:     configErrorPtr,
	}
}

func (s *Service) toDetail(entry CatalogEntry, connectionState string) Detail {
	info := s.toInfo(entry, connectionState)
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

func (s *Service) getConnectionByOAuthState(ctx context.Context, state string) (*connectionRecord, error) {
	if strings.TrimSpace(state) == "" {
		return nil, nil
	}
	query := fmt.Sprintf(
		"SELECT connector_id, state, credentials, auth_type, oauth_state, oauth_state_expires_at FROM connector_connections WHERE oauth_state = %s",
		s.bind(1),
	)
	row := s.db.QueryRowContext(ctx, query, state)
	return scanConnection(row)
}

func (s *Service) upsertConnection(ctx context.Context, record connectionRecord) error {
	if s.driver == "pgx" {
		query := `
INSERT INTO connector_connections (
    connector_id, state, credentials, auth_type, oauth_state, oauth_state_expires_at
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (connector_id) DO UPDATE SET
    state = EXCLUDED.state,
    credentials = EXCLUDED.credentials,
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
			record.AuthType,
			nullString(record.OAuthState),
			nullTime(record.OAuthStateExpiresAt),
		)
		return err
	}
	query := `
INSERT INTO connector_connections (
    connector_id, state, credentials, auth_type, oauth_state, oauth_state_expires_at
) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(connector_id) DO UPDATE SET
    state = excluded.state,
    credentials = excluded.credentials,
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
		record.AuthType,
		nullString(record.OAuthState),
		nullTime(record.OAuthStateExpiresAt),
	)
	return err
}

func (s *Service) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (s *Service) oauthCredentials(connectorID string) (string, string, error) {
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

func (s *Service) oauthConfigError(connectorID string, authType string, status string) string {
	if authType != "oauth2" || status != "available" {
		return ""
	}
	_, _, err := s.oauthCredentials(connectorID)
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

func scanConnection(scanner interface{ Scan(dest ...any) error }) (*connectionRecord, error) {
	var record connectionRecord
	err := scanner.Scan(
		&record.ConnectorID,
		&record.State,
		&record.Credentials,
		&record.AuthType,
		&record.OAuthState,
		&record.OAuthStateExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func nullString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func nullTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}

func requireOAuthCredentials(clientID string, clientSecret string, label string) (string, string, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return "", "", fmt.Errorf("%s OAuth Client ID / Secret 未配置", label)
	}
	return clientID, clientSecret, nil
}

func randomStateToken() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
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

func connectorFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
