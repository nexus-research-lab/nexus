package connectors

import (
	"context"
	"database/sql"
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"

	_ "github.com/mattn/go-sqlite3"
)

func TestOAuthClientStoreGetEmpty(t *testing.T) {
	store, closeDB := newOAuthClientStoreForTest(t)
	defer closeDB()

	record, err := store.Get(context.Background(), "user-1", "github")
	if err != nil {
		t.Fatalf("读取空 OAuth client 失败: %v", err)
	}
	if record != nil {
		t.Fatalf("空表应返回 nil，实际: %+v", record)
	}
}

func TestOAuthClientStoreUpsertGetAndDelete(t *testing.T) {
	store, closeDB := newOAuthClientStoreForTest(t)
	defer closeDB()
	ctx := context.Background()

	err := store.Upsert(ctx, OAuthClient{
		OwnerUserID:  "user-1",
		ConnectorID:  "github",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	})
	if err != nil {
		t.Fatalf("写入 OAuth client 失败: %v", err)
	}

	record, err := store.Get(ctx, "user-1", "github")
	if err != nil {
		t.Fatalf("读取 OAuth client 失败: %v", err)
	}
	if record == nil {
		t.Fatal("写入后应能读取 OAuth client")
	}
	if record.ClientID != "client-id" || record.ClientSecret != "client-secret" {
		t.Fatalf("OAuth client 内容不正确: %+v", record)
	}
	if record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() {
		t.Fatalf("时间字段应填充: %+v", record)
	}

	if err = store.Delete(ctx, "user-1", "github"); err != nil {
		t.Fatalf("删除 OAuth client 失败: %v", err)
	}
	record, err = store.Get(ctx, "user-1", "github")
	if err != nil {
		t.Fatalf("删除后读取 OAuth client 失败: %v", err)
	}
	if record != nil {
		t.Fatalf("删除后应返回 nil，实际: %+v", record)
	}
}

func TestOAuthClientStoreWrongKeyReturnsDecryptError(t *testing.T) {
	store, closeDB := newOAuthClientStoreForTest(t)
	defer closeDB()
	ctx := context.Background()

	if err := store.Upsert(ctx, OAuthClient{
		OwnerUserID:  "user-1",
		ConnectorID:  "github",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}); err != nil {
		t.Fatalf("写入 OAuth client 失败: %v", err)
	}

	wrongStore := NewOAuthClientStore(store.db, "sqlite3", []byte("abcdefghijklmnopqrstuvwxyz123456"))
	_, err := wrongStore.Get(ctx, "user-1", "github")
	if err == nil {
		t.Fatal("错误 key 解密应失败")
	}
}

func TestOAuthClientStoreRequiresEncryptionKey(t *testing.T) {
	db := newOAuthClientStoreDB(t)
	defer db.Close()

	store := NewOAuthClientStore(db, "sqlite3", nil)
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

func newOAuthClientStoreForTest(t *testing.T) (*OAuthClientStore, func()) {
	t.Helper()

	db := newOAuthClientStoreDB(t)
	key, err := credentials.DecodeKey(testConnectorCredentialKey())
	if err != nil {
		t.Fatalf("解析测试密钥失败: %v", err)
	}
	return NewOAuthClientStore(db, "sqlite3", key), func() {
		_ = db.Close()
	}
}

func newOAuthClientStoreDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := filepath.Join(t.TempDir(), "connectors.db")
	db, err := sql.Open("sqlite3", databaseURL)
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

func testConnectorCredentialKey() string {
	return base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
}
