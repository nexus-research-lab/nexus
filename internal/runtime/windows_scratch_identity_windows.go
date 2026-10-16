//go:build windows

// INPUT: 原进程内lease对象及创建时保留的parent/leaf身份。
// OUTPUT: 启动前固定volume/fileID128/creationTime，不信任任务marker。
// POS: Windows冷恢复身份来源；路径只能查找原owner，不能制造授权。
package runtime

import (
	"errors"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"os"
	"path/filepath"
)

func captureWindowsScratch(binding windowsSandboxBinding) (string, string, error) {
	registryMu.Lock()
	r := registry[binding.ScratchRoot]
	registryMu.Unlock()
	if r == nil {
		return "", "", errors.New("original Windows scratch owner missing")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.cleanupErr != nil || r.base == nil || r.marker.OwnerUserID != binding.Owner || r.marker.SessionKey != binding.Session || r.marker.LeaseID != binding.LeaseID {
		return "", "", errors.New("Windows scratch owner changed")
	}
	parent, err := confinedfs.Open(r.root)
	if err != nil {
		return "", "", err
	}
	defer parent.Close()
	observed, err := parent.Stat(".")
	if err != nil || !os.SameFile(observed, r.baseIdentity) {
		return "", "", errors.New("original Windows scratch parent replaced")
	}
	leaf, err := parent.OpenRootNoSymlink(filepath.Base(r.path))
	if err != nil {
		return "", "", err
	}
	defer leaf.Close()
	observed, err = leaf.Stat(".")
	if err != nil || !os.SameFile(observed, r.leafIdentity) {
		return "", "", errors.New("original Windows scratch leaf replaced")
	}
	baseID, err := parent.WindowsDirectoryIdentity()
	if err != nil {
		return "", "", err
	}
	leafID, err := leaf.WindowsDirectoryIdentity()
	return baseID, leafID, err
}
