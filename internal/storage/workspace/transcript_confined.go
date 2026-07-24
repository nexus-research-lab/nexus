package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// openTranscriptPath 将 transcript 绑定到 workspace 对应的 projects 根，
// 或绑定到同一 workspace 的显式运行输出根。
func openTranscriptPath(workspacePath string, transcriptPath string) (*confinedfs.Root, string, os.FileInfo, error) {
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
		if info.Mode()&os.ModeSymlink != 0 {
			root.Close()
			return nil, "", nil, errors.New("transcript symlink is not allowed")
		}
		return root, filepath.ToSlash(relative), info, nil
	}
	if lastErr != nil {
		return nil, "", nil, lastErr
	}
	return nil, "", nil, os.ErrNotExist
}
