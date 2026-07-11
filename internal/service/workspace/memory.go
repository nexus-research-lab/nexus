package workspace

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
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
	root := filepath.Clean(agentValue.WorkspacePath)
	snapshot := &MemorySnapshot{
		Documents: []MemoryDocument{},
		Layout:    "empty",
	}
	indexContent := ""
	indexPath := filepath.Join(root, memoryEntrypointName)
	if document, content, ok := readMemoryIndex(indexPath); ok {
		snapshot.Index = &document
		indexContent = content
	}

	indexedPaths := memoryIndexedPaths(indexContent)
	documents, total := scanMemoryDocuments(ctx, root, indexedPaths)
	if len(documents) > memoryDocumentLimit {
		documents = documents[:memoryDocumentLimit]
	}
	snapshot.Documents = documents
	snapshot.Truncated = total > len(documents)
	snapshot.Layout = memoryLayout(snapshot.Index != nil, documents)
	return snapshot, nil
}

func readMemoryIndex(path string) (MemoryDocument, string, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return MemoryDocument{}, "", false
	}
	content, err := os.ReadFile(path)
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

func scanMemoryDocuments(ctx context.Context, root string, indexedPaths map[string]struct{}) ([]MemoryDocument, int) {
	memoryRoot := filepath.Join(root, memoryDirectoryName)
	documents := make([]MemoryDocument, 0, 32)
	_ = filepath.Walk(memoryRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.IsDir() || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(info.Name()), ".md") {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		normalizedPath := filepath.ToSlash(relativePath)
		frontmatter := readMemoryFrontmatter(path)
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

func readMemoryFrontmatter(path string) map[string]string {
	file, err := os.Open(path)
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
		path := strings.Trim(strings.TrimSpace(match[1]), "<>")
		path = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
		if path == memoryDirectoryName || !strings.HasPrefix(path, memoryDirectoryName+"/") {
			continue
		}
		result[path] = struct{}{}
	}
	return result
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
