//go:build linux

package runtimeidentity

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type protectedInodeKey struct {
	device uint64
	inode  uint64
}

type protectedInodeObservation struct {
	linkCount   uint64
	occurrences uint64
	domains     map[string]struct{}
	samplePaths []string
}

// validateRuntimeIsolationHardLinks 拒绝跨用户、跨项目或指向隔离根外的硬链接。
//
// ACL 和 chown 作用于 inode 而不是路径；若存量数据保留跨边界硬链接，最后迁移
// 的 owner 会改变所有别名的权限。该检查必须由已提升完整 root 身份的 launcher
// 执行，不能要求 host app UID 穿越 runtime 创建的 0700 私有目录。
func validateRuntimeIsolationHardLinks(stateRoot string) error {
	observations := map[protectedInodeKey]*protectedInodeObservation{}
	for _, protectedRoot := range []struct {
		path   string
		prefix string
	}{
		{path: filepath.Join(stateRoot, "users"), prefix: "user"},
		{path: filepath.Join(stateRoot, "shared-workspaces"), prefix: "project"},
	} {
		if err := inspectProtectedHardLinks(
			protectedRoot.path,
			protectedRoot.prefix,
			observations,
		); err != nil {
			return err
		}
	}
	for _, observation := range observations {
		if len(observation.domains) <= 1 &&
			observation.occurrences >= observation.linkCount {
			continue
		}
		return fmt.Errorf(
			"runtime isolation 检测到跨边界硬链接: nlink=%d observed=%d paths=%s",
			observation.linkCount,
			observation.occurrences,
			strings.Join(observation.samplePaths, ", "),
		)
	}
	return nil
}

func inspectProtectedHardLinks(
	root string,
	domainPrefix string,
	observations map[protectedInodeKey]*protectedInodeObservation,
) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("runtime isolation 根必须是无符号链接目录: %s", root)
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("无法读取文件 inode 元数据: %s", path)
		}
		linkCount := uint64(stat.Nlink)
		if linkCount <= 1 {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		domain := isolationDomain(domainPrefix, relative)
		key := protectedInodeKey{device: uint64(stat.Dev), inode: uint64(stat.Ino)}
		observation := observations[key]
		if observation == nil {
			observation = &protectedInodeObservation{
				linkCount: linkCount,
				domains:   map[string]struct{}{},
			}
			observations[key] = observation
		}
		observation.occurrences++
		observation.domains[domain] = struct{}{}
		if len(observation.samplePaths) < 3 {
			observation.samplePaths = append(observation.samplePaths, path)
		}
		return nil
	})
}

func isolationDomain(prefix string, relativePath string) string {
	relativePath = filepath.ToSlash(filepath.Clean(relativePath))
	segment, _, _ := strings.Cut(relativePath, "/")
	if segment == "" || segment == "." || segment == ".." {
		segment = "__root__"
	}
	return prefix + ":" + segment
}
