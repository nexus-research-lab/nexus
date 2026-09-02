// INPUT: 已验证 Agent、workspace 路径、正文与可选的读取基线 revision。
// OUTPUT: 原子文件修改、稳定新 revision，或可证明未落盘的 revision conflict。
// POS: workspace 文件修改边界；Memory 等并发敏感客户端在此执行条件写入。
package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

// UpdateFile 更新 workspace 文件内容。
func (s *Service) UpdateFile(ctx context.Context, agentID string, relativePath string, content string) (*FileContent, error) {
	return s.UpdateFileIfRevision(ctx, agentID, relativePath, content, nil)
}

// UpdateFileIfRevision 更新 workspace 文件内容。
//
// expectedRevision 为 nil 时保留旧客户端的无条件写入语义；非 nil 时只有当前正文
// 与读取基线完全一致才提交。比较与 API 写入由短分片锁串行化，外部 runtime 在比较
// 前已经完成的写入会稳定返回 ErrFileRevisionConflict，调用方不得自动重放。
func (s *Service) UpdateFileIfRevision(
	ctx context.Context,
	agentID string,
	relativePath string,
	content string,
	expectedRevision *string,
) (*FileContent, error) {
	if int64(len(content)) > workspaceWholeFileMaxBytes {
		return nil, ErrFileTooLarge
	}
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	_, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, invalidWorkspaceMutation(err)
	}
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	unlock := s.lockFileMutation(agentValue.AgentID, normalizedPath)
	defer unlock()
	var expectedContent []byte
	if expectedRevision != nil {
		currentContent, readErr := readWorkspaceWholeFile(confinedRoot, normalizedPath)
		if errors.Is(readErr, ErrFileNotFound) {
			return nil, ErrFileRevisionConflict
		}
		if readErr != nil {
			return nil, readErr
		}
		if workspaceFileRevision(currentContent) != strings.TrimSpace(*expectedRevision) {
			return nil, ErrFileRevisionConflict
		}
		expectedContent = currentContent
	}
	if err = confinedRoot.MkdirAll(filepath.Dir(normalizedPath), workspaceDirectoryMode()); err != nil {
		return nil, err
	}
	if expectedRevision == nil {
		if s.live != nil {
			s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
		}
		if err = confinedRoot.WriteFileAtomic(normalizedPath, []byte(content), workspaceFileMode()); err != nil {
			return nil, err
		}
	} else {
		var committed bool
		committed, err = confinedRoot.WriteFileAtomicIfContent(
			normalizedPath,
			[]byte(content),
			expectedContent,
			workspaceFileMode(),
		)
		if err != nil {
			return nil, err
		}
		if !committed {
			return nil, ErrFileRevisionConflict
		}
	}
	if s.live != nil {
		s.live.EmitAPIWrite(agentValue.AgentID, normalizedPath, content)
	}
	return &FileContent{
		Path:     normalizedPath,
		Content:  content,
		Revision: workspaceFileRevision([]byte(content)),
	}, nil
}

func workspaceFileRevision(content []byte) string {
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// CreateEntry 创建文件或目录。
func (s *Service) CreateEntry(ctx context.Context, agentID string, relativePath string, entryType string, content string) (*EntryMutationResponse, error) {
	entryType = strings.TrimSpace(entryType)
	if entryType == "file" && int64(len(content)) > workspaceWholeFileMaxBytes {
		return nil, ErrFileTooLarge
	}
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	_, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, invalidWorkspaceMutation(err)
	}
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	if _, err = confinedRoot.Lstat(normalizedPath); err == nil {
		return nil, invalidWorkspaceMutation(errors.New("目标已存在"))
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	switch entryType {
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
		return nil, invalidWorkspaceMutation(errors.New("仅支持创建 file 或 directory"))
	}
	if err != nil {
		return nil, err
	}
	if entryType == "file" && s.live != nil {
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
		return nil, invalidWorkspaceMutation(err)
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
		return invalidWorkspaceMutation(errors.New("新旧路径不能相同"))
	}
	info, err := r.confinedRoot.Lstat(r.normalizedSource)
	if os.IsNotExist(err) {
		return ErrFileNotFound
	}
	if err != nil {
		return err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return invalidWorkspaceMutation(errors.New("只能重命名普通文件或目录"))
	}
	r.sourceInfo = info
	if _, err = r.confinedRoot.Lstat(r.normalizedTarget); err == nil {
		return invalidWorkspaceMutation(errors.New("目标已存在"))
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
	content, err := readWorkspaceWholeFile(r.confinedRoot, r.normalizedSource)
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
		return nil, invalidWorkspaceMutation(err)
	}
	if strings.EqualFold(normalizedPath, memoryEntrypointName) {
		return nil, invalidWorkspaceMutation(errors.New("MEMORY.md 是记忆索引，不能通过通用文件接口删除"))
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
	if strings.EqualFold(normalizedPath, memoryDirectoryName) {
		return nil, invalidWorkspaceMutation(errors.New("记忆目录不能整体删除，请逐个删除记忆正文"))
	}
	if strings.HasPrefix(strings.ToLower(normalizedPath), memoryDirectoryName+"/") {
		if info.IsDir() {
			return nil, invalidWorkspaceMutation(errors.New("记忆目录不能整体删除，请逐个删除记忆正文"))
		}
		return s.DeleteMemoryDocument(ctx, agentID, normalizedPath)
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
