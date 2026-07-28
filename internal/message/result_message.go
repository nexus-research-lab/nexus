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

func (p *Processor) buildResultMessage(message sdkprotocol.ReceivedMessage, subtype string) protocol.Message {
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(message.UUID, "result_"+p.ctx.RoundID),
		"result",
	)
	if message.Result == nil {
		// 外部 provider 的坏包不能把宿主 round 直接打崩；按 CC 的失败
		// 终态语义生成可展示、可追踪的最小 result。
		payload["subtype"] = "error"
		payload["is_error"] = true
		payload["errors"] = []string{"result payload missing"}
		payload["result"] = "result payload missing"
		return protocol.Message(payload)
	}
	payload["subtype"] = subtype
	payload["duration_ms"] = message.Result.DurationMS
	payload["duration_api_ms"] = message.Result.DurationAPIMS
	payload["num_turns"] = message.Result.NumTurns
	payload["total_cost_usd"] = message.Result.TotalCostUSD
	payload["usage"] = firstNonNilMap(message.Result.Usage, map[string]any{})
	if len(message.Result.ModelUsage) > 0 {
		payload["model_usage"] = cloneMap(message.Result.ModelUsage)
	}
	payload["is_error"] = subtype == "error"
	if runtimeSubtype := strings.TrimSpace(message.Result.Subtype); runtimeSubtype != "" && runtimeSubtype != subtype {
		payload["runtime_subtype"] = runtimeSubtype
	}
	terminalReason := strings.TrimSpace(message.Result.TerminalReason)
	errors := slices.Clone(message.Result.Errors)
	resultText := message.Result.Result
	stopReason := message.Result.StopReason
	if subtype == "error" {
		projection := normalizeProviderContentFilterError(
			resultText,
			terminalReason,
			errors,
			normalizeString(message.Result.StopReason),
		)
		resultText = projection.result
		terminalReason = projection.terminalReason
		errors = projection.errors
		if projection.terminalReason == contentFilteredTerminalReason {
			stopReason = "error"
		}
	}
	payload["result"] = resultText
	if terminalReason != "" {
		payload["terminal_reason"] = terminalReason
	}
	if stopReason != nil {
		payload["stop_reason"] = stopReason
	}
	if denials := projectPermissionDenials(message.Result.PermissionDenials); len(denials) > 0 {
		payload["permission_denials"] = denials
	}
	if len(errors) > 0 {
		payload["errors"] = errors
	}
	if message.Result.StructuredOutput != nil {
		payload["structured_output"] = message.Result.StructuredOutput
	}
	if fastModeState := strings.TrimSpace(message.Result.FastModeState); fastModeState != "" {
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
	if output == nil {
		return
	}
	if output.ResultSubtype != "error" && output.TerminalStatus != "error" {
		return
	}

	resultText := strings.TrimSpace(interruptReason)
	if resultText == "" {
		return
	}
	if resultText == InterruptWithoutMessage {
		resultText = ""
	}
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

func normalizeResultSubtype(result *sdkprotocol.ResultMessage) string {
	if result == nil {
		return "error"
	}
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
