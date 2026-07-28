// INPUT: Provider 返回的错误正文、终态原因与 stop reason。
// OUTPUT: 跨 runtime 共用的稳定 Provider 失败分类判断。
// POS: Provider 弱类型错误信号到 Nexus 协议语义的识别边界。
package protocol

import "strings"

const ProviderFailureContentFiltered = "content_filtered"

// IsProviderContentFilterError 判断错误信号是否表示 Provider 内容安全拦截。
func IsProviderContentFilterError(signals ...string) bool {
	for _, signal := range signals {
		normalized := strings.ToLower(strings.TrimSpace(signal))
		if normalized == "" {
			continue
		}
		switch normalized {
		case "1301", "sensitive", "content_filter", ProviderFailureContentFiltered, "content_policy_violation":
			return true
		}
		compact := strings.Join(strings.Fields(normalized), "")
		if strings.Contains(normalized, "系统检测到输入或生成内容可能包含不安全或敏感内容") ||
			strings.Contains(normalized, "[1301]") ||
			strings.Contains(compact, `"code":"1301"`) ||
			strings.Contains(normalized, "content filter") ||
			strings.Contains(normalized, "content_filter") ||
			strings.Contains(normalized, "content policy violation") ||
			strings.Contains(normalized, "sensitive content") ||
			strings.Contains(compact, `"finish_reason":"sensitive"`) {
			return true
		}
	}
	return false
}
