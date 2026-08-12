package migration

import (
	"math"
	"testing"

	"github.com/pressly/goose/v3"
)

// latestTestMigrationVersion 返回当前 checkout 的最高 migration；只用于断言
// 完整 goose.Up 结果，兼容修复自身仍分别断言 71/86 等精确历史 marker。
func latestTestMigrationVersion(t *testing.T) int64 {
	t.Helper()
	migrations, err := goose.CollectMigrations(
		providerRecoveryMigrationDir(t),
		0,
		math.MaxInt64,
	)
	if err != nil {
		t.Fatalf("读取 migration 目录失败: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("migration 目录为空")
	}
	return migrations[len(migrations)-1].Version
}
