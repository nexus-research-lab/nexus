package appfs

import (
	"crypto/sha256"
	"encoding/hex"
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

// UserSkillLibraryRoot 返回指定用户的外部 Skill 全局兼容根。
//
// 用户目录复用 Agent workspace 的 owner 层级，便于第三方工具与 nxs/Claude
// 共用同一份用户资源；系统 owner 沿用 workspace 的扁平布局。
func UserSkillLibraryRoot(ownerUserID string) string {
	workspaceRoot := filepath.Join(ConfigDir(), "workspace")
	ownerSegment := safePathSegment(ownerUserID)
	if ownerSegment == "__system__" {
		return workspaceRoot
	}
	return filepath.Join(workspaceRoot, ownerSegment)
}

// UserSkillDiscoveryRoot 返回用户外部 Skill 的 nxs/Claude 共同发现目录。
func UserSkillDiscoveryRoot(ownerUserID string) string {
	return filepath.Join(UserSkillLibraryRoot(ownerUserID), ".agents", "skills")
}

// SkillLibraryRoots 返回 runtime 需要读取的平台与当前用户 Skill 根。
func SkillLibraryRoots(ownerUserID string) []string {
	return []string{PlatformSkillRoot(), UserSkillLibraryRoot(ownerUserID)}
}

func safePathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "__system__"
	}
	var builder strings.Builder
	for _, character := range trimmed {
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z', character >= '0' && character <= '9':
			builder.WriteRune(character)
		case character == '-', character == '_', character == '.', character == '@':
			builder.WriteRune(character)
		default:
			builder.WriteRune('_')
		}
	}
	sanitized := builder.String()
	if sanitized == "" {
		return "__system__"
	}
	if sanitized == "." || sanitized == ".." || sanitized != trimmed || isReservedWindowsPathSegment(sanitized) {
		sum := sha256.Sum256([]byte(trimmed))
		return sanitized + "-" + hex.EncodeToString(sum[:4])
	}
	return sanitized
}

func isReservedWindowsPathSegment(value string) bool {
	upper := strings.ToUpper(strings.TrimRight(value, " ."))
	if dot := strings.IndexByte(upper, '.'); dot >= 0 {
		upper = upper[:dot]
	}
	switch upper {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
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
