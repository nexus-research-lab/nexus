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

func intPointerArg(args map[string]any, key string) *int {
	if args == nil {
		return nil
	}
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	var value int
	switch v := raw.(type) {
	case int:
		value = v
	case int64:
		value = int(v)
	case float64:
		if math.Trunc(v) != v {
			return nil
		}
		value = int(v)
	case string:
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &value); err != nil {
			return nil
		}
	default:
		return nil
	}
	return &value
}
