// Package argx 提供 MCP 工具入参的基础类型转换与访问器。
package argx

import (
	"fmt"
	"strings"
)

// String 取出指定 key 的字符串值并去除首尾空白。
func String(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	return strings.TrimSpace(StringOf(args[key]))
}

// StringOf 将任意 JSON 解码值转为字符串。
func StringOf(value any) string {
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

// Bool 取布尔值，缺省时返回 fallback。
func Bool(args map[string]any, key string, fallback bool) bool {
	if args == nil {
		return fallback
	}
	if raw, ok := args[key]; ok {
		return ParseBool(raw)
	}
	return fallback
}

// ParseBool 兼容 bool / 数字 / 字符串形式的布尔值。
func ParseBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return false
}

// Int 兼容 int / float / 字符串形式的整数值。
func Int(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var n int
		fmt.Sscanf(strings.TrimSpace(v), "%d", &n)
		return n
	}
	return 0
}

// FirstNonEmpty 返回第一个非空字符串。
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// HasObject 判断 args[key] 是否是 JSON object。
func HasObject(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	v, ok := args[key]
	if !ok {
		return false
	}
	_, isMap := v.(map[string]any)
	return isMap
}
