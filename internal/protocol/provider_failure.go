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

// IsProviderTokenLimitError 判断 Provider 是否因为上下文、输出或账户 Token
// 额度达到上限而拒绝请求。不同 Provider 会把这类错误标成
// invalid_request（并在正文带上下文/额度信号）、context_length 或 usage_limit，
// 不能只依赖一个字段名。
func IsProviderTokenLimitError(signals ...string) bool {
	for _, signal := range signals {
		normalized := strings.ToLower(strings.TrimSpace(signal))
		if normalized == "" {
			continue
		}
		compact := strings.NewReplacer("_", "", "-", "", " ", "", ".", "").Replace(normalized)
		switch compact {
		case "contextlength", "contextlengthexceeded", "contextlimit",
			"maxtokens", "maxoutputtokens", "outputtokenlimit", "tokenlimit",
			"tokenlimitexceeded", "toomanytokens", "promptistoolong",
			"requesttoolarge", "outoftokens", "notokensleft",
			"usagelimit", "usagelimitreached", "usagelimitexceeded",
			"quotaexceeded", "quotaexhausted", "insufficientquota",
			"tokenbudgetexceeded":
			return true
		}
		for _, marker := range []string{
			"contextlength", "contextlimit", "maxtokens", "maxoutputtokens",
			"outputtokenlimit", "tokenlimit", "toomanytokens",
			"promptistoolong", "requesttoolarge", "outoftokens",
			"notokensleft", "usagelimit", "quotaexceeded", "quotaexhausted",
			"insufficientquota", "tokenbudgetexceeded",
		} {
			if strings.Contains(compact, marker) {
				return true
			}
		}
		if strings.Contains(normalized, "maximum context length") ||
			strings.Contains(normalized, "context length exceeded") ||
			strings.Contains(normalized, "too many tokens") ||
			strings.Contains(normalized, "prompt is too long") ||
			strings.Contains(normalized, "request is too large") ||
			strings.Contains(normalized, "out of tokens") ||
			strings.Contains(normalized, "no tokens left") ||
			strings.Contains(normalized, "insufficient quota") ||
			strings.Contains(normalized, "token limit") ||
			strings.Contains(normalized, "usage limit") ||
			strings.Contains(normalized, "quota exceeded") ||
			strings.Contains(normalized, "quota exhausted") ||
			strings.Contains(normalized, "token budget exceeded") ||
			(strings.Contains(normalized, "上下文长度") &&
				(strings.Contains(normalized, "超过") || strings.Contains(normalized, "上限"))) ||
			(strings.Contains(normalized, "token") &&
				(strings.Contains(normalized, "耗尽") || strings.Contains(normalized, "上限"))) ||
			(strings.Contains(normalized, "额度") &&
				(strings.Contains(normalized, "用尽") || strings.Contains(normalized, "不足"))) {
			return true
		}
	}
	return false
}
