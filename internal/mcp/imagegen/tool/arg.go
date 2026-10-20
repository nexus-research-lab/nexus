package tool

import (
	"fmt"
	"math"
	"strings"
)

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
