package appfs

import (
	"os"
	"path/filepath"
	"strings"
)

const nexusConfigDirEnvName = "NEXUS_CONFIG_DIR"

// ConfigDir 返回 Nexus 的全局配置目录。
func ConfigDir() string {
	if value := strings.TrimSpace(os.Getenv(nexusConfigDirEnvName)); value != "" {
		return filepath.Clean(expandHome(value))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Clean(filepath.Join(".", ".nexus"))
	}
	return filepath.Join(home, ".nexus")
}

// AgentRuntimeBinDir 返回所有 agent 共享的运行时工具目录。
func AgentRuntimeBinDir() string {
	return filepath.Join(ConfigDir(), ".agents", "bin")
}

func expandHome(path string) string {
	value := strings.TrimSpace(path)
	switch {
	case value == "~":
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	case strings.HasPrefix(value, "~/"), strings.HasPrefix(value, `~\`):
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, value[2:])
		}
	}
	return value
}
