package runtime

import (
	"strings"
	"unicode/utf8"
)

func normalizeSDKBlockType(blockType string) string {
	switch strings.TrimSpace(blockType) {
	case "server_tool_use":
		return "tool_use"
	case "server_tool_result":
		return "tool_result"
	default:
		return strings.TrimSpace(blockType)
	}
}

func rawMap(value any) map[string]any {
	if payload, ok := value.(map[string]any); ok {
		return payload
	}
	return map[string]any{}
}

func rawString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func appendRawLogField(fields []any, key string, value any) []any {
	if strings.TrimSpace(key) == "" {
		return fields
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fields
		}
		return append(fields, key, strings.TrimSpace(typed))
	case int:
		return append(fields, key, typed)
	case int8:
		return append(fields, key, typed)
	case int16:
		return append(fields, key, typed)
	case int32:
		return append(fields, key, typed)
	case int64:
		return append(fields, key, typed)
	case uint:
		return append(fields, key, typed)
	case uint8:
		return append(fields, key, typed)
	case uint16:
		return append(fields, key, typed)
	case uint32:
		return append(fields, key, typed)
	case uint64:
		return append(fields, key, typed)
	case float32:
		return append(fields, key, typed)
	case float64:
		return append(fields, key, typed)
	case bool:
		return append(fields, key, typed)
	default:
		return fields
	}
}

func streamDebugText(value string) string {
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if value == "" {
		return ""
	}
	const maxRunes = 240
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes]) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
