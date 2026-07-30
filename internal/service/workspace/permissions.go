package workspace

import (
	"io/fs"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// workspaceDirectoryMode 在 Linux enforce 下保留私有组目录访问能力。
func workspaceDirectoryMode() fs.FileMode {
	return appfs.RuntimeCollaborativeDirectoryMode(0o755)
}

// workspaceFileMode 避免 enforce 的 POSIX default ACL 被创建 mode 收窄。
func workspaceFileMode() fs.FileMode {
	return appfs.RuntimeCollaborativeFileMode(0o644)
}

func runtimeIsolationEnforced() bool {
	return appfs.RuntimeIsolationEnforced()
}
