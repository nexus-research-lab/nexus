package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// UpdateFile 更新 workspace 文件内容。
func (s *Service) UpdateFile(ctx context.Context, agentID string, relativePath string, content string) (*FileContent, error) {
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
	if err = confinedRoot.MkdirAll(filepath.Dir(normalizedPath), workspaceDirectoryMode()); err != nil {
		return nil, err
	}
	if s.live != nil {
		s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
	}
	if err = confinedRoot.WriteFileAtomic(normalizedPath, []byte(content), workspaceFileMode()); err != nil {
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
	_, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, err
	}
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	if _, err = confinedRoot.Lstat(normalizedPath); err == nil {
		return nil, errors.New("目标已存在")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	switch strings.TrimSpace(entryType) {
	case "directory":
		err = confinedRoot.MkdirAll(normalizedPath, workspaceDirectoryMode())
	case "file":
		if s.live != nil {
			s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
		}
		if err = confinedRoot.MkdirAll(filepath.Dir(normalizedPath), workspaceDirectoryMode()); err != nil {
			return nil, err
		}
		err = confinedRoot.WriteFileAtomic(normalizedPath, []byte(content), workspaceFileMode())
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
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	rename := workspaceEntryRename{
		service:       s,
		agentID:       agentValue.AgentID,
		workspacePath: agentValue.WorkspacePath,
		confinedRoot:  confinedRoot,
	}
	return rename.run(relativePath, newPath)
}

type workspaceEntryRename struct {
	service          *Service
	agentID          string
	workspacePath    string
	confinedRoot     *confinedfs.Root
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
	if r.confinedRoot == nil {
		return nil, errors.New("workspace root is unavailable")
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
	info, err := r.confinedRoot.Lstat(r.normalizedSource)
	if os.IsNotExist(err) {
		return ErrFileNotFound
	}
	if err != nil {
		return err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return errors.New("只能重命名普通文件或目录")
	}
	r.sourceInfo = info
	if _, err = r.confinedRoot.Lstat(r.normalizedTarget); err == nil {
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
	content, err := r.confinedRoot.ReadFile(r.normalizedSource)
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
	if err := r.confinedRoot.MkdirAll(filepath.Dir(r.normalizedTarget), workspaceDirectoryMode()); err != nil {
		return err
	}
	return r.confinedRoot.Rename(r.normalizedSource, r.normalizedTarget)
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
	return r.sourceInfo != nil && r.sourceInfo.Mode().IsRegular()
}

// DeleteEntry 删除 workspace 条目。
func (s *Service) DeleteEntry(ctx context.Context, agentID string, relativePath string) (*EntryMutationResponse, error) {
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
	info, err := confinedRoot.Lstat(normalizedPath)
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
		err = confinedRoot.RemoveAll(normalizedPath)
	} else {
		err = confinedRoot.Remove(normalizedPath)
	}
	if err != nil {
		return nil, err
	}
	if s.live != nil && info != nil && !info.IsDir() {
		s.live.EmitAPIDelete(agentValue.AgentID, normalizedPath)
	}
	return &EntryMutationResponse{Path: normalizedPath}, nil
}
