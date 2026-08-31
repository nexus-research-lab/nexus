package roomrepo

import (
	"sync"
	"testing"

	"github.com/pressly/goose/v3"
)

// goose.SetDialect writes an unsynchronized package-level global in goose v3,
// which data-races when t.Parallel() tests run migrations concurrently.
// Set it once per test package instead of in every test helper.
var (
	gooseDialectOnce sync.Once
	gooseDialectErr  error
)

func ensureGooseSQLiteDialect(t *testing.T) {
	t.Helper()
	gooseDialectOnce.Do(func() {
		gooseDialectErr = goose.SetDialect("sqlite3")
	})
	if gooseDialectErr != nil {
		t.Fatalf("设置 goose 方言失败: %v", gooseDialectErr)
	}
}
