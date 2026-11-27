//go:build linux

// linux_test.go 覆盖 Linux 运行时身份 ACL、cgroup、环境、硬链接与策略测试。
package runtimeidentity

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	argFilesRoot := filepath.Join(
		appfs.UserRuntimeRootAt(config.StateRoot, value.OwnerUserID),
		"runtime",
		"arg-files",
	)
	if err := os.MkdirAll(argFilesRoot, 0o750); err != nil {
		t.Fatalf("创建旧 runtime 参数目录失败: %v", err)
	}
	argFile := filepath.Join(argFilesRoot, "mcp-config-test.json")
	if err := os.WriteFile(argFile, []byte(`{"mcpServers":{}}`), 0o640); err != nil {
		t.Fatalf("创建旧 runtime 参数文件失败: %v", err)
	}
	roomStateRoot := filepath.Join(
		appfs.UserStateRootAt(config.StateRoot, value.OwnerUserID),
		"rooms",
		"room-test",
	)
	if err := os.MkdirAll(roomStateRoot, 0o700); err != nil {
		t.Fatalf("创建旧 Room state 目录失败: %v", err)
	}
	roomStateFile := filepath.Join(roomStateRoot, "ledger.jsonl")
	if err := os.WriteFile(roomStateFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("创建旧 Room state 文件失败: %v", err)
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
	assertManagedWorkspaceMode(t, argFilesRoot, hostUID, runtimeUID, 0o750)
	assertManagedWorkspaceMode(t, argFile, hostUID, runtimeUID, 0o660)
	assertManagedWorkspaceMode(t, roomStateRoot, runtimeUID, runtimeUID, 0o770)
	assertManagedWorkspaceMode(t, roomStateFile, runtimeUID, runtimeUID, 0o660)
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
	if got := aclPermission(t, roomStateFile, aclNamedUser, hostUID); got != 6 {
		t.Fatalf("Room state host ACL = %o, want 6", got)
	}
}

func TestPrepareRuntimeArgFilesSurvivesRuntimeACLRepair(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("runtime 参数文件归属测试需要 root 执行 fchown")
	}
	const (
		hostUID    = 1001
		hostGID    = 1001
		runtimeUID = 20001
		owner      = "user-arg-file-test"
	)
	config := launcherConfig{
		StateRoot: t.TempDir(),
		HostUID:   hostUID,
		HostGID:   hostGID,
	}
	root := filepath.Join(
		appfs.UserRuntimeRootAt(config.StateRoot, owner),
		"runtime",
		"arg-files",
	)
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatalf("创建 runtime 参数目录失败: %v", err)
	}
	path := filepath.Join(root, "mcp-config-test.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{}}`), 0o640); err != nil {
		t.Fatalf("创建 runtime 参数文件失败: %v", err)
	}
	if err := os.Chown(root, hostUID, runtimeUID); err != nil {
		t.Fatalf("设置 runtime 参数目录初始归属失败: %v", err)
	}
	if err := os.Chown(path, hostUID, hostGID); err != nil {
		t.Fatalf("设置 runtime 参数文件初始归属失败: %v", err)
	}

	policy := preparedPolicy{
		OwnerUserID: owner,
		Identity: preparedIdentity{
			PrivateGID: runtimeUID,
		},
	}
	if err := prepareRuntimeArgFiles(config, policy, []string{"--mcp-config", path}); err != nil {
		t.Fatalf("prepareRuntimeArgFiles() error = %v", err)
	}
	if err := os.Chown(root, runtimeUID, runtimeUID); err != nil {
		t.Fatalf("模拟错误参数目录归属失败: %v", err)
	}
	if err := os.Chown(path, runtimeUID, runtimeUID); err != nil {
		t.Fatalf("模拟错误参数文件归属失败: %v", err)
	}
	if err := os.Chmod(root, 0o770|os.ModeSetgid); err != nil {
		t.Fatalf("模拟错误参数目录权限失败: %v", err)
	}
	value := &identity{
		OwnerUserID:   owner,
		UID:           runtimeUID,
		PrivateGID:    runtimeUID,
		LayoutVersion: userLayoutVersion,
	}
	if _, err := ensureIdentityLayout(config, value); err != nil {
		t.Fatalf("ensureIdentityLayout() error = %v", err)
	}
	if err := repairRuntimeACL(config, value); err != nil {
		t.Fatalf("repairRuntimeACL() error = %v", err)
	}
	assertManagedWorkspaceMode(t, root, hostUID, runtimeUID, 0o750)
	assertManagedWorkspaceMode(t, path, hostUID, runtimeUID, 0o660)
}

func assertManagedWorkspaceMode(t *testing.T, path string, uid int, gid int, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("Stat(%q) 未返回 syscall.Stat_t", path)
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

func TestNormalizeCgroupConfigRequiresRootWhenEnabled(t *testing.T) {
	config := launcherConfig{
		StateRoot:      "/var/lib/nexus",
		CgroupRequired: true,
	}
	if err := normalizeCgroupConfig(&config); err == nil {
		t.Fatal("cgroup_required=true 且未配置 root 时应失败")
	}
}

func TestParseRuntimeProcessSignal(t *testing.T) {
	signal, sessionWide, err := parseRuntimeProcessSignal("kill")
	if err != nil || signal != unix.SIGKILL || !sessionWide {
		t.Fatalf("parse kill = signal:%v sessionWide:%v err:%v", signal, sessionWide, err)
	}
	if _, _, err = parseRuntimeProcessSignal("stop"); err == nil {
		t.Fatal("未知 runtime signal 应被拒绝")
	}
}

func TestSignalManagedRuntimeProcessChecksOwnerAndKernelSession(t *testing.T) {
	sessionID, err := unix.Getsid(os.Getpid())
	if err != nil {
		t.Fatalf("Getsid() error = %v", err)
	}
	exists, matched, err := signalManagedRuntimeProcess(
		launcherConfig{},
		&identity{UID: os.Getuid()},
		os.Getpid(),
		sessionID,
		0,
	)
	if err != nil || !exists || !matched {
		t.Fatalf("current process = exists:%v matched:%v err:%v", exists, matched, err)
	}
	exists, matched, err = signalManagedRuntimeProcess(
		launcherConfig{},
		&identity{UID: os.Getuid() + 1},
		os.Getpid(),
		sessionID,
		0,
	)
	if err != nil || !exists || matched {
		t.Fatalf("wrong owner = exists:%v matched:%v err:%v", exists, matched, err)
	}
}

func TestNormalizeCgroupConfigRejectsStateRootOverlap(t *testing.T) {
	config := launcherConfig{
		StateRoot:  "/var/lib/nexus",
		CgroupRoot: "/var/lib/nexus/cgroups",
	}
	if err := normalizeCgroupConfig(&config); err == nil {
		t.Fatal("cgroup_root 位于 state_root 下时应失败")
	}
}

func TestRuntimeCgroupPathUsesStableBasename(t *testing.T) {
	config := launcherConfig{CgroupRoot: "/sys/fs/cgroup/nexus"}
	path, err := runtimeCgroupPath(config, "nxu_abc123")
	if err != nil {
		t.Fatalf("runtime cgroup path: %v", err)
	}
	want := filepath.Join(config.CgroupRoot, "nxu_abc123")
	if path != want || strings.Contains(path, "..") {
		t.Fatalf("path=%q, want %q", path, want)
	}
}

func TestSanitizedRuntimeEnvironmentNeverRestoresNexusCLI(t *testing.T) {
	stateRoot := filepath.Join(string(filepath.Separator), "srv", "nexus")
	homeRoot := filepath.Join(stateRoot, "runtime-home")
	policy := preparedPolicy{
		OwnerUserID: "owner-a",
		CWD:         filepath.Join(stateRoot, "users", "owner-a", "workspace", "agent-a"),
		EnvironmentNames: []string{
			"NEXUSCTL_COMMAND_PATH",
			"NEXUSCTL_USER_ID",
			"NEXUS_CLI_COMMAND_PATH",
			"NEXUS_CLI",
		},
		Identity: preparedIdentity{
			Username: "nexus-owner-a",
			HomeDir:  homeRoot,
			TempDir:  filepath.Join(homeRoot, "tmp"),
		},
	}
	result := sanitizedRuntimeEnvironment([]string{
		"NEXUSCTL_COMMAND_PATH=/host/nexusctl",
		"NEXUSCTL_USER_ID=forged-owner",
		"NEXUS_CLI_COMMAND_PATH=/host/nexusctl",
		"NEXUS_CLI=enabled",
		"PATH=/host/bin",
	}, launcherConfig{StateRoot: stateRoot}, policy)
	values := runtimeEnvironmentMap(result)

	for name := range values {
		if name == "NEXUSCTL" || name == "NEXUS_CLI" ||
			strings.HasPrefix(name, "NEXUSCTL_") ||
			strings.HasPrefix(name, "NEXUS_CLI_") {
			t.Fatalf("runtime 环境泄漏原始 Nexus CLI capability: %s=%q", name, values[name])
		}
	}
	if strings.Contains(values["PATH"], filepath.Join(stateRoot, "app", "bin")) {
		t.Fatalf("runtime PATH 仍包含旧 Agent CLI shim 目录: %q", values["PATH"])
	}
	if !strings.HasPrefix(values["PATH"], filepath.Join(homeRoot, ".local", "bin")+":") {
		t.Fatalf("runtime PATH 未从 owner 私有 bin 开始: %q", values["PATH"])
	}
}

func runtimeEnvironmentMap(environment []string) map[string]string {
	result := make(map[string]string, len(environment))
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			result[name] = value
		}
	}
	return result
}

func TestValidateRuntimeIsolationHardLinksAllowsLinksWithinOneOwner(t *testing.T) {
	stateRoot := t.TempDir()
	ownerRoot := filepath.Join(stateRoot, "users", "owner-a")
	if err := os.MkdirAll(ownerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(ownerRoot, "source")
	if err := os.WriteFile(source, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, filepath.Join(ownerRoot, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := validateRuntimeIsolationHardLinks(stateRoot); err != nil {
		t.Fatalf("同一 owner 内的完整硬链接集合不应被拒绝: %v", err)
	}
}
