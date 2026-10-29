// INPUT: 带旧版沙箱监督根、scratch leaf 与无关 runtime 文件的临时状态根。
// OUTPUT: 验证只删除旧沙箱资源、只读树可删除、失败不阻断且下次重试。
// POS: 旧沙箱资源清理迁移的回归测试。
package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRunWorkspaceFilesRemovesRetiredSandboxResources(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	workspaceRoot := filepath.Join(stateRoot, "workspace")
	scratchBase := filepath.Join(stateRoot, "users", "owner-a", "runtime", "sandbox")
	leasedLeaf := filepath.Join(scratchBase, "lease-1")
	readOnlyCache := filepath.Join(leasedLeaf, "gomod", "pkg@v1")
	unmarkedLeaf := filepath.Join(scratchBase, "not-ours")
	otherOwnerBase := filepath.Join(stateRoot, "users", "owner-b", "runtime", "sandbox")
	keptRuntimeFile := filepath.Join(stateRoot, "users", "owner-a", "runtime", "settings.json")

	writeMigrationTestFile(t, filepath.Join(appRoot, "processes", "p", "job.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, "processes", "scratch-recovery", "q", "file"), "x\n")
	writeMigrationTestFile(t, filepath.Join(leasedLeaf, retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, filepath.Join(readOnlyCache, "go.mod"), "module x\n")
	writeMigrationTestFile(t, filepath.Join(unmarkedLeaf, "keep.txt"), "keep\n")
	writeMigrationTestFile(t, filepath.Join(otherOwnerBase, "lease-2", retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, keptRuntimeFile, "{}\n")
	if err := os.Chmod(readOnlyCache, 0o500); err != nil {
		t.Fatal(err)
	}

	if err := RunWorkspaceFiles(appRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行工作区文件迁移失败: %v", err)
	}
	assertMigrationPathMissing(t, filepath.Join(appRoot, "processes"))
	assertMigrationPathMissing(t, leasedLeaf)
	assertMigrationPathMissing(t, otherOwnerBase)
	assertMigrationPathExists(t, filepath.Join(unmarkedLeaf, "keep.txt"))
	assertMigrationPathExists(t, keptRuntimeFile)
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))
}

func TestRunWorkspaceFilesRetriesRetiredSandboxCleanupWithoutBlockingStartup(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	scratchBase := filepath.Join(stateRoot, "users", "owner-a", "runtime", "sandbox")
	leasedLeaf := filepath.Join(scratchBase, "lease-1")
	lockedDir := filepath.Join(leasedLeaf, "locked")
	writeMigrationTestFile(t, filepath.Join(leasedLeaf, retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, filepath.Join(lockedDir, "file"), "x\n")
	// 改名成功、删除中途失败：租约标记已被删掉，墓碑名仍须让下次启动识别。
	failRemove := true
	previousRemove := removeRetiredTree
	removeRetiredTree = func(root string) error {
		if failRemove {
			_ = os.Remove(filepath.Join(root, retiredSandboxLeaseMarkerName))
			return os.ErrPermission
		}
		return previousRemove(root)
	}
	t.Cleanup(func() { removeRetiredTree = previousRemove })

	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatalf("尽力型清理失败不应阻断启动: %v", err)
	}
	assertMigrationPathExists(t, leasedLeaf+retiredSandboxTombstoneSuffix)
	assertMigrationPathMissing(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))

	failRemove = false
	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertMigrationPathMissing(t, scratchBase)
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))
}

func TestSandboxRecoveryTablesDropAndRollback(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "host.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	dir := providerRecoveryMigrationDir(t)
	if err = goose.UpTo(db, dir, 158); err != nil {
		t.Fatal(err)
	}
	// 旧版留下的未收口记录不能阻止删除。
	if _, err = db.Exec(`INSERT INTO sandbox_scratch_recoveries VALUES ('o','s','l',1,'launch','{}','prepared')`); err != nil {
		t.Fatal(err)
	}
	tables := []string{"sandbox_policy_receipts", "sandbox_process_launches", "sandbox_scratch_recoveries"}
	countTables := func() int {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN (?,?,?)`, tables[0], tables[1], tables[2]).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	if err = goose.UpTo(db, dir, 159); err != nil {
		t.Fatal(err)
	}
	if got := countTables(); got != 0 {
		t.Fatalf("sandbox tables after 00159 = %d, want 0", got)
	}
	if err = goose.DownTo(db, dir, 158); err != nil {
		t.Fatal(err)
	}
	if got := countTables(); got != len(tables) {
		t.Fatalf("sandbox tables after rollback = %d, want %d", got, len(tables))
	}
	if err = goose.Up(db, dir); err != nil {
		t.Fatal(err)
	}
}
