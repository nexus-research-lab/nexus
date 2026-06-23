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

func TestConfigDirUsesNexusConfigDir(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".nexus-custom")
	t.Setenv(claudeConfigDirEnvName, filepath.Join(t.TempDir(), ".claude-custom"))
	t.Setenv(nexusConfigDirEnvName, configDir)
	resetConfigDirCacheForTest()

	if got := ConfigDir(); got != filepath.Clean(configDir) {
		t.Fatalf("ConfigDir() 未使用 NEXUS_CONFIG_DIR: got=%q want=%q", got, filepath.Clean(configDir))
	}
	if got := AgentRuntimeBinDir(); got != filepath.Join(filepath.Clean(configDir), ".agents", "bin") {
		t.Fatalf("AgentRuntimeBinDir() 路径不正确: got=%q", got)
	}
}

func TestConfigDirFallsBackToClaudeConfigDir(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".claude-custom")
	t.Setenv(nexusConfigDirEnvName, "")
	t.Setenv(claudeConfigDirEnvName, configDir)
	resetConfigDirCacheForTest()

	if got := ConfigDir(); got != filepath.Clean(configDir) {
		t.Fatalf("ConfigDir() 未使用 CLAUDE_CONFIG_DIR fallback: got=%q want=%q", got, filepath.Clean(configDir))
	}
}

func TestConfigDirDefaultsToHomeNexus(t *testing.T) {
	homeDir := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", homeDir)
	t.Setenv(nexusConfigDirEnvName, "")
	t.Setenv(claudeConfigDirEnvName, "")
	resetConfigDirCacheForTest()

	if got := ConfigDir(); got != filepath.Join(homeDir, ".nexus") {
		t.Fatalf("ConfigDir() 默认目录不正确: got=%q want=%q", got, filepath.Join(homeDir, ".nexus"))
	}
}

func resetRootCacheForTest() {
	appRootOnce = sync.Once{}
	appRootPath = ""
}

func resetConfigDirCacheForTest() {
	configDirOnce = sync.Once{}
	configDirPath = ""
}
