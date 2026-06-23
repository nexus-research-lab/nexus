package workspace

import (
	"strings"
)

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
			result[trimmedKey] = strings.TrimSpace(stringFromAny(rawValue))
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
