package appfs

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const nexusConfigDirEnvName = "NEXUS_CONFIG_DIR"
const NexusStateRootEnvName = "NEXUS_STATE_ROOT"

// StateRoot 返回 Nexus 的统一持久化状态根。
//
// NEXUS_STATE_ROOT 是新的宿主配置；NEXUS_CONFIG_DIR 作为旧版本兼容回退。
func StateRoot() string {
	value := strings.TrimSpace(os.Getenv(NexusStateRootEnvName))
	if value == "" {
		value = strings.TrimSpace(os.Getenv(nexusConfigDirEnvName))
	}
	if value == "" {
		return resolveDefaultConfigDir()
	}
	return normalizeStateRoot(value)
}

// AppDir 返回 Nexus 宿主控制面目录。
func AppDir() string {
	return filepath.Join(StateRoot(), "app")
}

// UsersRoot 返回所有用户数据根。
func UsersRoot() string {
	return filepath.Join(StateRoot(), "users")
}

// UserDataRoot 返回指定用户的数据根。
func UserDataRoot(ownerUserID string) string {
	return UserDataRootAt(StateRoot(), ownerUserID)
}

// UserDataRootAt 返回指定状态根下的用户数据根。
func UserDataRootAt(stateRoot string, ownerUserID string) string {
	return filepath.Join(filepath.Clean(stateRoot), "users", UserPathSegment(ownerUserID))
}

// UserPathSegment 将用户标识转换为可安全拼接到路径中的单一路径段。
func UserPathSegment(ownerUserID string) string {
	return safePathSegment(ownerUserID)
}

// UserRuntimeRoot 返回指定用户的 runtime 根。
func UserRuntimeRoot(ownerUserID string) string {
	return UserRuntimeRootAt(StateRoot(), ownerUserID)
}

// UserRuntimeRootAt 返回指定状态根下的用户 runtime 根。
func UserRuntimeRootAt(stateRoot string, ownerUserID string) string {
	return filepath.Join(UserDataRootAt(stateRoot, ownerUserID), "runtime")
}

// UserWorkspaceRoot 返回指定用户的 workspace 根。
func UserWorkspaceRoot(ownerUserID string) string {
	return UserWorkspaceRootAt(StateRoot(), ownerUserID)
}

// UserWorkspaceRootAt 返回指定状态根下的用户 workspace 根。
func UserWorkspaceRootAt(stateRoot string, ownerUserID string) string {
	return filepath.Join(UserDataRootAt(stateRoot, ownerUserID), "workspace")
}

// EnsureUserRuntimeLayout 创建用户级 runtime 必需目录。
func EnsureUserRuntimeLayout(ownerUserID string) error {
	return EnsureUserRuntimeLayoutAt(StateRoot(), ownerUserID)
}

// EnsureUserRuntimeLayoutAt 在指定状态根创建用户级 runtime 必需目录。
func EnsureUserRuntimeLayoutAt(stateRoot string, ownerUserID string) error {
	runtimeRoot := UserRuntimeRootAt(stateRoot, ownerUserID)
	for _, directory := range []string{
		runtimeRoot,
		filepath.Join(runtimeRoot, "projects"),
		filepath.Join(runtimeRoot, "home"),
		filepath.Join(runtimeRoot, "cache"),
		filepath.Join(runtimeRoot, "logs"),
		filepath.Join(runtimeRoot, "tmp"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// ConfigDir 返回当前进程使用的 runtime config 根。
//
// server 进程保留旧的 NEXUS_CONFIG_DIR 语义；宿主自己的配置必须使用 AppDir。
func ConfigDir() string {
	if value := strings.TrimSpace(os.Getenv(nexusConfigDirEnvName)); value != "" {
		return filepath.Clean(expandHome(value))
	}
	return StateRoot()
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
	return filepath.Join(AppDir(), ".agents", "bin")
}

// PlatformSkillRoot 返回平台托管 Skill 的全局兼容根目录。
//
// 该目录同时提供 .agents/skills 与 .claude/skills 两个入口，分别供 nxs
// 和 Claude Code 运行时发现同一份平台 Skill 文件。
func PlatformSkillRoot() string {
	return filepath.Join(AppDir(), "platform-skills")
}

func normalizeStateRoot(path string) string {
	clean := filepath.Clean(expandHome(path))
	parent := filepath.Dir(clean)
	if (filepath.Base(clean) == "app" || filepath.Base(clean) == "config") &&
		filepath.Base(parent) == ".nexus" {
		return parent
	}
	return clean
}

func safePathSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "__system__"
	}
	var builder strings.Builder
	for _, character := range trimmed {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '-', character == '_', character == '.', character == '@':
			builder.WriteRune(character)
		default:
			builder.WriteByte('_')
		}
	}
	sanitized := builder.String()
	if sanitized == "" {
		return "__system__"
	}
	if sanitized == "." || sanitized == ".." ||
		sanitized != trimmed || strings.HasSuffix(sanitized, ".") ||
		isReservedWindowsPathSegment(sanitized) {
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
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
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
