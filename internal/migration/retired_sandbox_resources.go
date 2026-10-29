// INPUT: 宿主 app 根与其所在状态根。
// OUTPUT: 删除旧版 macOS 桌面沙箱的监督进程根与带租约标记的 scratch 目录。
// POS: 无状态沙箱后的一次性尽力清理；失败不阻断启动，下次启动重试。
package migration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	// 旧版宿主监督进程与 scratch 隔离区的根，整棵树只由旧沙箱写入。
	retiredSandboxProcessesDirName = "processes"
	// 旧版 scratch 父目录位于 users/<owner>/runtime 下；runtime 根同时是
	// SDK 配置目录，因此只删除带旧租约标记的子目录，不整目录删除。
	retiredSandboxScratchDirName  = "sandbox"
	retiredSandboxLeaseMarkerName = ".nexus-sandbox-lease.json"
	// 删除前先改名为墓碑：删除中途失败会先删掉租约标记，墓碑名保证下次仍能识别。
	retiredSandboxTombstoneSuffix = ".nexus-retired"
)

func removeRetiredSandboxResources(migrationContext workspaceFileMigrationContext) (int, error) {
	appRoot := migrationContext.configRoot
	removed := 0
	var errs []error
	processesRoot := filepath.Join(appRoot, retiredSandboxProcessesDirName)
	if ok, err := isRealDirectory(processesRoot); err != nil {
		errs = append(errs, err)
	} else if ok {
		if err := removeRetiredTree(processesRoot); err != nil {
			errs = append(errs, err)
		} else {
			removed++
		}
	}
	owners, err := directChildDirectories(filepath.Join(filepath.Dir(appRoot), "users"))
	if err != nil {
		return removed, errors.Join(append(errs, err)...)
	}
	for _, owner := range owners {
		count, err := removeRetiredSandboxScratch(filepath.Join(owner, "runtime", retiredSandboxScratchDirName))
		removed += count
		if err != nil {
			errs = append(errs, err)
		}
	}
	return removed, errors.Join(errs...)
}

// removeRetiredSandboxScratch 只删除带旧租约标记或墓碑名的 leaf；父目录清空后一并移除。
func removeRetiredSandboxScratch(base string) (int, error) {
	ok, err := isRealDirectory(base)
	if err != nil || !ok {
		return 0, err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return 0, fmt.Errorf("读取旧沙箱 scratch 目录 %q: %w", base, err)
	}
	removed := 0
	var errs []error
	for _, entry := range entries {
		leaf := filepath.Join(base, entry.Name())
		if entry.Type()&fs.ModeType != fs.ModeDir {
			continue
		}
		if !strings.HasSuffix(entry.Name(), retiredSandboxTombstoneSuffix) {
			if _, err := os.Lstat(filepath.Join(leaf, retiredSandboxLeaseMarkerName)); err != nil {
				if !os.IsNotExist(err) {
					errs = append(errs, fmt.Errorf("检查旧沙箱租约标记 %q: %w", leaf, err))
				}
				continue
			}
			tombstone := leaf + retiredSandboxTombstoneSuffix
			if err := os.Rename(leaf, tombstone); err != nil {
				errs = append(errs, fmt.Errorf("标记待删除的旧沙箱 scratch %q: %w", leaf, err))
				continue
			}
			leaf = tombstone
		}
		if err := removeRetiredTree(leaf); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	if remaining, err := os.ReadDir(base); err == nil && len(remaining) == 0 {
		if err := os.Remove(base); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("删除空的旧沙箱 scratch 目录 %q: %w", base, err))
		}
	}
	return removed, errors.Join(errs...)
}

func isRealDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查旧沙箱路径 %q: %w", path, err)
	}
	return info.IsDir(), nil
}

// removeRetiredTree 是删除入口，测试可替换以模拟中途失败。
var removeRetiredTree = removeTreeForced

// removeTreeForced 先补齐目录写权限再删除，覆盖 scratch 中的只读模块缓存。
// WalkDir 不跟随符号链接，链接本身作为普通条目删除。
func removeTreeForced(root string) error {
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && entry.IsDir() {
			_ = os.Chmod(path, 0o700)
		}
		return nil
	})
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("删除旧沙箱资源 %q: %w", root, err)
	}
	return nil
}
