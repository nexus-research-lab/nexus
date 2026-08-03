// INPUT: 部署环境的 WORKSPACE_PATH 与 runtime 调用方当前目录。
// OUTPUT: 不会把 Agent 当前目录误作宿主根的 canonical workspace 基址。
// POS: config 包的宿主 workspace 根规范化边界；产品 UI 不持久化该部署配置。
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

func configuredWorkspacePath(envWorkspacePath string) string {
	if isAgentRuntimeWorkspacePath(envWorkspacePath) {
		return appfs.UsersRoot()
	}
	return normalizeWorkspacePath(envWorkspacePath)
}

func isAgentRuntimeWorkspacePath(envWorkspacePath string) bool {
	runtimeWorkspacePath := strings.TrimSpace(os.Getenv("NEXUSCTL_WORKSPACE_PATH"))
	return runtimeWorkspacePath != "" && sameCleanPath(envWorkspacePath, runtimeWorkspacePath)
}

func normalizeWorkspacePath(path string) string {
	value := strings.TrimSpace(path)
	if value == "" {
		return ""
	}
	legacyDefault := filepath.Join(appfs.StateRoot(), "workspace")
	if sameCleanPath(value, legacyDefault) {
		return appfs.UsersRoot()
	}
	return value
}

func sameCleanPath(left string, right string) bool {
	left = filepath.Clean(expandLeadingHome(left))
	right = filepath.Clean(expandLeadingHome(right))
	if os.PathSeparator == '\\' {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func expandLeadingHome(path string) string {
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
			relative := strings.TrimLeft(value[2:], `/\`)
			relative = strings.ReplaceAll(relative, `\`, "/")
			return filepath.Join(home, filepath.FromSlash(relative))
		}
	}
	return value
}
