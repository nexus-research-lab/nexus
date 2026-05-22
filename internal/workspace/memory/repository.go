package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SearchMatch 表示检索结果。
type SearchMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// Slice 表示文件片段。
type Slice struct {
	Path     string `json:"path"`
	FromLine int    `json:"from_line"`
	ToLine   int    `json:"to_line"`
	Content  string `json:"content"`
}

// Repository 负责管理 workspace 记忆文件。
type Repository struct {
	workspacePath string
	parser        Parser
}

var (
	rootMemoryFiles  = []string{"MEMORY.md", "SOUL.md", "TOOLS.md", "AGENTS.md", "RUNBOOK.md"}
	errEntryNotFound = errors.New("条目未找到")
)

// NewRepository 创建记忆仓储。
func NewRepository(workspacePath string) *Repository {
	return &Repository{
		workspacePath: filepath.Clean(strings.TrimSpace(workspacePath)),
		parser:        Parser{},
	}
}

// Search 在记忆文件中做关键词检索，以条目块为单位匹配，支持跨字段搜索。
func (r *Repository) Search(query string, limit int) ([]SearchMatch, error) {
	terms := tokenizeQuery(query)
	if len(terms) == 0 {
		return nil, newClientError("query 不能为空")
	}
	if limit <= 0 {
		limit = 20
	}
	items := make([]SearchMatch, 0, limit)
	for _, path := range r.iterSearchFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		relPath := toRelative(r.workspacePath, path)
		for _, block := range splitIntoBlocks(string(content)) {
			if !containsAllTerms(strings.ToLower(block.content), terms) {
				continue
			}
			items = append(items, SearchMatch{
				Path:    relPath,
				Line:    block.startLine,
				Content: block.headline,
			})
			if len(items) >= limit {
				return items, nil
			}
		}
	}
	return items, nil
}

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

// ListRecentEntries 返回近期条目。
func (r *Repository) ListRecentEntries(days int, limit int) ([]*Entry, error) {
	if days <= 0 {
		days = 7
	}
	if limit <= 0 {
		limit = 50
	}
	cutoff := time.Now().AddDate(0, 0, -(days - 1))
	items := make([]*Entry, 0, limit)
	for _, path := range r.iterDiaryFiles() {
		diaryDate, ok := parseDiaryDate(path)
		if !ok || diaryDate.Before(beginOfDay(cutoff)) {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		parsed, err := r.parser.Parse(string(content), toRelative(r.workspacePath, path))
		if err != nil {
			return nil, err
		}
		for index := len(parsed) - 1; index >= 0; index-- {
			items = append(items, parsed[index])
			if len(items) >= limit {
				return items, nil
			}
		}
	}
	return items, nil
}

// AppendEntry 把条目追加到今日日志。
func (r *Repository) AppendEntry(entry *Entry) (string, error) {
	diaryPath := filepath.Join(r.workspacePath, "memory", entry.CreatedAt.Format("2006-01-02")+".md")
	if err := os.MkdirAll(filepath.Dir(diaryPath), 0o755); err != nil {
		return "", err
	}
	existing := ""
	if content, err := os.ReadFile(diaryPath); err == nil {
		existing = strings.TrimRight(string(content), "\n")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	nextContent := entry.Markdown() + "\n"
	if strings.TrimSpace(existing) != "" {
		nextContent = existing + "\n\n" + nextContent
	}
	if err := os.WriteFile(diaryPath, []byte(nextContent), 0o644); err != nil {
		return "", err
	}
	entry.Path = toRelative(r.workspacePath, diaryPath)
	return entry.Path, nil
}

// UpdateEntry 更新指定条目，优先从 ID 中解出日期直接定位文件，兜底全量扫描兼容旧格式。
func (r *Repository) UpdateEntry(entryID string, updater func(*Entry)) (*Entry, error) {
	if t, ok := parseDateFromEntryID(entryID); ok {
		path := filepath.Join(r.workspacePath, "memory", t.Format("2006-01-02")+".md")
		entry, err := r.updateEntryInFile(path, entryID, updater)
		if err == nil {
			return entry, nil
		}
		if !errors.Is(err, errEntryNotFound) && !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, path := range r.iterDiaryFiles() {
		entry, err := r.updateEntryInFile(path, entryID, updater)
		if err == nil {
			return entry, nil
		}
		if !errors.Is(err, errEntryNotFound) {
			return nil, err
		}
	}
	return nil, newClientError("未找到条目: %s", entryID)
}

func (r *Repository) updateEntryInFile(path string, entryID string, updater func(*Entry)) (*Entry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	relativePath := toRelative(r.workspacePath, path)
	entries, err := r.parser.Parse(string(content), relativePath)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.ID != entryID {
			continue
		}
		updater(entry)
		if err = os.WriteFile(path, []byte(renderEntries(entries)), 0o644); err != nil {
			return nil, err
		}
		return entry, nil
	}
	return nil, errEntryNotFound
}

// AppendToMemorySection 向长期文件追加规则。
func (r *Repository) AppendToMemorySection(filename string, sectionTitle string, bullet string) (string, error) {
	targetPath := filepath.Join(r.workspacePath, filename)
	existing := fmt.Sprintf("# %s\n\n", filename)
	if content, err := os.ReadFile(targetPath); err == nil {
		existing = string(content)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	marker := fmt.Sprintf("## %s\n", sectionTitle)
	normalized := existing
	if !strings.HasSuffix(normalized, "\n") {
		normalized += "\n"
	}
	var updated string
	if !strings.Contains(normalized, marker) {
		updated = normalized + "\n" + marker + bullet + "\n"
	} else {
		start := strings.Index(normalized, marker) + len(marker)
		nextSection := strings.Index(normalized[start:], "\n## ")
		if nextSection >= 0 {
			nextSection += start
		}
		prefix := normalized[:start]
		sectionBody := strings.TrimRight(normalized[start:maxIndex(nextSection, len(normalized))], "\n")
		suffix := ""
		if nextSection >= 0 {
			suffix = normalized[nextSection:]
		}
		if sectionBody != "" {
			sectionBody += "\n"
		}
		updated = prefix + sectionBody + bullet + "\n" + suffix
	}
	if err := os.WriteFile(targetPath, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return filename, nil
}

func (r *Repository) iterSearchFiles() []string {
	items := make([]string, 0, 16)
	for _, name := range rootMemoryFiles {
		path := filepath.Join(r.workspacePath, name)
		if fileExists(path) {
			items = append(items, path)
		}
	}
	memoryFiles := r.iterMemoryMarkdownFiles()
	items = append(items, memoryFiles.diaries...)
	items = append(items, memoryFiles.extra...)
	sort.Sort(sort.Reverse(sort.StringSlice(items)))
	return dedupe(items)
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

// searchBlock 表示一个可检索的文件块。
type searchBlock struct {
	startLine int
	headline  string
	content   string
}

// splitIntoBlocks 按 markdown 标题把文件内容分割成可检索的块。
// 标题前的非空行以单行为单位加入结果，供检索 MEMORY.md 等无条目结构的文件。
func splitIntoBlocks(content string) []searchBlock {
	lines := strings.Split(content, "\n")
	var blocks []searchBlock
	blockStart := -1
	var blockLines []string

	flush := func() {
		if blockStart < 0 || len(blockLines) == 0 {
			return
		}
		blocks = append(blocks, searchBlock{
			startLine: blockStart + 1,
			headline:  strings.TrimSpace(blockLines[0]),
			content:   strings.Join(blockLines, "\n"),
		})
	}

	for i, line := range lines {
		if isMarkdownHeading(line) {
			flush()
			blockStart = i
			blockLines = []string{line}
		} else if blockStart >= 0 {
			blockLines = append(blockLines, line)
		} else {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				blocks = append(blocks, searchBlock{startLine: i + 1, headline: trimmed, content: line})
			}
		}
	}
	flush()
	return blocks
}

func isMarkdownHeading(line string) bool {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	return i > 0 && i <= 6 && i < len(line) && line[i] == ' '
}

// parseDateFromEntryID 从 entry ID 中解出日期，用于快速定位日记文件。
// 支持 KIND-YYYYMMDD-... 格式（新）和 KIND-YYYYMMDD-HHMM-... 格式（旧）。
func parseDateFromEntryID(entryID string) (time.Time, bool) {
	parts := strings.SplitN(entryID, "-", 3)
	if len(parts) < 2 || len(parts[1]) != 8 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("20060102", parts[1], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func tokenizeQuery(query string) []string {
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func containsAllTerms(value string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(value, term) {
			return false
		}
	}
	return true
}

func parseDiaryDate(path string) (time.Time, bool) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	value, err := time.ParseInLocation("2006-01-02", name, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return value, true
}

func beginOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func renderEntries(entries []*Entry) string {
	items := make([]string, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.Markdown())
	}
	return strings.TrimSpace(strings.Join(items, "\n\n")) + "\n"
}

func toRelative(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func maxIndex(index int, fallback int) int {
	if index >= 0 {
		return index
	}
	return fallback
}
