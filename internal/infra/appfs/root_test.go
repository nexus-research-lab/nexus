package appfs

import (
	"path/filepath"
	"testing"
)

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
