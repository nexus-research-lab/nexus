package connectors

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// OAuthClient 表示用户配置的 OAuth 应用凭据。
type OAuthClient struct {
	OwnerUserID  string
	ConnectorID  string
	ClientID     string
	ClientSecret string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type oauthClientStore struct {
	db     *sql.DB
	driver string
	key    []byte
}

func newOAuthClientStore(db *sql.DB, driver string, key []byte) *oauthClientStore {
	return &oauthClientStore{db: db, driver: driver, key: key}
}

func (s *oauthClientStore) Get(ctx context.Context, ownerUserID, connectorID string) (*OAuthClient, error) {
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

func (s *oauthClientStore) Upsert(ctx context.Context, record OAuthClient) error {
	if len(s.key) == 0 {
		return errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 OAuth 应用凭据")
	}
	encryptedSecret, err := encryptCredentialPayload(s.key, []byte(strings.TrimSpace(record.ClientSecret)))
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

func (s *oauthClientStore) Delete(ctx context.Context, ownerUserID, connectorID string) error {
	query := fmt.Sprintf(
		"DELETE FROM connector_oauth_clients WHERE owner_user_id = %s AND connector_id = %s",
		s.bind(1),
		s.bind(2),
	)
	_, err := s.db.ExecContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(connectorID))
	return err
}

func (s *oauthClientStore) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (s *oauthClientStore) decryptSecret(encryptedSecret string) ([]byte, error) {
	if len(s.key) == 0 {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法读取 OAuth 应用凭据")
	}
	return decryptCredentialPayload(s.key, encryptedSecret)
}
