// =====================================================
// @File   ：workspace_paths.go
// @Date   ：2026/04/16 14:31:00
// @Author ：leemysw
// 2026/04/16 14:31:00   Create
// =====================================================

package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
)

// WorkspaceBasePath 返回 workspace 根目录。
func WorkspaceBasePath(cfg config.Config) string {
	if strings.TrimSpace(cfg.WorkspacePath) != "" {
		return expandHome(cfg.WorkspacePath)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nexus/workspace"
	}
	return filepath.Join(home, ".nexus", "workspace")
}

// UserWorkspaceBasePath 返回指定用户的 Agent workspace 根目录。
func UserWorkspaceBasePath(cfg config.Config, ownerUserID string) string {
	if strings.TrimSpace(ownerUserID) == systemOwnerUserID {
		return WorkspaceBasePath(cfg)
	}
	return filepath.Join(WorkspaceBasePath(cfg), "users", BuildWorkspaceDirName(ownerUserID), "agents")
}

// ResolveWorkspacePath 计算 Agent workspace 路径。
func ResolveWorkspacePath(cfg config.Config, ownerUserID string, agentName string) string {
	return filepath.Join(UserWorkspaceBasePath(cfg, ownerUserID), BuildWorkspaceDirName(agentName))
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
