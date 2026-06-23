package runtime

import (
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

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
		result.Usage, _ = resultMessage.TokenUsage()
		result.TerminalCategory = resultMessage.TerminalCategory()
		result.UsageLimitReached, result.UsageLimitReason = ResultUsageLimitReached(resultMessage)
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
	}
	if mapResult.ResultSubtype == "error" || mapResult.TerminalStatus == "error" {
		return "Runtime request failed"
	}
	return ""
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
