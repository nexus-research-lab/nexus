package trace

import (
	"reflect"
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

func RawMap(value any) map[string]any {
	if payload, ok := value.(map[string]any); ok {
		return payload
	}
	return map[string]any{}
}

func RawString(value any) string {
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
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return fields
	}
	switch reflected.Kind() {
	case reflect.String:
		text := strings.TrimSpace(reflected.String())
		if text == "" {
			return fields
		}
		return append(fields, key, text)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool:
		return append(fields, key, value)
	}
	return fields
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

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
