package appfs

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const nexusConfigDirEnvName = "NEXUS_CONFIG_DIR"

var (
	configDirOnce sync.Once
	configDirPath string
)

// ConfigDir 返回 Nexus 的全局配置目录。
func ConfigDir() string {
	if value := strings.TrimSpace(os.Getenv(nexusConfigDirEnvName)); value != "" {
		return filepath.Clean(expandHome(value))
	}
	configDirOnce.Do(func() {
		configDirPath = resolveDefaultConfigDir()
	})
	return configDirPath
}

func resolveDefaultConfigDir() string {
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

// PlatformSkillRoot 返回平台托管 Skill 的全局兼容根目录。
//
// 该目录同时提供 .agents/skills 与 .claude/skills 两个入口，分别供 nxs
// 和 Claude Code 运行时发现同一份平台 Skill 文件。
func PlatformSkillRoot() string {
	return filepath.Join(ConfigDir(), "platform-skills")
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
