package appfs

import (
	"io/fs"
	"os"
	"runtime"
	"strings"
)

// RuntimeIsolationEnforced 判断宿主是否启用 Linux runtime 强隔离。
func RuntimeIsolationEnforced() bool {
	return runtime.GOOS == "linux" &&
		strings.EqualFold(
			strings.TrimSpace(os.Getenv("NEXUS_RUNTIME_ISOLATION_MODE")),
			"enforce",
		)
}

// RuntimeCollaborativeDirectoryMode 保留宿主与 owner 私有组的目录协作位。
func RuntimeCollaborativeDirectoryMode(defaultMode fs.FileMode) fs.FileMode {
	if RuntimeIsolationEnforced() {
		return 0o770
	}
	return defaultMode
}

// RuntimeCollaborativeFileMode 保留宿主与 owner 私有组的文件读写位。
func RuntimeCollaborativeFileMode(defaultMode fs.FileMode) fs.FileMode {
	if RuntimeIsolationEnforced() {
		return 0o660
	}
	return defaultMode
}
