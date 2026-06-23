package memory

import (
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

func normalizeStatusSet(statuses []string) map[string]struct{} {
	result := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		value := strings.TrimSpace(status)
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func summarizeTitle(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if text == "" {
		return "自动记忆"
	}
	return truncateRunes(text, autoMemoryTitleMaxRunes)
}

func truncateRunes(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:maxRunes])) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if clean := strings.TrimSpace(value); clean != "" {
			return clean
		}
	}
	return ""
}

func sortedMapKeys(values map[string]int) []string {
	return slices.Sorted(maps.Keys(values))
}
