// INPUT: MCP 工具调用的原始 args。
// OUTPUT: 入口裁剪后的字符串、列表、对象与整数参数。
// POS: 各 MCP 工具共用的参数解析原语；类型不符一律视为缺省值，不做跨类型转换。
package sdktool

import (
	"math"
	"slices"
	"strings"
)

// StringArg 读取字符串参数并裁剪首尾空白；缺失或非字符串返回空串。
func StringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

// StringListArg 读取字符串数组，逐项裁剪并去掉空值与重复值，保持首次出现顺序；
// 参数缺失返回 nil，参数存在但无有效项返回空切片。
func StringListArg(args map[string]any, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	var values []string
	switch typed := raw.(type) {
	case []string:
		values = typed
	case []any:
		for _, item := range typed {
			if value, ok := item.(string); ok {
				values = append(values, value)
			}
		}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

// StringMapArg 读取字符串到字符串的对象参数；非字符串值被丢弃，返回副本。
func StringMapArg(args map[string]any, key string) map[string]string {
	switch typed := args[key].(type) {
	case map[string]string:
		result := make(map[string]string, len(typed))
		for itemKey, value := range typed {
			result[itemKey] = value
		}
		return result
	case map[string]any:
		result := make(map[string]string, len(typed))
		for itemKey, value := range typed {
			if text, ok := value.(string); ok {
				result[itemKey] = text
			}
		}
		return result
	default:
		return nil
	}
}

// ObjectArg 读取 JSON 对象参数；缺失或类型不符返回 nil。
func ObjectArg(args map[string]any, key string) map[string]any {
	value, _ := args[key].(map[string]any)
	return value
}

// IntArg 读取整数参数；JSON 数字只接受无小数部分的值，其余返回 0。
func IntArg(args map[string]any, key string) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		if math.Trunc(value) == value {
			return int(value)
		}
	}
	return 0
}
