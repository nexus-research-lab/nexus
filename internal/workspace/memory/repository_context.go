package memory

import (
	"os"
	"path/filepath"
	"strings"
)

func (r *Repository) ReadStableContext(maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = 3200
	}
	type rootFile struct {
		name  string
		title string
	}
	files := []rootFile{
		{name: "USER.md", title: "USER"},
		{name: "MEMORY.md", title: "MEMORY"},
	}
	lines := make([]string, 0, len(files)*4)
	total := 0
	for _, file := range files {
		path := filepath.Join(r.workspacePath, file.name)
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			continue
		}
		block := "# " + file.title + "\n" + trimmed
		if total+len(block) > maxChars {
			remaining := maxChars - total
			if remaining <= 0 {
				break
			}
			block = truncateRunes(block, remaining)
		}
		lines = append(lines, block)
		total += len(block)
		if total >= maxChars {
			break
		}
	}
	return strings.Join(lines, "\n\n"), nil
}
