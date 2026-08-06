// INPUT: confined workspace 遍历回调收到的路径、目录项与错误。
// OUTPUT: 保留根错误，并把不可读子树降级为局部跳过的遍历决策。
// POS: workspace 文件树与实时 watcher 共用的容错边界。
package workspace

import (
	"errors"
	"io/fs"
	"os"
)

// handleWorkspaceWalkError 防止一个私有子目录拖垮完整 workspace。
func handleWorkspaceWalkError(path string, entry fs.DirEntry, err error) error {
	if err == nil {
		return nil
	}
	if path == "." || !errors.Is(err, os.ErrPermission) {
		return err
	}
	if entry != nil && entry.IsDir() {
		return fs.SkipDir
	}
	return nil
}
