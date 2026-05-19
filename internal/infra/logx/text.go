package logx

import "strings"

// PreviewText 将可能很长的用户文本压缩成适合单行日志的预览。
func PreviewText(value string, maxRunes int) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	normalized = strings.Join(strings.Fields(normalized), " ")
	if maxRunes <= 0 {
		return normalized
	}
	runes := []rune(normalized)
	if len(runes) <= maxRunes {
		return normalized
	}
	return string(runes[:maxRunes]) + "..."
}
