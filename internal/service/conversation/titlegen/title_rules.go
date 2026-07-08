package titlegen

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	defaultConversationPattern = regexp.MustCompile(`^.+\s·\s对话\s+\d+$`)
	whitespacePattern          = regexp.MustCompile(`\s+`)
	defaultSessionTitles       = map[string]struct{}{
		"":         {},
		"New Chat": {},
		"未命名会话":    {},
		"未命名话题":    {},
	}
)

func isDefaultSessionTitle(title string) bool {
	normalized := strings.TrimSpace(title)
	_, ok := defaultSessionTitles[normalized]
	return ok
}

func isDefaultConversationTitle(title string, roomName string) bool {
	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" {
		return true
	}
	normalizedRoomName := strings.TrimSpace(roomName)
	if normalizedRoomName != "" && normalizedTitle == normalizedRoomName {
		return true
	}
	return defaultConversationPattern.MatchString(normalizedTitle)
}

func canReplaceSessionTitle(title string, fallbackTitle string) bool {
	if isDefaultSessionTitle(title) {
		return true
	}
	return sameNonEmptyTitle(title, fallbackTitle)
}

func canReplaceConversationTitle(title string, roomName string, fallbackTitle string) bool {
	if isDefaultConversationTitle(title, roomName) {
		return true
	}
	return sameNonEmptyTitle(title, fallbackTitle)
}

func sameNonEmptyTitle(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && left == right
}

func truncatePromptContent(content string, maxRunes int) string {
	normalized := strings.TrimSpace(content)
	if normalized == "" || maxRunes <= 0 {
		return normalized
	}
	if utf8.RuneCountInString(normalized) <= maxRunes {
		return normalized
	}
	runes := []rune(normalized)
	return string(runes[:maxRunes])
}

func sanitizeGeneratedTitle(raw string) string {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return ""
	}
	normalized = strings.Split(normalized, "\n")[0]
	normalized = whitespacePattern.ReplaceAllString(strings.TrimSpace(normalized), " ")
	normalized = strings.Trim(normalized, "\"'“”‘’`[]()（）{}<>《》。、，！？!?:：；;")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return ""
	}
	if utf8.RuneCountInString(normalized) > 24 {
		normalized = string([]rune(normalized)[:24])
	}
	return strings.TrimSpace(normalized)
}

func shouldRetryTitleRequest(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "deadline exceeded") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "unexpected eof")
}
