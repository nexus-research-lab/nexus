package workspace

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

type transcriptRootCandidate struct {
	path string
	root *confinedfs.Root
}

// openTranscriptEntry 将 transcript 路径绑定到 workspace 对应的 projects 根，
// 或绑定到同一 workspace 的显式运行输出根。中间目录必须是真实目录，调用方
// 只需决定是否接受最终条目的符号链接。
func openTranscriptEntry(workspacePath string, transcriptPath string) (*confinedfs.Root, string, os.FileInfo, error) {
	candidatePaths := []string{
		transcriptProjectsDirForWorkspace(workspacePath),
		filepath.Clean(workspacePath),
	}
	candidates := make([]transcriptRootCandidate, 0, len(candidatePaths))
	var lastOpenErr error
	for _, candidatePath := range candidatePaths {
		candidatePath = filepath.Clean(strings.TrimSpace(candidatePath))
		if candidatePath == "" || candidatePath == "." {
			continue
		}
		root, err := confinedfs.Open(candidatePath)
		if err != nil {
			lastOpenErr = err
			continue
		}
		defer root.Close()
		candidates = append(candidates, transcriptRootCandidate{
			path: candidatePath,
			root: root,
		})
	}
	root, relative, info, err := openTranscriptEntryAt(candidates, transcriptPath)
	if errors.Is(err, os.ErrNotExist) && lastOpenErr != nil {
		return nil, "", nil, lastOpenErr
	}
	return root, relative, info, err
}

func openTranscriptEntryAt(
	candidates []transcriptRootCandidate,
	transcriptPath string,
) (*confinedfs.Root, string, os.FileInfo, error) {
	transcriptPath = filepath.Clean(strings.TrimSpace(transcriptPath))
	if transcriptPath == "" || transcriptPath == "." {
		return nil, "", nil, os.ErrNotExist
	}
	var lastErr error
	for _, candidate := range candidates {
		candidate.path = filepath.Clean(strings.TrimSpace(candidate.path))
		if candidate.root == nil || candidate.path == "" || candidate.path == "." {
			continue
		}
		relative, err := filepath.Rel(candidate.path, transcriptPath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		relative = filepath.ToSlash(relative)
		parent, openErr := candidate.root.OpenRootNoSymlink(path.Dir(relative))
		if openErr != nil {
			lastErr = openErr
			continue
		}
		name := path.Base(relative)
		info, statErr := parent.Lstat(name)
		if statErr != nil {
			parent.Close()
			if errors.Is(statErr, os.ErrNotExist) {
				lastErr = statErr
				continue
			}
			return nil, "", nil, statErr
		}
		return parent, name, info, nil
	}
	if lastErr != nil {
		return nil, "", nil, lastErr
	}
	return nil, "", nil, os.ErrNotExist
}

func (s *AgentHistoryStore) openOwnerTranscriptCandidates(
	ownerUserID string,
	workspacePath string,
) ([]transcriptRootCandidate, func(), error) {
	workspacePath = filepath.Clean(strings.TrimSpace(workspacePath))
	workspaceRoot, workspaceErr := s.paths.OpenOwnerWorkspacePath(
		ownerUserID,
		workspacePath,
		false,
	)
	if workspaceErr != nil && !errors.Is(workspaceErr, os.ErrNotExist) {
		return nil, nil, workspaceErr
	}
	if workspaceErr != nil &&
		!s.paths.workspacePathBelongsToOwner(ownerUserID, workspacePath) {
		return nil, nil, errors.New("workspace path does not belong to owner")
	}

	projectsRoot, projectsErr := s.paths.openOwnerTranscriptProjectsRoot(
		ownerUserID,
		false,
	)
	if projectsErr != nil && !errors.Is(projectsErr, os.ErrNotExist) {
		if workspaceRoot != nil {
			workspaceRoot.Close()
		}
		return nil, nil, projectsErr
	}

	candidates := make([]transcriptRootCandidate, 0, 2)
	if projectsRoot != nil {
		candidates = append(candidates, transcriptRootCandidate{
			path: projectsRoot.Name(),
			root: projectsRoot,
		})
	}
	if workspaceRoot != nil {
		candidates = append(candidates, transcriptRootCandidate{
			path: workspacePath,
			root: workspaceRoot,
		})
	}
	closeRoots := func() {
		if projectsRoot != nil {
			projectsRoot.Close()
		}
		if workspaceRoot != nil {
			workspaceRoot.Close()
		}
	}
	return candidates, closeRoots, nil
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

func openTranscriptPathAt(
	candidates []transcriptRootCandidate,
	transcriptPath string,
) (*confinedfs.Root, string, os.FileInfo, error) {
	root, relative, info, err := openTranscriptEntryAt(candidates, transcriptPath)
	if err != nil {
		return nil, "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		root.Close()
		return nil, "", nil, errors.New("transcript symlink is not allowed")
	}
	return root, relative, info, nil
}

func (s *AgentHistoryStore) openTranscriptPath(
	workspacePath string,
	transcriptPath string,
) (*confinedfs.Root, string, os.FileInfo, error) {
	if strings.TrimSpace(s.ownerUserID) == "" {
		return openTranscriptPath(workspacePath, transcriptPath)
	}
	candidates, closeRoots, err := s.openOwnerTranscriptCandidates(
		s.ownerUserID,
		workspacePath,
	)
	if err != nil {
		return nil, "", nil, err
	}
	root, relative, info, err := openTranscriptPathAt(candidates, transcriptPath)
	closeRoots()
	return root, relative, info, err
}

// resolveTranscriptLinkTarget 只读取授权根内的链接本身。返回的目标仍需再次经过
// openTranscriptPath 校验，因此相对链接、绝对链接和链接链都不能逃逸授权根。
func resolveTranscriptLinkTarget(workspacePath string, transcriptPath string) (string, error) {
	root, relative, info, err := openTranscriptEntry(workspacePath, transcriptPath)
	return resolveTranscriptLinkTargetAtEntry(root, relative, info, transcriptPath, err)
}

func resolveTranscriptLinkTargetAt(
	candidates []transcriptRootCandidate,
	transcriptPath string,
) (string, error) {
	root, relative, info, err := openTranscriptEntryAt(candidates, transcriptPath)
	return resolveTranscriptLinkTargetAtEntry(root, relative, info, transcriptPath, err)
}

func (s *AgentHistoryStore) resolveTranscriptLinkTarget(
	workspacePath string,
	transcriptPath string,
) (string, error) {
	if strings.TrimSpace(s.ownerUserID) == "" {
		return resolveTranscriptLinkTarget(workspacePath, transcriptPath)
	}
	candidates, closeRoots, err := s.openOwnerTranscriptCandidates(
		s.ownerUserID,
		workspacePath,
	)
	if err != nil {
		return "", err
	}
	defer closeRoots()
	return resolveTranscriptLinkTargetAt(candidates, transcriptPath)
}

func resolveTranscriptLinkTargetAtEntry(
	root *confinedfs.Root,
	relative string,
	info os.FileInfo,
	transcriptPath string,
	err error,
) (string, error) {
	if err != nil {
		return "", err
	}
	defer root.Close()
	transcriptPath = filepath.Clean(strings.TrimSpace(transcriptPath))
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
