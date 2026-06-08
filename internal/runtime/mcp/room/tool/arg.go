package tool

import (
	"fmt"
	"math"
	"strings"
)

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	return strings.TrimSpace(stringValue(args[key]))
}

func stringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func stringListArg(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	raw, ok := args[key]
	if !ok {
		return nil
	}
	values := []string{}
	switch typed := raw.(type) {
	case []string:
		values = typed
	case []any:
		for _, item := range typed {
			values = append(values, stringValue(item))
		}
	default:
		if value := stringValue(typed); value != "" {
			values = append(values, value)
		}
	}
	return normalizeStrings(values)
}

func objectArg(args map[string]any, key string) map[string]any {
	if args == nil {
		return nil
	}
	if value, ok := args[key].(map[string]any); ok {
		return value
	}
	return nil
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		if math.Trunc(v) != v {
			return 0
		}
		return int(v)
	case string:
		var result int
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &result); err == nil {
			return result
		}
	}
	return 0
}

func normalizeStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" || containsString(result, normalized) {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
