package message

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (p *Processor) buildResultMessage(message sdkprotocol.ReceivedMessage, subtype string) protocol.Message {
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(strings.TrimSpace(message.UUID), "result_"+p.ctx.RoundID),
		"result",
	)
	payload["subtype"] = subtype
	payload["duration_ms"] = message.Result.DurationMS
	payload["duration_api_ms"] = message.Result.DurationAPIMS
	payload["num_turns"] = message.Result.NumTurns
	payload["total_cost_usd"] = message.Result.TotalCostUSD
	payload["usage"] = firstNonNilMap(message.Result.Usage, map[string]any{})
	payload["result"] = message.Result.Result
	payload["is_error"] = subtype == "error"
	if strings.TrimSpace(message.Result.TerminalReason) != "" {
		payload["terminal_reason"] = strings.TrimSpace(message.Result.TerminalReason)
	}
	if message.Result.StopReason != nil {
		payload["stop_reason"] = message.Result.StopReason
	}
	if denials := projectPermissionDenials(message.Result.PermissionDenials); len(denials) > 0 {
		payload["permission_denials"] = denials
	}
	if len(message.Result.Errors) > 0 {
		payload["errors"] = append([]string(nil), message.Result.Errors...)
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
	switch subtype {
	case "success", "error", "interrupted":
		return subtype
	default:
		if result.IsError {
			return "error"
		}
		return "success"
	}
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
