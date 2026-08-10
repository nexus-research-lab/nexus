// INPUT: owner 与 Connector ID。
// OUTPUT: 单次 SQL 快照中的配置版本、连接状态和脱敏凭据存在性。
// POS: Connector 对话配置 plan/apply CAS 与写后核验的读取真相源。
package connectors

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// GetConfigurationState 返回单个 Connector 的持久配置状态和 CAS 版本。
func (s *Service) GetConfigurationState(
	ctx context.Context,
	ownerUserID string,
	connectorID string,
) (*ConfigurationState, error) {
	ownerUserID = normalizeConnectorOwnerUserID(ctx, ownerUserID)
	connectorID = strings.TrimSpace(connectorID)
	entry, ok := getConnector(connectorID)
	if !ok {
		return nil, errors.New("未知连接器")
	}

	query := `
WITH target(owner_user_id, connector_id) AS (VALUES (?, ?))
SELECT
    COALESCE(version.version, 1),
    connection.connector_id,
    connection.state,
    connection.auth_type,
    CASE WHEN COALESCE(connection.credentials_encrypted, connection.credentials, '') <> '' THEN 1 ELSE 0 END,
    oauth_client.connector_id,
    oauth_client.client_id,
    CASE WHEN COALESCE(oauth_client.client_secret_encrypted, '') <> '' THEN 1 ELSE 0 END
FROM target
LEFT JOIN connector_configuration_versions AS version
    ON version.owner_user_id = target.owner_user_id
   AND version.connector_id = target.connector_id
LEFT JOIN connector_connections AS connection
    ON connection.owner_user_id = target.owner_user_id
   AND connection.connector_id = target.connector_id
LEFT JOIN connector_oauth_clients AS oauth_client
    ON oauth_client.owner_user_id = target.owner_user_id
   AND oauth_client.connector_id = target.connector_id`
	if s.driver == "pgx" {
		query = `
WITH target(owner_user_id, connector_id) AS (VALUES ($1::text, $2::text))
SELECT
    COALESCE(version.version, 1),
    connection.connector_id,
    connection.state,
    connection.auth_type,
    CASE WHEN COALESCE(connection.credentials_encrypted, connection.credentials, '') <> '' THEN TRUE ELSE FALSE END,
    oauth_client.connector_id,
    oauth_client.client_id,
    CASE WHEN COALESCE(oauth_client.client_secret_encrypted, '') <> '' THEN TRUE ELSE FALSE END
FROM target
LEFT JOIN connector_configuration_versions AS version
    ON version.owner_user_id = target.owner_user_id
   AND version.connector_id = target.connector_id
LEFT JOIN connector_connections AS connection
    ON connection.owner_user_id = target.owner_user_id
   AND connection.connector_id = target.connector_id
LEFT JOIN connector_oauth_clients AS oauth_client
    ON oauth_client.owner_user_id = target.owner_user_id
   AND oauth_client.connector_id = target.connector_id`
	}

	var connectionID sql.NullString
	var connectionState sql.NullString
	var connectionAuthType sql.NullString
	var connectionConfigured bool
	var oauthClientConnectorID sql.NullString
	var oauthClientID sql.NullString
	var oauthClientConfigured bool
	state := &ConfigurationState{ConnectorID: connectorID}
	if err := s.db.QueryRowContext(ctx, query, ownerUserID, connectorID).Scan(
		&state.ConfigurationVersion,
		&connectionID,
		&connectionState,
		&connectionAuthType,
		&connectionConfigured,
		&oauthClientConnectorID,
		&oauthClientID,
		&oauthClientConfigured,
	); err != nil {
		return nil, err
	}
	state.ConnectionExists = connectionID.Valid
	state.ConnectionState = connectionState.String
	state.ConnectionAuthType = connectionAuthType.String
	if !state.ConnectionExists {
		state.ConnectionState = "disconnected"
		state.ConnectionAuthType = entry.AuthType
	}
	state.ConnectionConfigured = connectionConfigured
	state.OAuthClientExists = oauthClientConnectorID.Valid
	state.OAuthClientID = oauthClientID.String
	state.OAuthClientConfigured = oauthClientConfigured
	return state, nil
}
