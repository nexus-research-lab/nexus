package workspace

import (
	"bufio"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const (
	memoryEntrypointName      = "MEMORY.md"
	memoryDirectoryName       = "memory"
	memoryDocumentLimit       = 200
	memoryFrontmatterMaxLines = 30
)

var memoryIndexLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+\.md)(?:#[^)]*)?\)`)

// MemorySnapshot 是 SDK 文件式记忆在 Agent workspace 中的只读投影。
type MemorySnapshot struct {
	Documents []MemoryDocument `json:"documents"`
	Index     *MemoryDocument  `json:"index,omitempty"`
	Layout    string           `json:"layout"`
	Truncated bool             `json:"truncated"`
}

// MemoryDocument 描述一个可视化记忆文件，不承载正文内容。
type MemoryDocument struct {
	Description string `json:"description,omitempty"`
	Indexed     bool   `json:"indexed"`
	Kind        string `json:"kind"`
	ModifiedAt  string `json:"modified_at"`
	Name        string `json:"name,omitempty"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	Title       string `json:"title"`
	Type        string `json:"type,omitempty"`
}

// GetMemorySnapshot 读取 SDK 管理的记忆文件布局，不参与记忆写入或召回。
func (s *Service) GetMemorySnapshot(ctx context.Context, agentID string) (*MemorySnapshot, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	snapshot := &MemorySnapshot{
		Documents: []MemoryDocument{},
		Layout:    "empty",
	}
	indexContent := ""
	confinedRoot, err := s.openAgentWorkspace(agentValue, false)
	if err != nil {
		return nil, err
	}
	defer confinedRoot.Close()
	if document, content, ok := readMemoryIndex(confinedRoot); ok {
		snapshot.Index = &document
		indexContent = content
	}

	indexedPaths := memoryIndexedPaths(indexContent)
	documents, total := scanMemoryDocuments(ctx, confinedRoot, indexedPaths)
	if len(documents) > memoryDocumentLimit {
		documents = documents[:memoryDocumentLimit]
	}
	snapshot.Documents = documents
	snapshot.Truncated = total > len(documents)
	snapshot.Layout = memoryLayout(snapshot.Index != nil, documents)
	return snapshot, nil
}

// DeleteMemoryDocument 删除一份正文记忆，并让短索引与文件系统保持一致。
func (s *Service) DeleteMemoryDocument(
	ctx context.Context,
	agentID string,
	relativePath string,
) (*EntryMutationResponse, error) {
	agentValue, err := s.ensureAgentWorkspace(ctx, agentID)
	if err != nil {
		return nil, err
	}
	_, normalizedPath, err := resolveWorkspacePath(agentValue.WorkspacePath, relativePath)
	if err != nil {
		return nil, invalidWorkspaceMutation(err)
	}
	if !isDeletableMemoryDocumentPath(normalizedPath) {
		return nil, invalidWorkspaceMutation(errors.New("不支持删除 memory/ 之外的记忆文件"))
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
	if !info.Mode().IsRegular() {
		return nil, invalidWorkspaceMutation(errors.New("不支持删除非普通记忆文件"))
	}

	indexContent, indexChanged, err := memoryIndexWithoutDocument(confinedRoot, normalizedPath)
	if err != nil {
		return nil, err
	}
	if indexChanged {
		if s.live != nil {
			s.live.SuppressWatcher(agentValue.AgentID, memoryEntrypointName)
		}
		if err = confinedRoot.WriteFileAtomic(
			memoryEntrypointName,
			indexContent.updated,
			workspaceFileMode(),
		); err != nil {
			return nil, err
		}
		s.emitMemoryIndexWrite(agentValue.AgentID, indexContent.updated)
	}
	if s.live != nil {
		s.live.SuppressWatcher(agentValue.AgentID, normalizedPath)
	}
	if err = confinedRoot.Remove(normalizedPath); err != nil {
		if indexChanged {
			if s.live != nil {
				s.live.SuppressWatcher(agentValue.AgentID, memoryEntrypointName)
			}
			rollbackErr := confinedRoot.WriteFileAtomic(
				memoryEntrypointName,
				indexContent.original,
				workspaceFileMode(),
			)
			if rollbackErr == nil {
				s.emitMemoryIndexWrite(agentValue.AgentID, indexContent.original)
			}
			return nil, errors.Join(err, rollbackErr)
		}
		return nil, err
	}
	if s.live != nil {
		s.live.EmitAPIDelete(agentValue.AgentID, normalizedPath)
	}
	return &EntryMutationResponse{Path: normalizedPath}, nil
}

type memoryIndexContent struct {
	original []byte
	updated  []byte
}

func memoryIndexWithoutDocument(
	root *confinedfs.Root,
	targetPath string,
) (memoryIndexContent, bool, error) {
	original, err := root.ReadFile(memoryEntrypointName)
	if os.IsNotExist(err) {
		return memoryIndexContent{}, false, nil
	}
	if err != nil {
		return memoryIndexContent{}, false, err
	}
	updated, changed := removeMemoryIndexLines(string(original), targetPath)
	content := memoryIndexContent{original: original, updated: []byte(updated)}
	if !changed {
		return content, false, nil
	}
	return content, true, nil
}

func (s *Service) emitMemoryIndexWrite(agentID string, content []byte) {
	if s.live == nil {
		return
	}
	s.live.EmitAPIWrite(agentID, memoryEntrypointName, string(content))
}

func isDeletableMemoryDocumentPath(path string) bool {
	normalizedPath := filepath.ToSlash(filepath.Clean(path))
	return strings.HasPrefix(normalizedPath, memoryDirectoryName+"/") &&
		strings.EqualFold(filepath.Ext(normalizedPath), ".md")
}

func removeMemoryIndexLines(content string, targetPath string) (string, bool) {
	lines := strings.SplitAfter(content, "\n")
	kept := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if memoryIndexLineTargetsPath(line, targetPath) {
			removed = true
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, ""), removed
}

func memoryIndexLineTargetsPath(line string, targetPath string) bool {
	for _, match := range memoryIndexLinkPattern.FindAllStringSubmatch(line, -1) {
		if len(match) >= 2 && normalizeMemoryIndexPath(match[1]) == targetPath {
			return true
		}
	}
	return false
}

func readMemoryIndex(root *confinedfs.Root) (MemoryDocument, string, bool) {
	info, err := root.Stat(memoryEntrypointName)
	if err != nil || !info.Mode().IsRegular() {
		return MemoryDocument{}, "", false
	}
	content, err := root.ReadFile(memoryEntrypointName)
	if err != nil {
		return MemoryDocument{}, "", false
	}
	return MemoryDocument{
		Indexed:    true,
		Kind:       "index",
		ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
		Path:       memoryEntrypointName,
		Size:       info.Size(),
		Title:      memoryEntrypointName,
	}, string(content), true
}

func scanMemoryDocuments(ctx context.Context, root *confinedfs.Root, indexedPaths map[string]struct{}) ([]MemoryDocument, int) {
	documents := make([]MemoryDocument, 0, 32)
	_ = root.Walk(memoryDirectoryName, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry == nil || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if info.IsDir() || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(info.Name()), ".md") {
			return nil
		}
		normalizedPath := filepath.ToSlash(path)
		frontmatter := readMemoryFrontmatter(root, normalizedPath)
		_, indexed := indexedPaths[normalizedPath]
		documents = append(documents, MemoryDocument{
			Description: frontmatter["description"],
			Indexed:     indexed,
			Kind:        memoryDocumentKind(normalizedPath),
			ModifiedAt:  info.ModTime().UTC().Format(time.RFC3339),
			Name:        frontmatter["name"],
			Path:        normalizedPath,
			Size:        info.Size(),
			Title:       memoryDocumentTitle(normalizedPath, frontmatter["name"]),
			Type:        normalizeMemoryType(frontmatter["type"]),
		})
		return nil
	})
	slices.SortFunc(documents, func(left MemoryDocument, right MemoryDocument) int {
		if left.ModifiedAt != right.ModifiedAt {
			return strings.Compare(right.ModifiedAt, left.ModifiedAt)
		}
		return strings.Compare(left.Path, right.Path)
	})
	return documents, len(documents)
}

func readMemoryFrontmatter(root *confinedfs.Root, path string) map[string]string {
	file, err := root.OpenFileNoSymlink(path, os.O_RDONLY, 0)
	if err != nil {
		return map[string]string{}
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)
	lines := make([]string, 0, memoryFrontmatterMaxLines)
	for scanner.Scan() && len(lines) < memoryFrontmatterMaxLines {
		lines = append(lines, scanner.Text())
	}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]string{}
	}
	result := map[string]string{}
	closed := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			closed = true
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			result[key] = value
		}
	}
	if !closed {
		return map[string]string{}
	}
	return result
}

func memoryIndexedPaths(content string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, match := range memoryIndexLinkPattern.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		if path := normalizeMemoryIndexPath(match[1]); path != "" {
			result[path] = struct{}{}
		}
	}
	return result
}

func normalizeMemoryIndexPath(value string) string {
	path := strings.Trim(strings.TrimSpace(value), "<>")
	path = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
	if path == memoryDirectoryName || !strings.HasPrefix(path, memoryDirectoryName+"/") {
		return ""
	}
	return path
}

func memoryDocumentKind(path string) string {
	if strings.HasPrefix(filepath.ToSlash(path), memoryDirectoryName+"/logs/") {
		return "daily_log"
	}
	return "topic"
}

func memoryDocumentTitle(path string, name string) string {
	if title := strings.TrimSpace(name); title != "" {
		return title
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if memoryDocumentKind(path) == "daily_log" {
		return base
	}
	return strings.ReplaceAll(strings.ReplaceAll(base, "_", " "), "-", " ")
}

func normalizeMemoryType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user", "feedback", "project", "reference":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func memoryLayout(hasIndex bool, documents []MemoryDocument) string {
	topicCount := 0
	logCount := 0
	for _, document := range documents {
		if document.Kind == "daily_log" {
			logCount++
		} else {
			topicCount++
		}
	}
	switch {
	case topicCount > 0 && logCount > 0:
		return "mixed"
	case logCount > 0:
		return "daily_log"
	case topicCount > 0 || hasIndex:
		return "topic"
	default:
		return "empty"
	}
}
