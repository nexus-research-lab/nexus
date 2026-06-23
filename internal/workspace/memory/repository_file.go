package memory

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// ReadSlice 读取文件片段。
func (r *Repository) ReadSlice(relativePath string, fromLine int, lines int) (*Slice, error) {
	targetPath, normalizedPath, err := r.resolveWorkspaceFile(relativePath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}
	contentLines := strings.Split(string(content), "\n")
	if fromLine <= 0 {
		fromLine = 1
	}
	if lines <= 0 {
		lines = 50
	}
	startIndex := fromLine - 1
	if startIndex >= len(contentLines) {
		startIndex = len(contentLines)
	}
	endIndex := startIndex + lines
	if endIndex > len(contentLines) {
		endIndex = len(contentLines)
	}
	return &Slice{
		Path:     normalizedPath,
		FromLine: startIndex + 1,
		ToLine:   endIndex,
		Content:  strings.Join(contentLines[startIndex:endIndex], "\n"),
	}, nil
}

func (r *Repository) iterSearchFiles() []string {
	items := make([]string, 0, 16)
	for _, name := range rootMemoryFiles {
		path := filepath.Join(r.workspacePath, name)
		if _, err := os.Stat(path); err == nil {
			items = append(items, path)
		}
	}
	memoryFiles := r.iterMemoryMarkdownFiles()
	items = append(items, memoryFiles.diaries...)
	items = append(items, memoryFiles.extra...)
	sort.Sort(sort.Reverse(sort.StringSlice(items)))
	return slices.Compact(items)
}

func (r *Repository) iterDiaryFiles() []string {
	return r.iterMemoryMarkdownFiles().diaries
}

type memoryMarkdownFiles struct {
	diaries []string
	extra   []string
}

func (r *Repository) iterMemoryMarkdownFiles() memoryMarkdownFiles {
	memoryDir := filepath.Join(r.workspacePath, "memory")
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return memoryMarkdownFiles{}
	}
	result := memoryMarkdownFiles{
		diaries: make([]string, 0, len(entries)),
		extra:   make([]string, 0, len(entries)),
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(memoryDir, entry.Name())
		if _, ok := parseDiaryDate(path); ok {
			result.diaries = append(result.diaries, path)
			continue
		}
		result.extra = append(result.extra, path)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(result.diaries)))
	sort.Sort(sort.Reverse(sort.StringSlice(result.extra)))
	return result
}

func (r *Repository) resolveWorkspaceFile(relativePath string) (string, string, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/"))
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return "", "", newClientError("path 不能为空")
	}
	targetPath := filepath.Clean(filepath.Join(r.workspacePath, normalized))
	workspaceRoot := filepath.Clean(r.workspacePath)
	if targetPath != workspaceRoot && !strings.HasPrefix(targetPath, workspaceRoot+string(os.PathSeparator)) {
		return "", "", newClientError("path 超出 workspace 范围")
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", newClientError("不能直接读取目录")
	}
	return targetPath, filepath.ToSlash(normalized), nil
}

func toRelative(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
