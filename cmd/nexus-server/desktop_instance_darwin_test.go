//go:build darwin

// INPUT: 含旧版数据库/会话的状态根及 sidecar 实例锁。
// OUTPUT: 锁先于迁移建立且迁移保留原 inode，旧数据进入 canonical 布局。
// POS: 可执行入口的迁移顺序验证，不启动真实用户服务。
package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/desktopinstance"
	"github.com/nexus-research-lab/nexus/internal/migration"
)

func TestDesktopInstanceLockSurvivesLegacyLayoutMigration(t *testing.T) {
	root := t.TempDir()
	for path, data := range map[string]string{"data/nexus.db": "legacy-database", "projects/session.jsonl": "legacy-session"} {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	lock, err := acquireDesktopInstanceLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := migration.RunStateLayout(root, nil); err != nil {
		t.Fatal(err)
	}
	if err := lock.(*desktopinstance.Guard).Verify(); err != nil {
		t.Fatal(err)
	}
	other, err := acquireDesktopInstanceLock(root)
	if other != nil {
		other.Close()
	}
	if !errors.Is(err, desktopinstance.ErrInUse) {
		t.Fatalf("migration lost ownership: %v", err)
	}
	for path, want := range map[string]string{"app/data/nexus.db": "legacy-database", "users/__system__/runtime/projects/session.jsonl": "legacy-session"} {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != want {
			t.Fatalf("%s data=%q err=%v", path, got, err)
		}
	}
}
