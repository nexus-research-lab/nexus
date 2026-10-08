package roomrepo

import (
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"
)

func roomRepositoryMigrationDir(t *testing.T, dialect string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", dialect)
}
