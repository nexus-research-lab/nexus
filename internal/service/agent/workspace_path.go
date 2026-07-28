package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
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

func ensureDirectoryWithinRoot(boundaryRoot string, targetPath string, mode os.FileMode) error {
	boundaryRoot = filepath.Clean(strings.TrimSpace(boundaryRoot))
	targetPath = filepath.Clean(strings.TrimSpace(targetPath))
	if boundaryRoot == "" || targetPath == "" {
		return errors.New("workspace directory path is empty")
	}
	relative, err := filepath.Rel(boundaryRoot, targetPath)
	if err != nil || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("workspace directory escapes configured root")
	}
	if err = os.MkdirAll(boundaryRoot, mode); err != nil {
		return err
	}
	root, err := confinedfs.Open(boundaryRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	child, err := root.OpenOrCreateRootNoSymlink(filepath.ToSlash(relative), mode)
	if err != nil {
		return err
	}
	return child.Close()
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
