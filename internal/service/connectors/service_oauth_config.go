package connectors

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	connectorstore "github.com/nexus-research-lab/nexus/internal/storage/connectors"
)

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
	if client == nil {
		return "", "", fmt.Errorf("%s OAuth Client ID / Secret 未配置，请先在连接器详情中配置自己的 OAuth 应用", entry.Title)
	}
	clientID := strings.TrimSpace(client.ClientID)
	clientSecret := strings.TrimSpace(client.ClientSecret)
	if clientID == "" || clientSecret == "" {
		return "", "", fmt.Errorf("%s OAuth Client ID / Secret 未配置，请先在连接器详情中配置自己的 OAuth 应用", entry.Title)
	}
	return clientID, clientSecret, nil
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
	clientID := strings.TrimSpace(client.ClientID)
	clientSecret := strings.TrimSpace(client.ClientSecret)
	return &OAuthClientConfig{
		ConnectorID: entry.ConnectorID,
		ClientID:    clientID,
		Configured:  clientID != "" && clientSecret != "",
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

func requireOAuthCredentials(clientID string, clientSecret string, label string) (string, string, error) {
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if clientID == "" || clientSecret == "" {
		return "", "", fmt.Errorf("%s OAuth Client ID / Secret 未配置", label)
	}
	return clientID, clientSecret, nil
}

func requireOAuthClientID(clientID string, label string) (string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", fmt.Errorf("%s OAuth Client ID 未配置", label)
	}
	return clientID, nil
}
