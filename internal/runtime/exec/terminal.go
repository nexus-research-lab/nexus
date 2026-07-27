package exec

import (
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func terminalRoundResult(
	mapResult RoundMapResult,
	assistantTerminalResult *RoundExecutionResult,
	resultMessage *sdkprotocol.ResultMessage,
	startedAt time.Time,
) RoundExecutionResult {
	result := RoundExecutionResult{
		TerminalStatus:   strings.TrimSpace(mapResult.TerminalStatus),
		ResultSubtype:    strings.TrimSpace(mapResult.ResultSubtype),
		ErrorMessage:     terminalErrorMessage(mapResult),
		TerminalCategory: sdkprotocol.TerminalCategoryUnknown,
	}
	if resultMessage != nil {
		observed := observedResultMessage(resultMessage, startedAt)
		result.Usage = observed.Usage
		result.TerminalCategory = observed.TerminalCategory
		result.UsageLimitReached = observed.UsageLimitReached
		result.UsageLimitReason = observed.UsageLimitReason
	}
	if !isSuccessfulRoundResult(result) {
		return roundResultWithElapsed(result, startedAt)
	}
	if assistantResult, ok := terminalAssistantResult(mapResult); ok && assistantResult.CompletedByAssistant {
		result.CompletedByAssistant = true
		return roundResultWithElapsed(result, startedAt)
	}
	if hasSuccessfulResultMessage(mapResult) {
		result.CompletedByAssistant = true
		return roundResultWithElapsed(result, startedAt)
	}
	if assistantTerminalResult != nil && assistantTerminalResult.CompletedByAssistant {
		result.CompletedByAssistant = true
	}
	return roundResultWithElapsed(result, startedAt)
}

// observedResultMessage preserves provider accounting independently from local
// mapping, persistence, and event delivery. The returned status intentionally
// remains non-terminal: callers still follow the local error path, while Goal
// settlement can reconcile the provider usage that already arrived.
func observedResultMessage(
	resultMessage *sdkprotocol.ResultMessage,
	startedAt time.Time,
) RoundExecutionResult {
	if resultMessage == nil {
		return RoundExecutionResult{}
	}
	result := RoundExecutionResult{
		ResultSubtype:    strings.TrimSpace(resultMessage.Subtype),
		TerminalCategory: resultMessage.TerminalCategory(),
	}
	result.Usage, _ = resultMessage.TokenUsage()
	result.UsageLimitReached, result.UsageLimitReason = runtimectx.ResultUsageLimitReached(resultMessage)
	return roundResultWithElapsed(result, startedAt)
}

func isSuccessfulRoundResult(result RoundExecutionResult) bool {
	return result.TerminalStatus == "finished" &&
		(result.ResultSubtype == "" || result.ResultSubtype == "success")
}

func hasSuccessfulResultMessage(mapResult RoundMapResult) bool {
	for _, messageValue := range mapResult.DurableMessages {
		if messageValue == nil || protocol.MessageRole(messageValue) != "result" {
			continue
		}
		if messageString(messageValue["subtype"]) == "error" || messageValue["is_error"] == true {
			continue
		}
		return true
	}
	return false
}

func terminalErrorMessage(mapResult RoundMapResult) string {
	for _, messageValue := range mapResult.DurableMessages {
		if messageValue == nil || protocol.MessageRole(messageValue) != "result" {
			continue
		}
		if messageString(messageValue["subtype"]) != "error" && messageValue["is_error"] != true {
			continue
		}
		if resultText := strings.TrimSpace(messageString(messageValue["result"])); resultText != "" {
			return resultText
		}
		if terminalReason := strings.TrimSpace(messageString(messageValue["terminal_reason"])); terminalReason != "" {
			return terminalReason
		}
		if errorsText := terminalErrorsText(messageValue["errors"]); errorsText != "" {
			return errorsText
		}
	}
	if mapResult.ResultSubtype == "error" || mapResult.TerminalStatus == "error" {
		return "Runtime request failed"
	}
	return ""
}

func terminalErrorsText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return strings.TrimSpace(strings.Join(trimNonEmptyStrings(typed), "; "))
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(messageString(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "; "))
	default:
		return ""
	}
}

func trimNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func terminalAssistantResult(mapResult RoundMapResult) (RoundExecutionResult, bool) {
	for _, messageValue := range mapResult.DurableMessages {
		if messageValue == nil || protocol.MessageRole(messageValue) != "assistant" {
			continue
		}
		if messageValue["is_complete"] != true {
			continue
		}
		if !isTerminalAssistantStopReason(messageString(messageValue["stop_reason"])) {
			continue
		}
		return RoundExecutionResult{
			TerminalStatus:       "finished",
			ResultSubtype:        "success",
			CompletedByAssistant: true,
		}, true
	}
	return RoundExecutionResult{}, false
}

func isTerminalAssistantStopReason(stopReason string) bool {
	switch strings.TrimSpace(stopReason) {
	case "end_turn", "stop_sequence", "max_tokens":
		return true
	default:
		return false
	}
}
