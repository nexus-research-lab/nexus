package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// WorkspaceBasePath 返回 workspace 根目录。
func WorkspaceBasePath(cfg config.Config) string {
	if strings.TrimSpace(cfg.WorkspacePath) != "" {
		return expandHome(cfg.WorkspacePath)
	}
	return appfs.UsersRoot()
}

// UserWorkspaceBasePath 返回指定用户的 Agent workspace 根目录。
func UserWorkspaceBasePath(cfg config.Config, ownerUserID string) string {
	return filepath.Join(
		WorkspaceBasePath(cfg),
		appfs.UserPathSegment(ownerUserID),
		"workspace",
	)
}

// ResolveWorkspacePath 计算 Agent workspace 路径。
func ResolveWorkspacePath(cfg config.Config, ownerUserID string, agentID string) string {
	return filepath.Join(UserWorkspaceBasePath(cfg, ownerUserID), BuildWorkspaceDirName(agentID))
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
			relative := strings.TrimLeft(value[2:], `/\`)
			relative = strings.ReplaceAll(relative, `\`, "/")
			return filepath.Join(home, filepath.FromSlash(relative))
		}
	}
	return value
}
