// INPUT: runtime 暴露的 Provider 错误正文、终态原因与错误明细。
// OUTPUT: 稳定的内容安全拦截原因和不回显上游原文的用户可见说明。
// POS: Provider 内容安全错误到 Nexus 产品语义的归一化边界。
package message

import "github.com/nexus-research-lab/nexus/internal/protocol"

const (
	contentFilteredTerminalReason = protocol.ProviderFailureContentFiltered
	contentFilteredDisplayText    = "本轮请求被模型服务的内容安全策略拦截。可能由输入、对话上下文或生成内容触发。您可以调整表述后在当前对话继续；若仍被拦截，再尝试开启新对话。"
)

type providerErrorProjection struct {
	result         string
	terminalReason string
	errors         []string
}

func normalizeProviderContentFilterError(
	result string,
	terminalReason string,
	errors []string,
	additionalSignals ...string,
) providerErrorProjection {
	signals := make([]string, 0, 2+len(errors)+len(additionalSignals))
	signals = append(signals, result, terminalReason)
	signals = append(signals, errors...)
	signals = append(signals, additionalSignals...)
	if !protocol.IsProviderContentFilterError(signals...) {
		return providerErrorProjection{
			result:         result,
			terminalReason: terminalReason,
			errors:         errors,
		}
	}
	return providerErrorProjection{
		result:         contentFilteredDisplayText,
		terminalReason: contentFilteredTerminalReason,
		errors:         []string{contentFilteredTerminalReason},
	}
}
