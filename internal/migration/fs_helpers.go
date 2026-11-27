// INPUT: 迁移步骤给出的状态根内路径。
// OUTPUT: 不跟随符号链接的目录枚举、存在性检查与删除原语。
// POS: 文件迁移共享的文件系统原语；各迁移只描述删什么，不各自实现怎么删。
package migration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// directChildDirectories 列出 root 下的真实子目录；root 不存在视为空。
func directChildDirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录 %q: %w", root, err)
	}
	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(root, entry.Name()))
		}
	}
	return directories, nil
}

func isRealDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查迁移路径 %q: %w", path, err)
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
		return fmt.Errorf("删除迁移残留 %q: %w", root, err)
	}
	return nil
}

// removeEmptyDirectory 只删除真实的空目录；非空或不存在都视为无需处理。
func removeEmptyDirectory(path string) (int, error) {
	ok, err := isRealDirectory(path)
	if err != nil || !ok {
		return 0, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, fmt.Errorf("读取待清理目录 %q: %w", path, err)
	}
	for _, entry := range entries {
		if entry.Name() != ".DS_Store" {
			return 0, nil
		}
	}
	if err := os.RemoveAll(path); err != nil {
		return 0, fmt.Errorf("删除空目录 %q: %w", path, err)
	}
	return 1, nil
}

// removeRegularFile 删除普通文件；emptyOnly 时只删零字节文件，避免误删仍有内容的数据。
func removeRegularFile(path string, emptyOnly bool) (int, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("检查待清理文件 %q: %w", path, err)
	}
	if !info.Mode().IsRegular() || (emptyOnly && info.Size() != 0) {
		return 0, nil
	}
	if err := os.Remove(path); err != nil {
		return 0, fmt.Errorf("删除残留文件 %q: %w", path, err)
	}
	return 1, nil
}
