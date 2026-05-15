package appfs

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRootPrefersConfiguredBundleRoot(t *testing.T) {
	bundleRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundleRoot, "skills", "scheduled-task-manager"), 0o755); err != nil {
		t.Fatalf("创建 skills 目录失败: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(bundleRoot, "skills", "scheduled-task-manager", "SKILL.md"),
		[]byte("---\nname: scheduled-task-manager\n---\n"),
		0o644,
	); err != nil {
		t.Fatalf("写入 SKILL.md 失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(bundleRoot, "db", "migrations"), 0o755); err != nil {
		t.Fatalf("创建 db 目录失败: %v", err)
	}

	t.Setenv(appRootEnvName, bundleRoot)
	resetRootCacheForTest()

	if got := Root(); got != filepath.Clean(bundleRoot) {
		t.Fatalf("Root() 未优先使用 bundle root: got=%q want=%q", got, filepath.Clean(bundleRoot))
	}
}

func resetRootCacheForTest() {
	appRootOnce = sync.Once{}
	appRootPath = ""
}
