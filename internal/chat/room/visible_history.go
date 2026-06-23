package room

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const roomHistoryTruncatedSuffix = "\n...(truncated)"

func contextPublicMessages(messages []protocol.Message, trigger Trigger) []protocol.Message {
	triggerMessageID := strings.TrimSpace(trigger.MessageID)
	if triggerMessageID == "" || len(messages) == 0 {
		return messages
	}
	filtered := make([]protocol.Message, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(normalizeAnyString(message["message_id"])) == triggerMessageID {
			continue
		}
		filtered = append(filtered, message)
	}
	return filtered
}

func buildHistoryLines(history []protocol.Message, agentNameByID map[string]string) []string {
	if len(history) == 0 {
		return nil
	}

	start := 0
	if len(history) > roomMaxHistoryMessages {
		start = len(history) - roomMaxHistoryMessages
	}

	formatted := make([]string, 0, len(history)-start)
	for _, message := range history[start:] {
		line := formatHistoryLine(message, agentNameByID)
		if line != "" {
			formatted = append(formatted, line)
		}
	}

	lines := make([]string, 0, len(formatted))
	totalChars := 0
	for index := len(formatted) - 1; index >= 0; index-- {
		line := formatted[index]
		nextChars := totalChars + len(line)
		if totalChars > 0 {
			nextChars++
		}
		if nextChars > roomMaxHistoryChars {
			if len(lines) == 0 {
				truncated := truncateHistoryText(line, roomMaxHistoryChars)
				if truncated != "" {
					lines = append(lines, truncated)
				}
			}
			break
		}
		lines = append(lines, line)
		totalChars = nextChars
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	return lines
}

func formatHistoryLine(message protocol.Message, agentNameByID map[string]string) string {
	role := strings.TrimSpace(normalizeAnyString(message["role"]))
	var content string
	switch role {
	case "user":
		content = extractHistoryText(message)
	case "assistant":
		if isComplete, ok := message["is_complete"].(bool); ok && !isComplete {
			return ""
		}
		content = extractAssistantResultText(message)
	default:
		return ""
	}
	if content == "" {
		return ""
	}

	switch role {
	case "user":
		return "User: " + content
	case "assistant":
		agentID := normalizeAnyString(message["agent_id"])
		return fmt.Sprintf("Assistant(%s): %s", firstNonEmpty(agentNameByID[agentID], agentID, "Assistant"), content)
	default:
		return ""
	}
}

func extractAssistantResultText(message protocol.Message) string {
	if summary, ok := message["result_summary"].(map[string]any); ok {
		if text := extractHistoryText(message); text != "" {
			return text
		}
		return strings.TrimSpace(normalizeAnyString(summary["result"]))
	}
	if message["is_complete"] == true {
		return extractHistoryText(message)
	}
	return ""
}

// ExtractAssistantResultText 返回 assistant 终态摘要中的公开文本。
func ExtractAssistantResultText(message protocol.Message) string {
	return extractAssistantResultText(message)
}

func extractHistoryText(message protocol.Message) string {
	if raw, ok := message["content"].(string); ok {
		return strings.TrimSpace(raw)
	}

	items := normalizeHistoryContentBlocks(message["content"])
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, 0, len(items))
	for _, payload := range items {
		if text := strings.TrimSpace(normalizeAnyString(payload["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// ExtractHistoryText 返回消息 content 中可进入 Room 公区上下文的文本。
func ExtractHistoryText(message protocol.Message) string {
	return extractHistoryText(message)
}

func normalizeHistoryContentBlocks(content any) []map[string]any {
	switch typed := content.(type) {
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if payload, ok := item.(map[string]any); ok {
				items = append(items, payload)
			}
		}
		return items
	case []map[string]any:
		return slices.Clone(typed)
	default:
		return nil
	}
}

func truncateHistoryText(value string, maxBytes int) string {
	trimmed := strings.TrimSpace(value)
	if maxBytes <= 0 || len(trimmed) <= maxBytes {
		return trimmed
	}
	if maxBytes <= len(roomHistoryTruncatedSuffix) {
		return trimStringByBytes(trimmed, maxBytes)
	}
	body := trimStringByBytes(trimmed, maxBytes-len(roomHistoryTruncatedSuffix))
	if body == "" {
		return trimStringByBytes(trimmed, maxBytes)
	}
	return strings.TrimSpace(body) + roomHistoryTruncatedSuffix
}

func trimStringByBytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return strings.TrimSpace(value)
	}
	end := 0
	for index, currentRune := range value {
		width := utf8.RuneLen(currentRune)
		if width <= 0 {
			width = 1
		}
		if index+width > maxBytes {
			break
		}
		end = index + width
	}
	return strings.TrimSpace(value[:end])
}
