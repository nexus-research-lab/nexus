package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
		prefix, rest, _ := strings.Cut(normalized, marker)
		prefix += marker
		sectionBody, suffix, hasNextSection := strings.Cut(rest, "\n## ")
		if hasNextSection {
			suffix = "\n## " + suffix
		}
		sectionBody = strings.TrimRight(sectionBody, "\n")
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

// parseDateFromEntryID 从 entry ID 中解出日期，用于快速定位日记文件。
// 支持 KIND-YYYYMMDD-... 格式（新）和 KIND-YYYYMMDD-HHMM-... 格式（旧）。
func parseDateFromEntryID(entryID string) (time.Time, bool) {
	_, rest, ok := strings.Cut(entryID, "-")
	if !ok {
		return time.Time{}, false
	}
	date, _, _ := strings.Cut(rest, "-")
	if len(date) != 8 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("20060102", date, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
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
