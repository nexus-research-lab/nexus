package connectors

import (
	"database/sql"
	"encoding/base64"

	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"

	_ "modernc.org/sqlite"
)

// NewOAuthClientStore 创建 connector OAuth client 仓储。
func NewOAuthClientStore(db *sql.DB, driver string, key []byte) *OAuthClientStore {
	raw := ""
	if len(key) > 0 {
		raw = base64.StdEncoding.EncodeToString(key)
	}
	keyring, keyringErr := credentials.NewKeyring(raw, nil)
	return NewOAuthClientStoreWithKeyring(db, driver, keyring, keyringErr)
}
