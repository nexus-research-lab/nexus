package workspace

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ListFiles 返回 Agent workspace 的文件树。
func (s *Service) ListFiles(ctx context.Context, agentID string) ([]FileEntry, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	entries := make([]FileEntry, 0, 32)
	root := filepath.Clean(agentValue.WorkspacePath)
	if err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalizedPath := filepath.ToSlash(relativePath)
		if shouldHideWorkspaceEntry(normalizedPath) {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entry := FileEntry{
			Path:       normalizedPath,
			Name:       info.Name(),
			IsDir:      info.IsDir(),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
			Depth:      len(strings.Split(normalizedPath, "/")),
		}
		if !entry.IsDir {
			size := info.Size()
			entry.Size = &size
		}
		entries = append(entries, entry)
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
	targetPath, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("不能直接读取目录")
	}
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}
	return &FileContent{
		Path:    normalizedPath,
		Content: string(content),
	}, nil
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
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return "", "", ErrFileNotFound
	}
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", errors.New("不能下载目录")
	}
	return targetPath, filepath.Base(normalizedPath), nil
}
