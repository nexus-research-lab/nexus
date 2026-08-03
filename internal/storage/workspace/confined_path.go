package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// openStorePath 将由 Store 生成的绝对路径重新绑定到已知宿主根。
//
// storage 层历史 API 以绝对路径传递文件位置；这里不把该路径直接交给
// os.Open，而是先确认它位于 workspace 或 host root，再以相对路径交给
// confinedfs。这样 runtime 即使在中途替换符号链接，也不能让宿主跨出根。
func (s *SessionFileStore) openStorePath(target string, createRoot bool) (*confinedfs.Root, string, error) {
	if s == nil || s.paths == nil {
		return nil, "", errors.New("workspace storage root is nil")
	}
	target = filepath.Clean(strings.TrimSpace(target))
	if target == "" || target == "." {
		return nil, "", errors.New("workspace storage path is empty")
	}
	candidates := []string{s.paths.HomeRoot, s.paths.UsersRoot}
	slices.SortStableFunc(candidates, func(left, right string) int {
		return len(right) - len(left)
	})
	var lastErr error
	for _, candidate := range candidates {
		candidate = filepath.Clean(strings.TrimSpace(candidate))
		if candidate == "" || candidate == "." {
			continue
		}
		relative, err := filepath.Rel(candidate, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if createRoot {
			if err = os.MkdirAll(candidate, storageDirectoryMode()); err != nil {
				lastErr = err
				continue
			}
		}
		root, err := confinedfs.Open(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		relative = filepath.ToSlash(relative)
		if relative == "" {
			relative = "."
		}
		return root, relative, nil
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", errors.New("workspace storage path outside configured roots")
}

func relativeStorePath(rootPath string, target string) (*confinedfs.Root, string, error) {
	return relativeStorePathWithCreate(rootPath, target, true)
}

func relativeStorePathWithCreate(rootPath string, target string, createRoot bool) (*confinedfs.Root, string, error) {
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	target = filepath.Clean(strings.TrimSpace(target))
	if rootPath == "" || target == "" || rootPath == "." || target == "." {
		return nil, "", errors.New("workspace storage path is empty")
	}
	relative, err := filepath.Rel(rootPath, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, "", errors.New("workspace storage path outside configured root")
	}
	if createRoot {
		if err = os.MkdirAll(rootPath, storageRootDirectoryMode(rootPath)); err != nil {
			return nil, "", err
		}
	}
	root, err := confinedfs.Open(rootPath)
	if err != nil {
		return nil, "", err
	}
	relative = filepath.ToSlash(relative)
	if relative == "" {
		relative = "."
	}
	return root, relative, nil
}

// storageRootDirectoryMode 为宿主托管的 Room 状态保留 host-only 目录权限。
//
// Room JSONL 由 server 进程读写，不能沿用 runtime workspace 的协作目录模式；
// 否则 owner runtime 的私有组会重新获得伪造 handoff/wake 的能力。
func storageRootDirectoryMode(rootPath string) os.FileMode {
	clean := filepath.Clean(rootPath)
	if filepath.Base(clean) == "rooms" &&
		filepath.Base(filepath.Dir(clean)) == "state" &&
		filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(clean)))) == "users" {
		return 0o700
	}
	return storageDirectoryMode()
}
