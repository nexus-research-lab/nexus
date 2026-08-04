//go:build linux

// 本文件验证 Linux owner 布局升级会修复旧 nxs 创建的私有持久化 ACL。
package runtimeidentity

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"golang.org/x/sys/unix"
)

func TestEnsureIdentityLayoutRepairsManagedWorkspaceACL(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("布局归一测试需要 root 执行 fchown")
	}
	const (
		hostUID    = 1001
		hostGID    = 1001
		runtimeUID = 20001
	)
	config := launcherConfig{
		StateRoot: t.TempDir(),
		HostUID:   hostUID,
		HostGID:   hostGID,
	}
	value := &identity{
		OwnerUserID:   "user-acl-test",
		UID:           runtimeUID,
		PrivateGID:    runtimeUID,
		LayoutVersion: userLayoutVersion - 1,
	}
	agentRoot := filepath.Join(
		appfs.UserWorkspaceRootAt(config.StateRoot, value.OwnerUserID),
		"agent-test",
	)
	memoryRoot := filepath.Join(agentRoot, "memory")
	if err := os.MkdirAll(memoryRoot, 0o700); err != nil {
		t.Fatalf("创建旧 memory 目录失败: %v", err)
	}
	memoryFile := filepath.Join(memoryRoot, "project.md")
	if err := os.WriteFile(memoryFile, []byte("legacy"), 0o600); err != nil {
		t.Fatalf("创建旧 memory 文件失败: %v", err)
	}
	summaryRoot := filepath.Join(
		appfs.UserRuntimeRootAt(config.StateRoot, value.OwnerUserID),
		"projects",
		"project-test",
		"session-test",
		"session-memory",
	)
	if err := os.MkdirAll(summaryRoot, 0o700); err != nil {
		t.Fatalf("创建旧 session summary 目录失败: %v", err)
	}
	summaryFile := filepath.Join(summaryRoot, "summary.md")
	if err := os.WriteFile(summaryFile, []byte("legacy summary"), 0o600); err != nil {
		t.Fatalf("创建旧 session summary 文件失败: %v", err)
	}

	changed, err := ensureIdentityLayout(config, value)
	if err != nil {
		t.Fatalf("ensureIdentityLayout() error = %v", err)
	}
	if !changed || value.LayoutVersion != userLayoutVersion {
		t.Fatalf("layout result = (%t, %d), want upgraded to %d", changed, value.LayoutVersion, userLayoutVersion)
	}
	assertManagedWorkspaceMode(t, agentRoot, 0, runtimeUID, 0o770)
	assertManagedWorkspaceMode(t, memoryRoot, runtimeUID, runtimeUID, 0o770)
	assertManagedWorkspaceMode(t, memoryFile, runtimeUID, runtimeUID, 0o660)
	assertManagedWorkspaceMode(t, summaryRoot, runtimeUID, runtimeUID, 0o770)
	assertManagedWorkspaceMode(t, summaryFile, runtimeUID, runtimeUID, 0o660)
	if got := aclPermission(t, memoryRoot, aclNamedUser, hostUID); got != 7 {
		t.Fatalf("memory host ACL = %o, want 7", got)
	}
	if got := aclPermission(t, memoryFile, aclNamedUser, hostUID); got != 6 {
		t.Fatalf("memory file host ACL = %o, want 6", got)
	}
	if got := aclPermission(t, summaryRoot, aclNamedUser, hostUID); got != 7 {
		t.Fatalf("summary host ACL = %o, want 7", got)
	}
	if got := aclPermission(t, summaryFile, aclNamedUser, hostUID); got != 6 {
		t.Fatalf("summary file host ACL = %o, want 6", got)
	}
}

func assertManagedWorkspaceMode(t *testing.T, path string, uid int, gid int, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	stat, ok := info.Sys().(*unix.Stat_t)
	if !ok {
		t.Fatalf("Stat(%q) 未返回 unix.Stat_t", path)
	}
	if int(stat.Uid) != uid || int(stat.Gid) != gid || info.Mode().Perm() != mode {
		t.Fatalf(
			"Stat(%q) = uid:%d gid:%d mode:%o, want uid:%d gid:%d mode:%o",
			path,
			stat.Uid,
			stat.Gid,
			info.Mode().Perm(),
			uid,
			gid,
			mode,
		)
	}
}

func aclPermission(t *testing.T, path string, tag uint16, id int) uint16 {
	t.Helper()
	size, err := unix.Getxattr(path, "system.posix_acl_access", nil)
	if err != nil {
		t.Fatalf("Getxattr(%q) size error = %v", path, err)
	}
	payload := make([]byte, size)
	if _, err = unix.Getxattr(path, "system.posix_acl_access", payload); err != nil {
		t.Fatalf("Getxattr(%q) error = %v", path, err)
	}
	if len(payload) < 4 || binary.LittleEndian.Uint32(payload[:4]) != aclXattrVersion {
		t.Fatalf("ACL(%q) header invalid", path)
	}
	for offset := 4; offset+8 <= len(payload); offset += 8 {
		entryTag := binary.LittleEndian.Uint16(payload[offset : offset+2])
		entryPermission := binary.LittleEndian.Uint16(payload[offset+2 : offset+4])
		entryID := binary.LittleEndian.Uint32(payload[offset+4 : offset+8])
		if entryTag == tag && entryID == uint32(id) {
			return entryPermission
		}
	}
	t.Fatalf("ACL(%q) missing tag=%d id=%d", path, tag, id)
	return 0
}
