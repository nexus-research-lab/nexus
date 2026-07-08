package runtime

import "strings"

// firstNonEmpty 返回首个去空白后非空的字符串；trace 子包有各自的私有副本，
// 这里是 runtime 核心（client/summary）仍需的最小工具，故重复保留。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
