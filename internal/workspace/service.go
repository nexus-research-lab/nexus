// =====================================================
// @File   ：service.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
)

var (
	// ErrFileNotFound 表示 workspace 文件不存在。
	ErrFileNotFound = errors.New("workspace file not found")
)

const maxUploadSize = 20 * 1024 * 1024

var textExtensions = map[string]struct{}{
	"txt": {}, "md": {}, "markdown": {}, "json": {}, "jsonl": {}, "yaml": {}, "yml": {}, "toml": {}, "xml": {},
	"csv": {}, "ts": {}, "tsx": {}, "js": {}, "jsx": {}, "mjs": {}, "cjs": {}, "py": {}, "java": {}, "go": {},
	"rs": {}, "rb": {}, "php": {}, "sh": {}, "bash": {}, "zsh": {}, "sql": {}, "html": {}, "css": {}, "scss": {},
	"less": {}, "log": {}, "ini": {}, "conf": {}, "env": {}, "dockerfile": {}, "makefile": {}, "cmake": {},
	"gradle": {}, "proto": {}, "graphql": {}, "svg": {}, "rst": {}, "adoc": {},
}

// FileEntry 表示 workspace 文件树条目。
type FileEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	IsDir      bool   `json:"is_dir"`
	Size       *int64 `json:"size,omitempty"`
	ModifiedAt string `json:"modified_at"`
	Depth      int    `json:"depth"`
}

// FileContent 表示 workspace 文件内容。
type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// EntryMutationResponse 表示创建/删除返回。
type EntryMutationResponse struct {
	Path string `json:"path"`
}

// EntryRenameResponse 表示重命名返回。
type EntryRenameResponse struct {
	Path    string `json:"path"`
	NewPath string `json:"new_path"`
}

// UploadResult 表示上传文件结果。
type UploadResult struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Service 提供 workspace 文件读写能力。
type Service struct {
	config config.Config
	agents *agent2.Service
	live   *liveManager
}

// NewService 创建 workspace 服务。
func NewService(cfg config.Config, agents *agent2.Service) *Service {
	return &Service{
		config: cfg,
		agents: agents,
		live:   newLiveManager(),
	}
}

// SubscribeLive 订阅指定 Agent 的 workspace 实时事件。
func (s *Service) SubscribeLive(ctx context.Context, agentID string, listener LiveListener) (string, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return "", err
	}
	return s.live.Subscribe(agentValue.AgentID, agentValue.WorkspacePath, listener)
}

// UnsubscribeLive 取消某个 workspace 实时订阅。
func (s *Service) UnsubscribeLive(token string) {
	if s.live == nil {
		return
	}
	s.live.Unsubscribe(token)
}

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
	sort.Slice(entries, func(i int, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Path < entries[j].Path
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
	if _, err = os.Stat(sourcePath); os.IsNotExist(err) {
		return nil, ErrFileNotFound
	} else if err != nil {
		return nil, err
	}
	if _, err = os.Stat(targetPath); err == nil {
		return nil, errors.New("目标已存在")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	sourceInfo, err := os.Stat(sourcePath)
	if os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}
	if err != nil {
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

// UploadFile 上传单个文件到 workspace。
func (s *Service) UploadFile(ctx context.Context, agentID string, filename string, destination string, reader io.Reader) (*UploadResult, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	safeName := normalizeUploadName(filename)
	if safeName == "" {
		safeName = "uploaded_file"
	}
	content, err := io.ReadAll(io.LimitReader(reader, maxUploadSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) > maxUploadSize {
		return nil, errors.New("文件大小超过限制 (20MB)")
	}

	relativePath := buildUploadTargetPath(strings.TrimSpace(destination), safeName)
	targetPath, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, err
	}
	if normalizedPath, targetPath, err = ensureUniqueWorkspaceFile(targetPath, normalizedPath); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, err
	}
	if s.live != nil {
		s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
	}
	if err = os.WriteFile(targetPath, content, 0o644); err != nil {
		return nil, err
	}
	if s.live != nil {
		if snapshot, ok := tryDecodeTextSnapshot(normalizedPath, content); ok {
			s.live.EmitAPIWrite(agentValue.AgentID, normalizedPath, snapshot)
		}
	}
	return &UploadResult{
		Path: normalizedPath,
		Name: filepath.Base(normalizedPath),
		Size: int64(len(content)),
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

func (s *Service) ensureAgentWorkspace(ctx context.Context, agentID string) (*agent2.Agent, error) {
	agentValue, err := s.agents.GetAgent(ctx, strings.TrimSpace(agentID))
	if err != nil {
		return nil, err
	}
	if err = EnsureInitialized(
		agentValue.AgentID,
		agentValue.Name,
		agentValue.WorkspacePath,
		agentValue.IsMain,
		agentValue.CreatedAt,
	); err != nil {
		return nil, err
	}
	return agentValue, nil
}

func resolveWorkspacePath(workspacePath string, relativePath string) (string, string, error) {
	root := filepath.Clean(workspacePath)
	normalizedPath := strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/"))
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	if normalizedPath == "" {
		return "", "", errors.New("文件路径不能为空")
	}
	if isProtectedWorkspacePath(normalizedPath) {
		return "", "", errors.New("不能直接操作内部运行时目录")
	}
	targetPath := filepath.Clean(filepath.Join(root, normalizedPath))
	rootWithSeparator := root + string(os.PathSeparator)
	if targetPath != root && !strings.HasPrefix(targetPath, rootWithSeparator) {
		return "", "", errors.New("文件路径超出 workspace 范围")
	}
	return targetPath, filepath.ToSlash(normalizedPath), nil
}

func shouldHideWorkspaceEntry(relativePath string) bool {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(relativePath))
	return normalizedPath == ".agents" ||
		strings.HasPrefix(normalizedPath, ".agents/") ||
		normalizedPath == ".git" ||
		strings.HasPrefix(normalizedPath, ".git/") ||
		normalizedPath == ".claude" ||
		strings.HasPrefix(normalizedPath, ".claude/") ||
		normalizedPath == "__pycache__" ||
		strings.HasPrefix(normalizedPath, "__pycache__/") ||
		strings.HasPrefix(filepath.Base(normalizedPath), ".DS_")
}

func isProtectedWorkspacePath(relativePath string) bool {
	normalizedPath := filepath.ToSlash(strings.TrimSpace(relativePath))
	protectedRoots := []string{".agents", ".claude", ".git", "__pycache__"}
	for _, root := range protectedRoots {
		if normalizedPath == root || strings.HasPrefix(normalizedPath, root+"/") {
			return true
		}
	}
	return false
}

func normalizeUploadName(filename string) string {
	raw := strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func buildUploadTargetPath(destination string, filename string) string {
	target := strings.TrimSpace(strings.ReplaceAll(destination, "\\", "/"))
	target = strings.TrimPrefix(target, "/")
	if target == "" {
		return filename
	}
	if strings.HasSuffix(target, "/") {
		return target + filename
	}
	lowerBase := strings.ToLower(filepath.Base(target))
	if strings.Contains(lowerBase, ".") {
		return target
	}
	return target + "/" + filename
}

func ensureUniqueWorkspaceFile(targetPath string, normalizedPath string) (string, string, error) {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return normalizedPath, targetPath, nil
	} else if err != nil {
		return "", "", err
	}
	extension := filepath.Ext(normalizedPath)
	base := strings.TrimSuffix(filepath.Base(normalizedPath), extension)
	parent := filepath.ToSlash(filepath.Dir(normalizedPath))
	timestamp := time.Now().Format("20060102-150405")
	nextName := base + "-" + timestamp + extension
	if parent == "." || parent == "" {
		return nextName, filepath.Join(filepath.Dir(targetPath), nextName), nil
	}
	nextPath := parent + "/" + nextName
	return nextPath, filepath.Join(filepath.Dir(targetPath), nextName), nil
}

func tryDecodeTextSnapshot(path string, content []byte) (string, bool) {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if _, ok := textExtensions[extension]; ok {
		return string(content), true
	}
	if utf8Text(content) {
		return string(content), true
	}
	return "", false
}

func utf8Text(content []byte) bool {
	for len(content) > 0 {
		if content[0] == 0 {
			return false
		}
		if content[0] < 0x80 {
			content = content[1:]
			continue
		}
		_, size := utf8.DecodeRune(content)
		if size == 1 {
			return false
		}
		content = content[size:]
	}
	return true
}
