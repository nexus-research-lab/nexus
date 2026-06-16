package transport

import "strings"

func SplitText(text string, limit int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}

	result := make([]string, 0, len(runes)/limit+1)
	for start := 0; start < len(runes); start += limit {
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		result = append(result, string(runes[start:end]))
	}
	return result
}
