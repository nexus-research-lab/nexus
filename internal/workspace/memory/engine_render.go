package memory

import (
	"fmt"
	"strings"
)

func renderRelevantMemories(items []MemoryItem, maxChars int) string {
	if len(items) == 0 {
		return ""
	}
	lines := []string{"<relevant-memories>"}
	total := len(lines[0])
	for _, item := range items {
		line := fmt.Sprintf(
			"- [%s] %s：%s (status=%s, scope=%s, access_count=%d)",
			item.EntryID,
			item.Title,
			truncateRunes(item.Content, 220),
			item.Status,
			item.Scope,
			item.AccessCount,
		)
		if total+len(line) > maxChars {
			break
		}
		lines = append(lines, line)
		total += len(line)
	}
	lines = append(lines, "</relevant-memories>")
	return strings.Join(lines, "\n")
}
