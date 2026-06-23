package feishudocx

import "strings"

func quoteMarkdown(value string) string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		lines = append(lines, "> "+line)
	}
	return strings.Join(lines, "\n")
}

func splitPath(path string) []string {
	raw := strings.Split(path, "/")
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func indent(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat("  ", depth)
}

func repeatString(value string, count int) []string {
	if count <= 0 {
		return nil
	}
	result := make([]string, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if value, ok := item.(string); ok && value != "" {
				result = append(result, value)
			}
		}
		return result
	default:
		return nil
	}
}

func stringField(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return value
}

func boolField(raw map[string]any, key string) bool {
	value, _ := raw[key].(bool)
	return value
}

func intField(raw map[string]any, key string) int {
	if raw == nil {
		return 0
	}
	switch value := raw[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}
