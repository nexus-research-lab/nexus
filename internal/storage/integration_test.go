// integration_test.go 覆盖数据库基础与目录版本集成测试。
package storage_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/pressly/goose/v3"
)

// A positive target freezes an old release; zero applies the current chain and
// verifies that another startup has no migrations left to apply.
func migrateIntegrationDatabase(t testing.TB, cfg config.Config, target int64) {
	t.Helper()
	db, err := storage.OpenMigrationDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	dialect := goose.DialectSQLite3
	if cfg.DatabaseDriver == "postgres" {
		dialect = goose.DialectPostgres
	}
	provider, err := goose.NewProvider(dialect, db, os.DirFS(filepath.Join("..", "..", "db", "migrations", cfg.DatabaseDriver)))
	if err != nil {
		t.Fatal(err)
	}
	if target > 0 {
		if _, err = provider.UpTo(t.Context(), target); err != nil {
			t.Fatal(err)
		}
		return
	}
	if _, err = provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	if results, err := provider.Up(t.Context()); err != nil || len(results) != 0 {
		t.Fatalf("migration replay: %v, %v", results, err)
	}
}

func openIntegrationDatabase(t testing.TB, cfg config.Config) *sql.DB {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func integrationDatabase(t testing.TB, driver string) config.Config {
	t.Helper()
	cfg := config.Config{DatabaseDriver: driver, DatabaseURL: filepath.Join(t.TempDir(), "integration.db")}
	if driver == "sqlite" {
		return cfg
	}
	dsn := os.Getenv("NEXUS_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("set NEXUS_TEST_POSTGRES_URL to run PostgreSQL integration tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	schema := fmt.Sprintf("nexus_test_%d", time.Now().UnixNano())
	if _, err = admin.ExecContext(t.Context(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Errorf("cleanup schema: %v", err)
		}
	})
	endpoint, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := endpoint.Query()
	query.Set("search_path", schema)
	endpoint.RawQuery = query.Encode()
	cfg.DatabaseURL = endpoint.String()
	return cfg
}
