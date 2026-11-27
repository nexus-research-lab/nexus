// INPUT: 独立宿主数据库连接、migration bootstrap 与损坏密钥状态。
// OUTPUT: 跨连接唯一初始化、数据库重开稳定性、损坏失败关闭和升级保留回执的证明。
// POS: 配置 revision 持久密钥的存储与 migration 回归。
package configuration

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

func newRevisionKeyDatabase(t *testing.T) (config.Config, *sql.DB) {
	t.Helper()
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: filepath.Join(t.TempDir(), "host.db")}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, "../../../db/migrations/sqlite", 141); err != nil {
		t.Fatal(err)
	}
	return cfg, db
}

func migrateRevisionKey(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
}

func TestRevisionKeyRejectsMissingOrCorruptState(t *testing.T) {
	for _, scenario := range []string{"missing", "malformed", "empty", "unknown_version", "invalid_bootstrap"} {
		t.Run(scenario, func(t *testing.T) {
			cfg, db := newRevisionKeyDatabase(t)
			migrateRevisionKey(t, db)
			store := NewRevisionKeyStore(cfg, db)
			if _, err := store.Key(t.Context()); err != nil {
				t.Fatal(err)
			}
			query := map[string]string{
				"missing":           "DELETE FROM configuration_revision_key",
				"malformed":         "UPDATE configuration_revision_key SET key_hex = 'sensitive-malformed-key'",
				"empty":             "UPDATE configuration_revision_key SET key_hex = ''",
				"unknown_version":   "UPDATE configuration_revision_key SET version = 99",
				"invalid_bootstrap": "UPDATE configuration_revision_key SET version = 0",
			}[scenario]
			if _, err := db.ExecContext(t.Context(), query); err != nil {
				t.Fatal(err)
			}
			for _, candidate := range []*RevisionKeyStore{store, NewRevisionKeyStore(cfg, db)} {
				if _, err := candidate.Key(t.Context()); err == nil || strings.Contains(err.Error(), "sensitive-malformed-key") {
					t.Fatalf("invalid key was accepted, replaced, cached or exposed: %v", err)
				}
			}
		})
	}
}

func TestRevisionKeyAcrossProcesses(t *testing.T) {
	cfg, db := newRevisionKeyDatabase(t)
	migrateRevisionKey(t, db)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	outputs := make([][]byte, 2)
	errs := make([]error, 2)
	var workers sync.WaitGroup
	for index := range outputs {
		workers.Go(func() {
			command := exec.CommandContext(t.Context(), binary, "-test.run=^TestRevisionKeyChildProcess$")
			command.Env = append(os.Environ(), "NEXUS_TEST_CONFIGURATION_REVISION_DB="+cfg.DatabaseURL)
			outputs[index], errs[index] = command.CombinedOutput()
		})
	}
	workers.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("child %d failed: %v: %s", index, err, outputs[index])
		}
	}
	if len(outputs[0]) != 64 || !bytes.Equal(outputs[0], outputs[1]) {
		t.Fatal("independent processes did not share the persisted revision key")
	}
}

func TestRevisionKeyChildProcess(t *testing.T) {
	databasePath := os.Getenv("NEXUS_TEST_CONFIGURATION_REVISION_DB")
	if databasePath == "" {
		return
	}
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: databasePath}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	key, err := NewRevisionKeyStore(cfg, db).Key(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// Only a fingerprint of this temporary test database's random key crosses
	// the child pipe. No configuration service, cache or Go memory is shared.
	fmt.Printf("%x", sha256.Sum256(key))
	os.Exit(0)
}
