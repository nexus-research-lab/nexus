package memory

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ExtractMessageText 把 runtime message 转成可用于记忆的正文。
func ExtractMessageText(message protocol.Message) string {
	if len(message) == 0 {
		return ""
	}
	if value := normalizeMessageString(message["text"]); value != "" {
		return value
	}
	if value := normalizeMessageString(message["content"]); value != "" {
		return value
	}
	if content, ok := message["content"].([]any); ok {
		parts := make([]string, 0, len(content))
		for _, block := range content {
			if text := extractContentBlockText(block); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	if content, ok := message["content"].([]map[string]any); ok {
		parts := make([]string, 0, len(content))
		for _, block := range content {
			if text := extractContentBlockText(block); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return ""
}

func extractContentBlockText(block any) string {
	value, ok := block.(map[string]any)
	if !ok {
		return normalizeMessageString(block)
	}
	for _, key := range []string{"text", "content", "result"} {
		if text := normalizeMessageString(value[key]); text != "" {
			return text
		}
	}
	return ""
}

func normalizeMessageString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}
