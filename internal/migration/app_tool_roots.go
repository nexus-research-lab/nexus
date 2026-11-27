// INPUT: 宿主 app 根。
// OUTPUT: Computer Use 状态从 app/data/computer-use 迁到 app/computer-use；删除旧 shim 目录 app/.agents。
// POS: 一次性尽力迁移，先于 Computer Use 服务与 shim 重写执行；失败只告警，下次启动重试。
package migration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func relocateAppToolRoots(migrationContext workspaceFileMigrationContext) (int, error) {
	appRoot := migrationContext.configRoot
	moved, moveErr := relocateComputerUseState(
		filepath.Join(appRoot, "data", "computer-use"),
		filepath.Join(appRoot, "computer-use"),
	)
	// 旧 shim 目录只含每次启动都会在 app/bin 重写的脚本，无需搬迁。
	removed, removeErr := removeLegacyShimRoot(filepath.Join(appRoot, ".agents"))
	return moved + removed, errors.Join(moveErr, removeErr)
}

// relocateComputerUseState 整体改名以保留已安装版本、启用状态与 token。
// 目标已存在说明新路径已被服务初始化过，以新路径为准并丢弃旧副本。
func relocateComputerUseState(source string, target string) (int, error) {
	ok, err := isRealDirectory(source)
	if err != nil || !ok {
		return 0, err
	}
	if _, err := os.Lstat(target); err == nil {
		if err := removeRetiredTree(source); err != nil {
			return 0, err
		}
		return 1, nil
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("检查 Computer Use 新状态目录 %q: %w", target, err)
	}
	if err := os.Rename(source, target); err != nil {
		return 0, fmt.Errorf("迁移 Computer Use 状态 %q -> %q: %w", source, target, err)
	}
	return 1, nil
}

// removeLegacyShimRoot 只删除 bin 与 Finder 元数据；出现其他条目时保留并不报错。
func removeLegacyShimRoot(root string) (int, error) {
	ok, err := isRealDirectory(root)
	if err != nil || !ok {
		return 0, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, fmt.Errorf("读取旧 shim 目录 %q: %w", root, err)
	}
	for _, entry := range entries {
		if entry.Name() != "bin" && entry.Name() != ".DS_Store" {
			return 0, nil
		}
	}
	if err := removeRetiredTree(root); err != nil {
		return 0, err
	}
	return 1, nil
}
