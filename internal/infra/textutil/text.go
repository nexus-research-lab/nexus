// INPUT: 调用方已持有的候选字符串、可选字符串指针或动态 JSON 值。
// OUTPUT: 去除首尾空白后的确定性字符串。
// POS: 全仓唯一的字符串取值原语；领域语义（默认值、校验）仍由调用方表达。
package textutil

import "strings"

// FirstNonEmpty 返回第一个去除首尾空白后非空的值；全部为空时返回空串。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// PointerValue 返回去除首尾空白后的指针内容；nil 视为空串。
func PointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

// AnyString 返回去除首尾空白后的字符串值；非字符串视为空串。
func AnyString(value any) string {
	typed, _ := value.(string)
	return strings.TrimSpace(typed)
}
