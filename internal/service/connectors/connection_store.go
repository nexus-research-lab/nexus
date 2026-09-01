// INPUT: Connector 配置记录、可选 availability 与期望配置版本。
// OUTPUT: 加密凭据及原子保留/更新 availability 的 owner 级连接记录。
// POS: 所有 Connector 持久写入的唯一 SQL 装配边界。
package connectors

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func (s *Service) upsertConnection(ctx context.Context, record connectionRecord) error {
	_, err := s.upsertConnectionAtVersion(ctx, record, nil)
	return err
}

func (s *Service) upsertConnectionAtVersion(
	ctx context.Context,
	record connectionRecord,
	expectedVersion *int64,
) (int64, error) {
	record.OwnerUserID = normalizeConnectorOwnerUserID(ctx, record.OwnerUserID)
	if err := s.encryptConnectionCredentials(&record); err != nil {
		return 0, err
	}
	return s.mutateConnector(
		ctx,
		record.OwnerUserID,
		record.ConnectorID,
		expectedVersion,
		func(tx *sql.Tx) error {
			return s.writeConnection(ctx, tx, record)
		},
	)
}

type connectorExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Service) writeConnection(
	ctx context.Context,
	executor connectorExecer,
	record connectionRecord,
) error {
	if s.driver == "pgx" {
		query := `
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, credentials, credentials_encrypted, auth_type,
    oauth_state, oauth_state_expires_at, enabled
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, TRUE))
ON CONFLICT (owner_user_id, connector_id) DO UPDATE SET
    state = EXCLUDED.state,
    credentials = EXCLUDED.credentials,
    credentials_encrypted = EXCLUDED.credentials_encrypted,
    auth_type = EXCLUDED.auth_type,
    oauth_state = EXCLUDED.oauth_state,
    oauth_state_expires_at = EXCLUDED.oauth_state_expires_at,
    enabled = COALESCE($9, connector_connections.enabled),
    updated_at = CURRENT_TIMESTAMP`
		_, err := executor.ExecContext(
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
			nullBool(record.AvailabilityEnabled),
		)
		return err
	}
	query := `
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, credentials, credentials_encrypted, auth_type,
    oauth_state, oauth_state_expires_at, enabled
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, TRUE))
ON CONFLICT(owner_user_id, connector_id) DO UPDATE SET
    state = excluded.state,
    credentials = excluded.credentials,
    credentials_encrypted = excluded.credentials_encrypted,
    auth_type = excluded.auth_type,
    oauth_state = excluded.oauth_state,
    oauth_state_expires_at = excluded.oauth_state_expires_at,
    enabled = COALESCE(?, connector_connections.enabled),
    updated_at = CURRENT_TIMESTAMP`
	_, err := executor.ExecContext(
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
		nullBool(record.AvailabilityEnabled),
		nullBool(record.AvailabilityEnabled),
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
	return cmp.Or(strings.TrimSpace(ownerUserID), authctx.OwnerUserID(ctx))
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

func nullString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func nullBool(value sql.NullBool) any {
	if value.Valid {
		return value.Bool
	}
	return nil
}

func emptyStringAsNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
