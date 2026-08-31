// INPUT: 当前会话的 durable history 与目标 Agent 身份。
// OUTPUT: 仅包含稳定失败类别的下一轮隐藏恢复上下文。
// POS: 上一轮终态历史到当前用户轮可解释上下文的应用层边界。
package conversation

import (
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const (
	recoveryAuthenticationFailed = "authentication_failed"
	recoveryBillingError         = "billing_error"
	recoveryInvalidRequest       = "invalid_request"
	recoveryMaxOutputTokens      = "max_output_tokens"
	recoveryRateLimit            = "rate_limit"
	recoveryRuntimeError         = "runtime_error"
	recoveryServerError          = "server_error"
	recoveryUnknown              = "unknown"
)

type terminalOutcome struct {
	failed  bool
	reason  string
	signals []string
}

// RoundRecoveryContextualInputs returns a hidden explanation of the latest failed
// terminal outcome. A later successful or interrupted terminal suppresses older failures.
// Pass an empty agentID for a history store that is already scoped to one Agent.
func RoundRecoveryContextualInputs(history []protocol.Message, agentID string) []runtimectx.ContextualInputBlock {
	outcome, ok := latestTerminalOutcome(history, agentID)
	if !ok || !outcome.failed {
		return nil
	}
	reason := normalizeRecoveryReason(outcome)
	metadata := map[string]string{
		"terminal_reason": reason,
	}
	if agentID = strings.TrimSpace(agentID); agentID != "" {
		metadata["agent_id"] = agentID
	}
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock(
			runtimectx.ContextualInputNameRoundRecovery,
			recoveryInstruction(reason),
			runtimectx.ContextualInputPriorityRoundRecovery,
			metadata,
		),
	}
}

func latestTerminalOutcome(history []protocol.Message, agentID string) (terminalOutcome, bool) {
	agentID = strings.TrimSpace(agentID)
	for index := len(history) - 1; index >= 0; index-- {
		message := history[index]
		if !matchesRecoveryAgent(message, agentID) {
			continue
		}
		switch protocol.MessageRole(message) {
		case "result":
			return terminalOutcomeFrom(message, message), true
		case "assistant":
			if summary, ok := resultSummary(message["result_summary"]); ok {
				return terminalOutcomeFrom(summary, message), true
			}
			if boolValue(message["is_complete"]) {
				return terminalOutcome{}, true
			}
		}
	}
	return terminalOutcome{}, false
}

func terminalOutcomeFrom(terminal map[string]any, message protocol.Message) terminalOutcome {
	subtype := stringValue(terminal["subtype"])
	isError := boolValue(terminal["is_error"]) || subtype == "error"
	signals := []string{
		stringValue(terminal["terminal_reason"]),
		stringValue(terminal["stop_reason"]),
		stringValue(terminal["result"]),
		fmt.Sprint(terminal["errors"]),
		fmt.Sprint(message["content"]),
	}
	if protocol.IsProviderContentFilterError(signals...) {
		isError = true
	}
	return terminalOutcome{
		failed:  isError,
		reason:  stringValue(terminal["terminal_reason"]),
		signals: signals,
	}
}

func matchesRecoveryAgent(message protocol.Message, agentID string) bool {
	if agentID == "" {
		return true
	}
	return stringValue(message["agent_id"]) == agentID
}

func resultSummary(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, len(typed) > 0
	case protocol.Message:
		return map[string]any(typed), len(typed) > 0
	default:
		return nil, false
	}
}

func normalizeRecoveryReason(outcome terminalOutcome) string {
	if protocol.IsProviderContentFilterError(outcome.signals...) {
		return protocol.ProviderFailureContentFiltered
	}
	reason := strings.ToLower(strings.TrimSpace(outcome.reason))
	switch reason {
	case recoveryAuthenticationFailed,
		recoveryBillingError,
		recoveryInvalidRequest,
		recoveryMaxOutputTokens,
		recoveryRateLimit,
		recoveryRuntimeError,
		recoveryServerError,
		recoveryUnknown:
		return reason
	default:
		return recoveryRuntimeError
	}
}

func recoveryInstruction(reason string) string {
	description := map[string]string{
		protocol.ProviderFailureContentFiltered: "The model provider's content-safety policy blocked the prior request or attempted response. The durable record cannot determine whether the trigger came from the latest input, accumulated conversation context, or attempted output.",
		recoveryAuthenticationFailed:            "The prior model request failed authentication.",
		recoveryBillingError:                    "The prior model request was rejected because of an account billing or quota condition.",
		recoveryInvalidRequest:                  "The provider rejected the prior model request as invalid.",
		recoveryMaxOutputTokens:                 "The prior response stopped because it reached the output-token limit.",
		recoveryRateLimit:                       "The provider rate-limited the prior model request.",
		recoveryServerError:                     "The provider or runtime reported a server-side failure.",
		recoveryUnknown:                         "The prior model request failed without a more specific safe reason.",
		recoveryRuntimeError:                    "The prior model or runtime request failed without a safe, specific reason in the durable record.",
	}[reason]
	return fmt.Sprintf(
		"The preceding turn for this agent ended before a complete assistant answer.\nRecorded terminal reason: %s.\n%s\nIf the user asks what happened, explain this recorded reason plainly. Do not claim the prior task completed, invent missing details, or automatically repeat the failed generation. Continue from the user's current instruction.",
		reason,
		description,
	)
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}
