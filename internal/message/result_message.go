// INPUT: runtime terminal result、usage、权限拒绝与 Provider 错误明细。
// OUTPUT: Nexus durable result，并归一化跨 runtime 的失败终态。
// POS: runtime result 到统一消息协议的构造边界。
package message

import (
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const hookStoppedDisplayText = "该操作被当前运行时规则拦截，本轮已停止。"

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

func (p *Processor) buildResultMessage(
	messageID string,
	result sdkprotocol.ResultMessage,
	subtype string,
) protocol.Message {
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(messageID, "result_"+p.ctx.RoundID),
		"result",
	)
	payload["subtype"] = subtype
	payload["duration_ms"] = result.DurationMS
	payload["duration_api_ms"] = result.DurationAPIMS
	payload["num_turns"] = result.NumTurns
	payload["total_cost_usd"] = result.TotalCostUSD
	payload["usage"] = firstNonNilMap(result.Usage, map[string]any{})
	if len(result.ModelUsage) > 0 {
		payload["model_usage"] = cloneMap(result.ModelUsage)
	}
	payload["is_error"] = subtype == "error"
	runtimeSubtype := strings.TrimSpace(result.Subtype)
	if runtimeSubtype != "" && runtimeSubtype != subtype {
		payload["runtime_subtype"] = runtimeSubtype
	}
	terminalReason := strings.TrimSpace(result.TerminalReason)
	errors := slices.Clone(result.Errors)
	resultText := result.Result
	stopReason := result.StopReason
	if subtype == "error" {
		projection := normalizeProviderContentFilterError(
			resultText,
			terminalReason,
			errors,
			normalizeString(result.StopReason),
		)
		resultText = projection.result
		terminalReason = projection.terminalReason
		errors = projection.errors
		if projection.terminalReason == contentFilteredTerminalReason {
			stopReason = "error"
		}
		if runtimeSubtype == "error_hook_stopped" {
			resultText = hookStoppedDisplayText
			errors = []string{hookStoppedDisplayText}
		}
	}
	payload["result"] = resultText
	if terminalReason != "" {
		payload["terminal_reason"] = terminalReason
	}
	if stopReason != nil {
		payload["stop_reason"] = stopReason
	}
	if denials := projectPermissionDenials(result.PermissionDenials); len(denials) > 0 {
		payload["permission_denials"] = denials
	}
	if len(errors) > 0 {
		payload["errors"] = errors
	}
	if result.StructuredOutput != nil {
		payload["structured_output"] = result.StructuredOutput
	}
	if fastModeState := strings.TrimSpace(result.FastModeState); fastModeState != "" {
		payload["fast_mode_state"] = fastModeState
	}
	return protocol.Message(payload)
}

func projectPermissionDenials(items []sdkprotocol.PermissionDenial) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload := map[string]any{}
		if toolName := strings.TrimSpace(item.ToolName); toolName != "" {
			payload["tool_name"] = toolName
		}
		if toolUseID := strings.TrimSpace(item.ToolUseID); toolUseID != "" {
			payload["tool_use_id"] = toolUseID
		}
		if len(item.ToolInput) > 0 {
			payload["tool_input"] = cloneMap(item.ToolInput)
		}
		if len(payload) > 0 {
			result = append(result, payload)
		}
	}
	return result
}

// NormalizeInterruptedOutput 统一把“用户主动停止后 SDK 仍返回 error”的结果收口成 interrupted。
func NormalizeInterruptedOutput(output *Output, interruptReason string) {
	isError := output.ResultSubtype == "error" || output.TerminalStatus == "error"
	isInterrupted := output.ResultSubtype == "interrupted" || output.TerminalStatus == "interrupted"
	if !isError && !isInterrupted {
		return
	}

	if strings.TrimSpace(interruptReason) == "" {
		return
	}
	resultText := NormalizeInterruptDisplayText(interruptReason)
	// SDK 可能直接返回 interrupted，也可能先返回 error。两条路径必须共用
	// 同一展示边界，否则原生 interrupted 的 raw result 会绕过清理后落盘。
	output.ResultSubtype = "interrupted"
	output.TerminalStatus = "interrupted"
	for index := range output.DurableMessages {
		messageValue := output.DurableMessages[index]
		if protocol.MessageRole(messageValue) != "result" {
			continue
		}
		messageValue["subtype"] = "interrupted"
		messageValue["is_error"] = false
		if resultText == "" {
			delete(messageValue, "result")
		} else {
			messageValue["result"] = resultText
		}
		output.DurableMessages[index] = messageValue
	}
}

func normalizeResultSubtype(result sdkprotocol.ResultMessage) string {
	subtype := strings.TrimSpace(result.Subtype)
	if subtype == "interrupted" {
		return "interrupted"
	}
	if result.IsError || subtype == "error" || strings.HasPrefix(subtype, "error_") {
		return "error"
	}
	return "success"
}

func statusFromResultSubtype(subtype string) string {
	switch subtype {
	case "interrupted":
		return "interrupted"
	case "error":
		return "error"
	default:
		return "finished"
	}
}
