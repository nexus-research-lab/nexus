// INPUT: 已验证 Agent、workspace 相对路径与 confined-fd 文件读取。
// OUTPUT: 文件树或带稳定正文 revision 的文件内容。
// POS: workspace 读取边界；revision 由正文计算，不依赖时间戳或 inode。
package workspace

import (
	"cmp"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const workspaceWholeFileMaxBytes int64 = 4 * 1024 * 1024

// ListFiles 返回 Agent workspace 的文件树。
func (s *Service) ListFiles(ctx context.Context, agentID string) ([]FileEntry, error) {
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	entries := make([]FileEntry, 0, 32)
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	if err = confinedRoot.Walk(".", func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return handleWorkspaceWalkError(path, dirEntry, walkErr)
		}
		if path == "." {
			return nil
		}
		if dirEntry == nil {
			return nil
		}
		// 不把符号链接当作目录继续展开；Root 已阻止其越出根，
		// 但浏览器仍应只展示真实 workspace 节点。
		if dirEntry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := dirEntry.Info()
		if err != nil {
			return handleWorkspaceWalkError(path, dirEntry, err)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		normalizedPath := filepath.ToSlash(path)
		if shouldHideWorkspaceBrowserEntry(normalizedPath) {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldHideWorkspaceEntry(normalizedPath) {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		fileEntry := FileEntry{
			Path:       normalizedPath,
			Name:       info.Name(),
			IsDir:      info.IsDir(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
			Depth:      len(strings.Split(normalizedPath, "/")),
		}
		if !fileEntry.IsDir {
			size := info.Size()
			fileEntry.Size = &size
		}
		entries = append(entries, fileEntry)
		return nil
	}); err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(left FileEntry, right FileEntry) int {
		if left.IsDir != right.IsDir {
			if left.IsDir {
				return -1
			}
			return 1
		}
		return cmp.Compare(left.Path, right.Path)
	})
	return entries, nil
}

// GetFile 读取 workspace 文件。
func (s *Service) GetFile(ctx context.Context, agentID string, relativePath string) (*FileContent, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	_, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, err
	}
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	content, err := readWorkspaceWholeFile(confinedRoot, normalizedPath)
	if err != nil {
		return nil, err
	}
	return &FileContent{
		Path:     normalizedPath,
		Content:  string(content),
		Revision: workspaceFileRevision(content),
	}, nil
}

// readWorkspaceWholeFile 为所有需要单个正文载荷的入口提供同一内存上限。
func readWorkspaceWholeFile(root *confinedfs.Root, relativePath string) ([]byte, error) {
	file, err := root.OpenFileNoSymlink(relativePath, os.O_RDONLY, 0)
	if os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		if info.IsDir() {
			return nil, errors.New("不能直接读取目录")
		}
		return nil, errors.New("只能读取普通文件")
	}
	if info.Size() > workspaceWholeFileMaxBytes {
		return nil, ErrFileTooLarge
	}
	content, err := io.ReadAll(io.LimitReader(file, workspaceWholeFileMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > workspaceWholeFileMaxBytes {
		return nil, ErrFileTooLarge
	}
	return content, nil
}

// GetFileForDownload 返回下载所需的真实文件路径和文件名。
func (s *Service) GetFileForDownload(ctx context.Context, agentID string, relativePath string) (string, string, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return "", "", err
	}
	targetPath, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return "", "", err
	}
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return "", "", err
	}
	defer confinedRoot.Close()
	info, err := confinedRoot.Stat(normalizedPath)
	if os.IsNotExist(err) {
		return "", "", ErrFileNotFound
	}
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() {
		if info.IsDir() {
			return "", "", errors.New("不能下载目录")
		}
		return "", "", errors.New("只能下载普通文件")
	}
	return targetPath, filepath.Base(normalizedPath), nil
}

// OpenFileForDownload 在完成 owner/workspace 校验后返回已打开的文件。
//
// handler 必须消费并关闭返回的 fd，不应再根据返回路径调用 http.ServeFile；
// 这样下载期间即使 workspace 发生 rename 或 symlink 替换，读取的仍是已校验 inode。
func (s *Service) OpenFileForDownload(ctx context.Context, agentID string, relativePath string) (*os.File, string, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, "", err
	}
	_, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, "", err
	}
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, "", err
	}
	file, err := confinedRoot.OpenFileNoSymlink(normalizedPath, os.O_RDONLY, 0)
	if err != nil {
		confinedRoot.Close()
		if os.IsNotExist(err) {
			return nil, "", ErrFileNotFound
		}
		return nil, "", err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		confinedRoot.Close()
		return nil, "", err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		confinedRoot.Close()
		if info.IsDir() {
			return nil, "", errors.New("不能下载目录")
		}
		return nil, "", errors.New("只能下载普通文件")
	}
	// 文件已经由根 fd 校验并打开；后续 HTTP 读取直接使用文件 fd，
	// 因此可以释放目录根而不重新解析用户可控路径。
	_ = confinedRoot.Close()
	return file, filepath.Base(normalizedPath), nil
}
