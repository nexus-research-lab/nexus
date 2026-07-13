package workspace

import (
	"context"
	"errors"
	"io/fs"
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
	rename := workspaceEntryRename{
		service:       s,
		agentID:       agentValue.AgentID,
		workspacePath: agentValue.WorkspacePath,
	}
	return rename.run(relativePath, newPath)
}

type workspaceEntryRename struct {
	service          *Service
	agentID          string
	workspacePath    string
	sourcePath       string
	targetPath       string
	normalizedSource string
	normalizedTarget string
	sourceInfo       fs.FileInfo
	fileContent      *string
}

func (r *workspaceEntryRename) run(relativePath string, newPath string) (*EntryRenameResponse, error) {
	if err := r.resolvePaths(relativePath, newPath); err != nil {
		return nil, err
	}
	if err := r.validateMove(); err != nil {
		return nil, err
	}
	r.captureFileContent()
	r.suppressFileWatchers()
	if err := r.move(); err != nil {
		return nil, err
	}
	r.emitFileMove()
	return &EntryRenameResponse{Path: r.normalizedSource, NewPath: r.normalizedTarget}, nil
}

func (r *workspaceEntryRename) resolvePaths(relativePath string, newPath string) error {
	var err error
	r.sourcePath, r.normalizedSource, err = resolveWorkspacePath(r.workspacePath, relativePath)
	if err != nil {
		return err
	}
	r.targetPath, r.normalizedTarget, err = resolveWorkspacePath(r.workspacePath, newPath)
	return err
}

func (r *workspaceEntryRename) validateMove() error {
	if r.normalizedSource == r.normalizedTarget {
		return errors.New("新旧路径不能相同")
	}
	info, err := os.Stat(r.sourcePath)
	if os.IsNotExist(err) {
		return ErrFileNotFound
	}
	if err != nil {
		return err
	}
	r.sourceInfo = info
	if _, err = os.Stat(r.targetPath); err == nil {
		return errors.New("目标已存在")
	}
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (r *workspaceEntryRename) captureFileContent() {
	if !r.isFile() {
		return
	}
	content, err := os.ReadFile(r.sourcePath)
	if err != nil {
		return
	}
	text := string(content)
	r.fileContent = &text
}

func (r *workspaceEntryRename) suppressFileWatchers() {
	if r.service.live == nil || !r.isFile() {
		return
	}
	r.service.live.SuppressWatcher(r.agentID, r.normalizedSource)
	r.service.live.SuppressWatcher(r.agentID, r.normalizedTarget)
}

func (r *workspaceEntryRename) move() error {
	if err := os.MkdirAll(filepath.Dir(r.targetPath), 0o755); err != nil {
		return err
	}
	return os.Rename(r.sourcePath, r.targetPath)
}

func (r *workspaceEntryRename) emitFileMove() {
	if r.service.live == nil || !r.isFile() {
		return
	}
	r.service.live.EmitAPIDelete(r.agentID, r.normalizedSource)
	if r.fileContent != nil {
		r.service.live.EmitAPIWrite(r.agentID, r.normalizedTarget, *r.fileContent)
	}
}

func (r *workspaceEntryRename) isFile() bool {
	return r.sourceInfo != nil && !r.sourceInfo.IsDir()
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
