package agent

import (
	"io/fs"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// agentWorkspaceDirectoryMode 在 enforce 下让宿主与 runtime 使用私有组协作。
func agentWorkspaceDirectoryMode() fs.FileMode {
	return appfs.RuntimeCollaborativeDirectoryMode(0o700)
}

// agentWorkspaceFileMode 在 enforce 下避免原子替换丢失私有组写权限。
func agentWorkspaceFileMode(defaultMode fs.FileMode) fs.FileMode {
	return appfs.RuntimeCollaborativeFileMode(defaultMode)
}
