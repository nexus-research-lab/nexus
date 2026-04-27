package connectors

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
)

// OAuthClientStore 封装 connector OAuth client 的 SQL 读写。
type OAuthClientStore struct {
	db     *sql.DB
	driver string
	key    []byte
}

// NewOAuthClientStore 创建 connector OAuth client 仓储。
func NewOAuthClientStore(db *sql.DB, driver string, key []byte) *OAuthClientStore {
	return &OAuthClientStore{db: db, driver: driver, key: key}
}

func (s *OAuthClientStore) Get(ctx context.Context, ownerUserID, connectorID string) (*OAuthClient, error) {
	query := fmt.Sprintf(
		"SELECT owner_user_id, connector_id, client_id, client_secret_encrypted, created_at, updated_at FROM connector_oauth_clients WHERE owner_user_id = %s AND connector_id = %s",
		s.bind(1),
		s.bind(2),
	)
	var record OAuthClient
	var encryptedSecret string
	err := s.db.QueryRowContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(connectorID)).Scan(
		&record.OwnerUserID,
		&record.ConnectorID,
		&record.ClientID,
		&encryptedSecret,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	secret, err := s.decryptSecret(encryptedSecret)
	if err != nil {
		return nil, err
	}
	record.ClientSecret = string(secret)
	return &record, nil
}

func (s *OAuthClientStore) ListByOwner(ctx context.Context, ownerUserID string) (map[string]OAuthClient, error) {
	query := fmt.Sprintf(
		"SELECT owner_user_id, connector_id, client_id, client_secret_encrypted, created_at, updated_at FROM connector_oauth_clients WHERE owner_user_id = %s",
		s.bind(1),
	)
	rows, err := s.db.QueryContext(ctx, query, strings.TrimSpace(ownerUserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]OAuthClient{}
	for rows.Next() {
		var record OAuthClient
		var encryptedSecret string
		if err = rows.Scan(
			&record.OwnerUserID,
			&record.ConnectorID,
			&record.ClientID,
			&encryptedSecret,
			&record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			return nil, err
		}
		secret, decryptErr := s.decryptSecret(encryptedSecret)
		if decryptErr != nil {
			return nil, decryptErr
		}
		record.ClientSecret = string(secret)
		result[record.ConnectorID] = record
	}
	return result, rows.Err()
}

func (s *OAuthClientStore) Upsert(ctx context.Context, record OAuthClient) error {
	if len(s.key) == 0 {
		return errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 OAuth 应用凭据")
	}
	encryptedSecret, err := credentials.EncryptPayload(s.key, []byte(strings.TrimSpace(record.ClientSecret)))
	if err != nil {
		return err
	}
	if s.driver == "pgx" {
		query := `
INSERT INTO connector_oauth_clients (
    owner_user_id, connector_id, client_id, client_secret_encrypted
) VALUES ($1, $2, $3, $4)
ON CONFLICT (owner_user_id, connector_id) DO UPDATE SET
    client_id = EXCLUDED.client_id,
    client_secret_encrypted = EXCLUDED.client_secret_encrypted,
    updated_at = CURRENT_TIMESTAMP`
		_, err = s.db.ExecContext(
			ctx,
			query,
			strings.TrimSpace(record.OwnerUserID),
			strings.TrimSpace(record.ConnectorID),
			strings.TrimSpace(record.ClientID),
			encryptedSecret,
		)
		return err
	}
	query := `
INSERT INTO connector_oauth_clients (
    owner_user_id, connector_id, client_id, client_secret_encrypted
) VALUES (?, ?, ?, ?)
ON CONFLICT(owner_user_id, connector_id) DO UPDATE SET
    client_id = excluded.client_id,
    client_secret_encrypted = excluded.client_secret_encrypted,
    updated_at = CURRENT_TIMESTAMP`
	_, err = s.db.ExecContext(
		ctx,
		query,
		strings.TrimSpace(record.OwnerUserID),
		strings.TrimSpace(record.ConnectorID),
		strings.TrimSpace(record.ClientID),
		encryptedSecret,
	)
	return err
}

func (s *OAuthClientStore) Delete(ctx context.Context, ownerUserID, connectorID string) error {
	query := fmt.Sprintf(
		"DELETE FROM connector_oauth_clients WHERE owner_user_id = %s AND connector_id = %s",
		s.bind(1),
		s.bind(2),
	)
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(connectorID))
	return err
}

func (s *OAuthClientStore) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (s *OAuthClientStore) decryptSecret(encryptedSecret string) ([]byte, error) {
	if len(s.key) == 0 {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法读取 OAuth 应用凭据")
	}
	return credentials.DecryptPayload(s.key, encryptedSecret)
}
