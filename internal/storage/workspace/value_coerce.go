package workspace

import (
	"encoding/json"
	"strconv"
	"strings"
)

func intFromAny(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(typed))
		return parsed
	default:
		return 0
	}
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringPointer(value string) *string {
	copyValue := value
	return &copyValue
}

func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func boolValueAny(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func stringMapFromAny(value any) map[string]string {
	typed, ok := value.(map[string]any)
	if !ok || len(typed) == 0 {
		return nil
	}
	result := make(map[string]string, len(typed))
	for key, rawValue := range typed {
		if trimmedKey := strings.TrimSpace(key); trimmedKey != "" {
			result[trimmedKey] = stringFromAny(rawValue)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
