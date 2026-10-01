// INPUT: 迁移目录中的 goose 文件名（<version>_<name>.sql）。
// OUTPUT: 按名称片段解析出的迁移版本号。
// POS: 测试基建——迁移经过重排（d3a9fa0d9 等）后版本数字会漂移，
// 断言不得硬编码具体版本，见 #289。
package sandbox

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// sandboxMigrationVersion 返回名称包含 namePart 的迁移版本号，
// 例如 "sandbox_lifecycle_recovery_scan" → 156。迁移重排后该函数
// 自动跟随新位置，避免测试因版本号漂移而落地即红。
func sandboxMigrationVersion(t *testing.T, namePart string) int64 {
	t.Helper()
	entries, err := os.ReadDir("../../../db/migrations/sqlite")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	marker := "_" + namePart
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		i := strings.Index(name, marker)
		if i <= 0 {
			continue
		}
		var version int64
		if _, err := fmt.Sscanf(name[:i], "%d", &version); err == nil {
			return version
		}
	}
	t.Fatalf("migration whose name contains %q not found", namePart)
	return 0
}
