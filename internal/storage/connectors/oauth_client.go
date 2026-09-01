// INPUT: owner Connector OAuth client、active/legacy credentials keyring 与 SQL transaction。
// OUTPUT: 用自带 key identity envelope 加密保存并兼容读取旧 v1 secret 的 OAuth client。
// POS: Connector OAuth client secret 的唯一 SQL 加密存储边界。
package connectors

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
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

// OAuthClientStore 封装 connector OAuth client 的 SQL 读写。
type OAuthClientStore struct {
	db         *sql.DB
	driver     string
	keyring    *credentials.Keyring
	keyringErr error
}

// NewOAuthClientStore 创建 connector OAuth client 仓储。
func NewOAuthClientStore(db *sql.DB, driver string, key []byte) *OAuthClientStore {
	raw := ""
	if len(key) > 0 {
		raw = base64.StdEncoding.EncodeToString(key)
	}
	keyring, keyringErr := credentials.NewKeyring(raw, nil)
	return NewOAuthClientStoreWithKeyring(db, driver, keyring, keyringErr)
}

// NewOAuthClientStoreWithKeyring 让 OAuth client 与 Connector connection 共享 active/legacy keyring。
func NewOAuthClientStoreWithKeyring(
	db *sql.DB,
	driver string,
	keyring *credentials.Keyring,
	keyringErr error,
) *OAuthClientStore {
	return &OAuthClientStore{db: db, driver: driver, keyring: keyring, keyringErr: keyringErr}
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

func (s *OAuthClientStore) Upsert(ctx context.Context, record OAuthClient) error {
	return s.upsert(ctx, s.db, record)
}

// UpsertTx 在调用方事务中保存 OAuth Client，供连接状态与版本原子推进。
func (s *OAuthClientStore) UpsertTx(ctx context.Context, tx *sql.Tx, record OAuthClient) error {
	if tx == nil {
		return errors.New("OAuth Client transaction 不能为空")
	}
	return s.upsert(ctx, tx, record)
}

type oauthClientExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *OAuthClientStore) upsert(
	ctx context.Context,
	executor oauthClientExecer,
	record OAuthClient,
) error {
	if s.keyringErr != nil || s.keyring == nil {
		return errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法保存 OAuth 应用凭据")
	}
	encryptedSecret, err := s.keyring.EncryptEnvelope([]byte(strings.TrimSpace(record.ClientSecret)))
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
		_, err = executor.ExecContext(
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
	_, err = executor.ExecContext(
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
	return s.delete(ctx, s.db, ownerUserID, connectorID)
}

// DeleteTx 在调用方事务中删除 OAuth Client。
func (s *OAuthClientStore) DeleteTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	connectorID string,
) error {
	if tx == nil {
		return errors.New("OAuth Client transaction 不能为空")
	}
	return s.delete(ctx, tx, ownerUserID, connectorID)
}

func (s *OAuthClientStore) delete(
	ctx context.Context,
	executor oauthClientExecer,
	ownerUserID string,
	connectorID string,
) error {
	query := fmt.Sprintf(
		"DELETE FROM connector_oauth_clients WHERE owner_user_id = %s AND connector_id = %s",
		s.bind(1),
		s.bind(2),
	)
	_, err := executor.ExecContext(ctx, query, strings.TrimSpace(ownerUserID), strings.TrimSpace(connectorID))
	return err
}

func (s *OAuthClientStore) bind(index int) string {
	if s.driver == "pgx" {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (s *OAuthClientStore) decryptSecret(encryptedSecret string) ([]byte, error) {
	if s.keyringErr != nil || s.keyring == nil {
		return nil, errors.New("CONNECTOR_CREDENTIALS_KEY 未配置，无法读取 OAuth 应用凭据")
	}
	return s.keyring.DecryptEnvelope(encryptedSecret)
}
