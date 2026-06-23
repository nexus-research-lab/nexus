package memory

import (
	"os"
	"strings"
)

func (r *Repository) ListEntries(limit int) ([]*Entry, error) {
	capacityHint := limit
	if capacityHint <= 0 {
		capacityHint = 128
	}
	items := make([]*Entry, 0, capacityHint)
	for _, path := range r.iterDiaryFiles() {
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
			if limit > 0 && len(items) >= limit {
				return items, nil
			}
		}
	}
	return items, nil
}

func (r *Repository) FindEntry(entryID string) (*Entry, error) {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return nil, newClientError("entry_id 不能为空")
	}
	for _, path := range r.iterDiaryFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		entries, err := r.parser.Parse(string(content), toRelative(r.workspacePath, path))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.ID == entryID {
				return entry, nil
			}
		}
	}
	return nil, newClientError("未找到条目: %s", entryID)
}

func (r *Repository) DeleteEntry(entryID string) error {
	entryID = strings.TrimSpace(entryID)
	if entryID == "" {
		return newClientError("entry_id 不能为空")
	}
	for _, path := range r.iterDiaryFiles() {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relativePath := toRelative(r.workspacePath, path)
		entries, err := r.parser.Parse(string(content), relativePath)
		if err != nil {
			return err
		}
		next := make([]*Entry, 0, len(entries))
		found := false
		for _, entry := range entries {
			if entry.ID == entryID {
				found = true
				continue
			}
			next = append(next, entry)
		}
		if !found {
			continue
		}
		if len(next) == 0 {
			if err := os.Remove(path); err != nil {
				return err
			}
			return nil
		}
		return os.WriteFile(path, []byte(renderEntries(next)), 0o644)
	}
	return newClientError("未找到条目: %s", entryID)
}
