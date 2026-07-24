package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// openTranscriptEntry 将 transcript 路径绑定到 workspace 对应的 projects 根，
// 或绑定到同一 workspace 的显式运行输出根。调用方仍需决定是否接受符号链接。
func openTranscriptEntry(workspacePath string, transcriptPath string) (*confinedfs.Root, string, os.FileInfo, error) {
	transcriptPath = filepath.Clean(strings.TrimSpace(transcriptPath))
	if transcriptPath == "" {
		return nil, "", nil, os.ErrNotExist
	}
	candidates := []string{
		transcriptProjectsDirForWorkspace(workspacePath),
		filepath.Clean(workspacePath),
		appfs.AppDir(),
	}
	var lastErr error
	for _, candidate := range candidates {
		candidate = filepath.Clean(strings.TrimSpace(candidate))
		if candidate == "" || candidate == "." {
			continue
		}
		relative, err := filepath.Rel(candidate, transcriptPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		root, _, err := relativeStorePathWithCreate(candidate, transcriptPath, false)
		if err != nil {
			lastErr = err
			continue
		}
		info, statErr := root.Lstat(filepath.ToSlash(relative))
		if statErr != nil {
			root.Close()
			if errors.Is(statErr, os.ErrNotExist) {
				lastErr = statErr
				continue
			}
			return nil, "", nil, statErr
		}
		return root, filepath.ToSlash(relative), info, nil
	}
	if lastErr != nil {
		return nil, "", nil, lastErr
	}
	return nil, "", nil, os.ErrNotExist
}

// openTranscriptPath 只接受授权根内的真实 transcript 文件；通用读写入口不得
// 跟随 runtime 可替换的符号链接。
func openTranscriptPath(workspacePath string, transcriptPath string) (*confinedfs.Root, string, os.FileInfo, error) {
	root, relative, info, err := openTranscriptEntry(workspacePath, transcriptPath)
	if err != nil {
		return nil, "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		root.Close()
		return nil, "", nil, errors.New("transcript symlink is not allowed")
	}
	return root, relative, info, nil
}

// resolveTranscriptLinkTarget 只读取授权根内的链接本身。返回的目标仍需再次经过
// openTranscriptPath 校验，因此相对链接、绝对链接和链接链都不能逃逸授权根。
func resolveTranscriptLinkTarget(workspacePath string, transcriptPath string) (string, error) {
	transcriptPath = filepath.Clean(strings.TrimSpace(transcriptPath))
	root, relative, info, err := openTranscriptEntry(workspacePath, transcriptPath)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if info.Mode()&os.ModeSymlink == 0 {
		return "", errors.New("transcript path is not a symlink")
	}
	target, err := root.Readlink(relative)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(transcriptPath), target)
	}
	return filepath.Clean(target), nil
}
