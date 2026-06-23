package workspace

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const maxRawPreviewSize = 5 * 1024 * 1024

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

// GetRawFile 返回舞台内联预览所需的真实文件信息。
func (s *Service) GetRawFile(ctx context.Context, agentID string, relativePath string) (*RawFile, error) {
	rawFile, err := s.resolveRawFile(ctx, agentID, relativePath)
	if err != nil {
		return nil, err
	}
	if rawFile.Size > maxRawPreviewSize {
		return nil, ErrFileTooLarge
	}
	return rawFile, nil
}

// GetFileMeta 返回工作区文件预览元信息。
func (s *Service) GetFileMeta(ctx context.Context, agentID string, relativePath string) (*FileMeta, error) {
	rawFile, err := s.resolveRawFile(ctx, agentID, relativePath)
	if err != nil {
		return nil, err
	}
	return &FileMeta{
		Path:         rawFile.Path,
		Name:         rawFile.Name,
		Size:         rawFile.Size,
		ModifiedAt:   rawFile.ModifiedAt.Format(time.RFC3339Nano),
		ContentType:  rawFile.ContentType,
		ETag:         rawFile.ETag,
		RawAvailable: rawFile.Size <= maxRawPreviewSize,
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

func (s *Service) resolveRawFile(ctx context.Context, agentID string, relativePath string) (*RawFile, error) {
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
		return nil, errors.New("不能预览目录")
	}
	return &RawFile{
		Path:        normalizedPath,
		FilePath:    targetPath,
		Name:        filepath.Base(normalizedPath),
		Size:        info.Size(),
		ModifiedAt:  info.ModTime(),
		ContentType: detectWorkspaceContentType(normalizedPath),
		ETag:        buildWorkspaceFileETag(info),
	}, nil
}

func buildWorkspaceFileETag(info os.FileInfo) string {
	return fmt.Sprintf(`W/"%x-%x"`, info.Size(), info.ModTime().UnixNano())
}

func detectWorkspaceContentType(relativePath string) string {
	extension := strings.ToLower(filepath.Ext(relativePath))
	switch extension {
	case ".html", ".htm", ".xhtml":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs", ".cjs":
		return "text/javascript; charset=utf-8"
	case ".json", ".jsonl", ".map":
		return "application/json; charset=utf-8"
	case ".md", ".markdown", ".txt", ".log", ".csv":
		return "text/plain; charset=utf-8"
	}
	if detected := mime.TypeByExtension(extension); detected != "" {
		return detected
	}
	return "application/octet-stream"
}
