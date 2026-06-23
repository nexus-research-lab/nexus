package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// UpdateFile 更新 workspace 文件内容。
func (s *Service) UpdateFile(ctx context.Context, agentID string, relativePath string, content string) (*FileContent, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	targetPath, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, err
	}
	if s.live != nil {
		s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
	}
	if err = os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return nil, err
	}
	if s.live != nil {
		s.live.EmitAPIWrite(agentValue.AgentID, normalizedPath, content)
	}
	return &FileContent{Path: normalizedPath, Content: content}, nil
}

// CreateEntry 创建文件或目录。
func (s *Service) CreateEntry(ctx context.Context, agentID string, relativePath string, entryType string, content string) (*EntryMutationResponse, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	targetPath, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(targetPath); err == nil {
		return nil, errors.New("目标已存在")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	switch strings.TrimSpace(entryType) {
	case "directory":
		err = os.MkdirAll(targetPath, 0o755)
	case "file":
		if s.live != nil {
			s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
		}
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return nil, err
		}
		err = os.WriteFile(targetPath, []byte(content), 0o644)
	default:
		return nil, errors.New("仅支持创建 file 或 directory")
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(entryType) == "file" && s.live != nil {
		s.live.EmitAPIWrite(agentValue.AgentID, normalizedPath, content)
	}
	return &EntryMutationResponse{Path: normalizedPath}, nil
}

// RenameEntry 重命名 workspace 条目。
func (s *Service) RenameEntry(ctx context.Context, agentID string, relativePath string, newPath string) (*EntryRenameResponse, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	sourcePath, normalizedSource, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, err
	}
	targetPath, normalizedTarget, err := resolveWorkspacePath(agentValue.WorkspacePath, newPath)
	if err != nil {
		return nil, err
	}
	if normalizedSource == normalizedTarget {
		return nil, errors.New("新旧路径不能相同")
	}
	sourceInfo, err := os.Stat(sourcePath)
	if os.IsNotExist(err) {
		return nil, ErrFileNotFound
	} else if err != nil {
		return nil, err
	}
	if _, err = os.Stat(targetPath); err == nil {
		return nil, errors.New("目标已存在")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	var fileContent *string
	if sourceInfo != nil && !sourceInfo.IsDir() {
		content, readErr := os.ReadFile(sourcePath)
		if readErr == nil {
			text := string(content)
			fileContent = &text
		}
	}
	if s.live != nil && sourceInfo != nil && !sourceInfo.IsDir() {
		s.live.SuppressWatcher(agentValue.AgentID, normalizedSource)
		s.live.SuppressWatcher(agentValue.AgentID, normalizedTarget)
	}
	if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, err
	}
	if err = os.Rename(sourcePath, targetPath); err != nil {
		return nil, err
	}
	if s.live != nil && sourceInfo != nil && !sourceInfo.IsDir() {
		s.live.EmitAPIDelete(agentValue.AgentID, normalizedSource)
		if fileContent != nil {
			s.live.EmitAPIWrite(agentValue.AgentID, normalizedTarget, *fileContent)
		}
	}
	return &EntryRenameResponse{
		Path:    normalizedSource,
		NewPath: normalizedTarget,
	}, nil
}

// DeleteEntry 删除 workspace 条目。
func (s *Service) DeleteEntry(ctx context.Context, agentID string, relativePath string) (*EntryMutationResponse, error) {
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
	if s.live != nil && info != nil && !info.IsDir() {
		s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
	}
	if info.IsDir() {
		err = os.RemoveAll(targetPath)
	} else {
		err = os.Remove(targetPath)
	}
	if err != nil {
		return nil, err
	}
	if s.live != nil && info != nil && !info.IsDir() {
		s.live.EmitAPIDelete(agentValue.AgentID, normalizedPath)
	}
	return &EntryMutationResponse{Path: normalizedPath}, nil
}
