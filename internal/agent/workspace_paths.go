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

// ResolveWorkspacePath 计算 Agent workspace 路径。
func ResolveWorkspacePath(cfg config.Config, agentName string) string {
	return filepath.Join(WorkspaceBasePath(cfg), BuildWorkspaceDirName(agentName))
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
