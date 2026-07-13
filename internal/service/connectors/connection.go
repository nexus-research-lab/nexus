package connectors

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	connectordomain "github.com/nexus-research-lab/nexus/internal/connectors"
)

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
