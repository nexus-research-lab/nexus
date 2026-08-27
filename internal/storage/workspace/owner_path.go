package workspace

import (
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// workspacePathBelongsToOwner 判断持久化路径是否属于 owner workspace 域。
//
// canonical 与自定义根都必须包含 owner 子树。共享项目不能复用这个入口：
// 宿主进程不受 runtime ACL 约束，必须由携带项目成员授权的专用 API 打开。
func (s *Store) workspacePathBelongsToOwner(ownerUserID string, workspacePath string) bool {
	workspacePath = filepath.Clean(strings.TrimSpace(workspacePath))
	if workspacePath == "" || workspacePath == "." {
		return false
	}
	configuredOwnerRoot := filepath.Join(
		s.WorkspaceRoot,
		appfs.UserPathSegment(ownerUserID),
		"workspace",
	)
	if directChildPath(configuredOwnerRoot, workspacePath) {
		return true
	}
	canonicalOwnerRoot := appfs.UserWorkspaceRootAt(s.StateRoot, ownerUserID)
	if directChildPath(canonicalOwnerRoot, workspacePath) {
		return true
	}
	return false
}

func directChildPath(root string, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		filepath.Dir(relative) == "."
}
