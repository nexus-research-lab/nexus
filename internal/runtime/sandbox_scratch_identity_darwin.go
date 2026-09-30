//go:build darwin

// INPUT: 创建时保留的宿主 lease 句柄和原目录身份；不读取任务可写 marker。
// OUTPUT: 放行前可持久保存的 parent/leaf 文件系统身份；替换或释放即拒绝。
// POS: macOS 崩溃资源恢复的身份来源，不自行删除目录或清除 unknown。
package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func sandboxDirectoryIdentity(info os.FileInfo) (string, error) {
	if info == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("scratch identity requires directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Ino == 0 {
		return "", errors.New("scratch native directory identity unavailable")
	}
	// Device and inode identify the filesystem object. Generation and birth time
	// additionally distinguish reuse; mtime/ctime change during ordinary IO.
	return fmt.Sprintf("darwin-v1:%x:%x:%x:%x:%x", uint64(stat.Dev), stat.Ino, stat.Gen, stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec), nil
}

func supervisedScratchBinding(lease *SandboxResourceLease) (protocol.SandboxProcessScratch, error) {
	empty := protocol.SandboxProcessScratch{}
	if lease == nil {
		return empty, nil
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	if lease.released || lease.resource == nil {
		return empty, errors.New("scratch binding requires live lease")
	}
	resource := lease.resource
	resource.mu.Lock()
	defer resource.mu.Unlock()
	if resource.closed || resource.cleanupErr != nil || resource.base == nil {
		return empty, errors.New("scratch binding requires clean host resource")
	}
	baseID, err := sandboxDirectoryIdentity(resource.baseIdentity)
	if err != nil {
		return empty, err
	}
	leafID, err := sandboxDirectoryIdentity(resource.leafIdentity)
	if err != nil {
		return empty, err
	}
	name := filepath.Base(resource.path)
	if filepath.Dir(resource.path) != resource.root || !strings.HasPrefix(name, ".scratch-") || !filepath.IsAbs(resource.root) {
		return empty, errors.New("invalid host scratch path")
	}
	// Resolve the currently published parent and compare it with the original
	// handle. Never silently replace captured identity with a new directory.
	current, err := confinedfs.Open(resource.root)
	if err != nil {
		return empty, err
	}
	defer current.Close()
	info, err := current.Stat(".")
	if err != nil {
		return empty, err
	}
	currentID, err := sandboxDirectoryIdentity(info)
	if err != nil || currentID != baseID {
		return empty, errors.New("scratch parent identity changed")
	}
	leaf, err := current.OpenRootNoSymlink(name)
	if err != nil {
		return empty, err
	}
	defer leaf.Close()
	info, err = leaf.Stat(".")
	if err != nil {
		return empty, err
	}
	currentID, err = sandboxDirectoryIdentity(info)
	if err != nil || currentID != leafID {
		return empty, errors.New("scratch leaf identity changed")
	}
	return protocol.SandboxProcessScratch{BasePath: resource.root, LeafName: name, BaseIdentity: baseID, LeafIdentity: leafID}, nil
}
