package connectors

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/providers"
)

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

// GetCategories 返回连接器分类映射。
func (s *Service) GetCategories() map[string]string {
	return maps.Clone(categoryLabels)
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
		return slices.Clone(entry.RequiresExtra)
	}
	return slices.Clone(provider.RequiredExtraKeys())
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
		RequiresExtra:             slices.Clone(entry.RequiresExtra),
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
		Scopes:         slices.Clone(entry.Scopes),
		MCPServerURL:   entry.MCPServerURL,
		DocsURL:        entry.DocsURL,
		Features:       slices.Clone(entry.Features),
		FeatureDetails: connectorFeatureDetailsFor(entry),
		OAuthClientID:  oauthClientID,
	}
}

func getConnector(connectorID string) (CatalogEntry, bool) {
	normalized := strings.TrimSpace(connectorID)
	index := slices.IndexFunc(connectorCatalog, func(entry CatalogEntry) bool {
		return entry.ConnectorID == normalized
	})
	if index < 0 {
		return CatalogEntry{}, false
	}
	return connectorCatalog[index], true
}

func connectorMatches(entry CatalogEntry, query string) bool {
	fields := []string{
		strings.ToLower(entry.ConnectorID),
		strings.ToLower(entry.Name),
		strings.ToLower(entry.Title),
		strings.ToLower(entry.Description),
		strings.ToLower(strings.Join(entry.Features, " ")),
	}
	return slices.ContainsFunc(fields, func(field string) bool {
		return strings.Contains(field, query)
	})
}
