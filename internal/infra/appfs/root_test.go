package appfs

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestRootPrefersConfiguredBundleRoot(t *testing.T) {
	bundleRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundleRoot, "skills", "imagegen"), 0o755); err != nil {
		t.Fatalf("创建 skills 目录失败: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(bundleRoot, "skills", "imagegen", "SKILL.md"),
		[]byte("---\nname: imagegen\n---\n"),
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

func TestConfigDirUsesNexusConfigDir(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".nexus-custom")
	t.Setenv(NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, configDir)

	if got := ConfigDir(); got != filepath.Clean(configDir) {
		t.Fatalf("ConfigDir() 未使用 NEXUS_CONFIG_DIR: got=%q want=%q", got, filepath.Clean(configDir))
	}
	if got := AgentRuntimeBinDir(); got != filepath.Join(filepath.Clean(configDir), "app", ".agents", "bin") {
		t.Fatalf("AgentRuntimeBinDir() 路径不正确: got=%q", got)
	}
}

func TestStateRootPrefersExplicitStateRoot(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus-state")
	t.Setenv(nexusConfigDirEnvName, "")
	t.Setenv(NexusStateRootEnvName, stateRoot)

	if got := StateRoot(); got != filepath.Clean(stateRoot) {
		t.Fatalf("StateRoot() 未使用 NEXUS_STATE_ROOT: got=%q want=%q", got, filepath.Clean(stateRoot))
	}
	if got := AppDir(); got != filepath.Join(filepath.Clean(stateRoot), "app") {
		t.Fatalf("AppDir() 路径不正确: got=%q", got)
	}
}

func TestStateRootNormalizesLegacyNexusConfigSubdirectory(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(NexusStateRootEnvName, "")
	t.Setenv(nexusConfigDirEnvName, filepath.Join(stateRoot, "config"))

	if got := StateRoot(); got != stateRoot {
		t.Fatalf("StateRoot() 未兼容旧 config 子目录: got=%q want=%q", got, stateRoot)
	}
}

func TestConfigDirDefaultsToHomeNexus(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv(nexusConfigDirEnvName, "")
	t.Setenv(NexusStateRootEnvName, "")

	if got := ConfigDir(); got != filepath.Join(homeDir, ".nexus") {
		t.Fatalf("ConfigDir() 默认目录不正确: got=%q want=%q", got, filepath.Join(homeDir, ".nexus"))
	}
}

func TestUserPathsKeepStableOwnerSegment(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(NexusStateRootEnvName, stateRoot)
	t.Setenv(nexusConfigDirEnvName, "")

	if got := UserRuntimeRoot("__system__"); got != filepath.Join(stateRoot, "users", "__system__", "runtime") {
		t.Fatalf("系统用户 runtime 路径不稳定: got=%q", got)
	}
	if got := UserWorkspaceRoot("owner/a"); got != filepath.Join(stateRoot, "users", "owner_a-1844893a", "workspace") {
		t.Fatalf("用户 workspace 路径未安全归一化: got=%q", got)
	}
	if got := UserPathSegment("owner_a"); got == UserPathSegment("owner/a") {
		t.Fatalf("不同 owner 不应归一化到同一目录: got=%q", got)
	}
	if got := UserPathSegment("owner."); got == UserPathSegment("owner") {
		t.Fatalf("Windows 会折叠的尾点 owner 不应归一化到同一目录: got=%q", got)
	}
}

func TestEnsureUserRuntimeLayoutCreatesPrivateDirectories(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(NexusStateRootEnvName, stateRoot)
	t.Setenv(nexusConfigDirEnvName, "")

	if err := EnsureUserRuntimeLayout("user_demo"); err != nil {
		t.Fatalf("创建用户 runtime 布局失败: %v", err)
	}
	runtimeRoot := UserRuntimeRoot("user_demo")
	for _, directory := range []string{
		runtimeRoot,
		filepath.Join(runtimeRoot, "projects"),
		filepath.Join(runtimeRoot, "home"),
		filepath.Join(runtimeRoot, "cache"),
		filepath.Join(runtimeRoot, "logs"),
		filepath.Join(runtimeRoot, "tmp"),
	} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("读取用户 runtime 目录失败 %q: %v", directory, err)
		}
		if !info.IsDir() {
			t.Fatalf("用户 runtime 路径不是目录: %q", directory)
		}
		// Windows 的 FileMode 不表达 ACL；目录隔离由父目录继承的 ACL 决定。
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
			t.Fatalf("用户 runtime 目录权限错误 %q: %o", directory, info.Mode().Perm())
		}
	}
}

func resetRootCacheForTest() {
	appRootOnce = sync.Once{}
	appRootPath = ""
}
