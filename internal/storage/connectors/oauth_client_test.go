package connectors

import (
	"context"
	"database/sql"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"

	_ "modernc.org/sqlite"
)

func TestOAuthClientStoreRequiresEncryptionKey(t *testing.T) {
	db := newOAuthClientStoreDB(t)
	defer db.Close()

	store := NewOAuthClientStore(db, "sqlite", nil)
	err := store.Upsert(context.Background(), OAuthClient{
		OwnerUserID:  "user-1",
		ConnectorID:  "github",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	if err == nil || !strings.Contains(err.Error(), "CONNECTOR_CREDENTIALS_KEY 未配置") {
		t.Fatalf("缺少 key 应拒绝保存 secret，实际: %v", err)
	}
}

func newOAuthClientStoreDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := filepath.Join(t.TempDir(), "connectors.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if _, err = db.Exec(`
CREATE TABLE connector_oauth_clients (
    owner_user_id VARCHAR(64) NOT NULL,
    connector_id VARCHAR(128) NOT NULL,
    client_id VARCHAR(512) NOT NULL,
    client_secret_encrypted TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, connector_id)
)`); err != nil {
		_ = db.Close()
		t.Fatalf("创建 OAuth client 测试表失败: %v", err)
	}
	return db
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
