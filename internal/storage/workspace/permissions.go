package workspace

import (
	"io/fs"
	"os"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// storageDirectoryMode 在 Linux enforce 下避免宿主 app 文件树新建
// 目录时意外带出 other 读权限；用户 workspace 仍由父级 ACL 控制。
func storageDirectoryMode() fs.FileMode {
	return appfs.RuntimeCollaborativeDirectoryMode(0o755)
}

// storageFileMode 保留宿主与 runtime 私有组协作所需的 group 读写位。
func storageFileMode(defaultMode os.FileMode) os.FileMode {
	return appfs.RuntimeCollaborativeFileMode(defaultMode)
}
