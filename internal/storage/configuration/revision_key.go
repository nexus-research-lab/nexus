// INPUT: 宿主数据库、方言和迁移预留的 revision 密钥单例。
// OUTPUT: 原子安装或读取的 32 字节密钥；丢失、损坏或未知版本均返回错误。
// POS: 配置 revision 跨进程/重启稳定性的持久存储边界，不保存批准权限。
package configuration

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

type RevisionKeyStore struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

func NewRevisionKeyStore(cfg config.Config, db *sql.DB) *RevisionKeyStore {
	return &RevisionKeyStore{db: db, dialect: storage.NewSQLDialect(cfg.DatabaseDriver)}
}

// Key initializes only the migration's explicit version-0 slot. Competing
// hosts use the winning persisted key, never their losing local candidate.
// Every later call reads the database: a missing/corrupt key must not be hidden
// by an old in-memory cache or replaced with a different key on restart.
// This private metadata initialization changes no domain state or receipt.
func (s *RevisionKeyStore) Key(ctx context.Context) ([]byte, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("configuration revision key store is unavailable")
	}
	version, encoded, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	if version == 0 && encoded == "" {
		candidate := make([]byte, 32)
		if _, err = rand.Read(candidate); err != nil {
			return nil, fmt.Errorf("generate configuration revision key: %w", err)
		}
		_, err = s.db.ExecContext(ctx,
			`UPDATE configuration_revision_key SET version = 1, key_hex = `+s.dialect.Bind(1)+
				` WHERE singleton = 1 AND version = 0 AND key_hex = ''`, hex.EncodeToString(candidate))
		if err != nil {
			return nil, fmt.Errorf("initialize configuration revision key: %w", err)
		}
		version, encoded, err = s.read(ctx)
		if err != nil {
			return nil, err
		}
	}
	key, decodeErr := hex.DecodeString(encoded)
	if version != 1 || decodeErr != nil || len(key) != 32 {
		// Never include persisted key material in errors or diagnostics.
		return nil, errors.New("configuration revision key is malformed or has an unsupported version")
	}
	return key, nil
}

func (s *RevisionKeyStore) read(ctx context.Context) (int, string, error) {
	var version int
	var encoded string
	err := s.db.QueryRowContext(ctx,
		`SELECT version, key_hex FROM configuration_revision_key WHERE singleton = 1`,
	).Scan(&version, &encoded)
	if err != nil {
		return 0, "", fmt.Errorf("read configuration revision key: %w", err)
	}
	return version, encoded, nil
}
