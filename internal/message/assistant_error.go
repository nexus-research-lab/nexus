// INPUT: runtime assistant API 错误与 Provider 返回的错误正文。
// OUTPUT: 统一的 error result，并归一化已知的内容安全拦截。
// POS: assistant API 失败到 Nexus 终态消息的投影入口。
package message

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func (p *Processor) processAssistantAPIError(message sdkprotocol.ReceivedMessage) *protocol.Message {
	if message.Assistant == nil {
		return nil
	}
	assistantError := strings.TrimSpace(message.Assistant.Error)
	assistantAPIError := strings.TrimSpace(message.Assistant.APIError)
	if !message.Assistant.IsAPIError && assistantError == "" && assistantAPIError == "" {
		return nil
	}
	text := firstNonEmpty(
		assistantTextFromEnvelope(message.Assistant.Message),
		message.Assistant.ErrorDetails,
		assistantAPIError,
		assistantError,
		"Runtime API request failed",
	)
	reason := firstNonEmpty(assistantError, assistantAPIError)
	errors := []string(nil)
	if reason != "" {
		errors = []string{reason}
	}
	projection := normalizeProviderContentFilterError(
		text,
		reason,
		errors,
		message.Assistant.ErrorDetails,
		assistantAPIError,
		assistantError,
		normalizeString(message.Assistant.Message.StopReason),
	)
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(message.UUID, "result_"+p.ctx.RoundID),
		"result",
	)
	payload["subtype"] = "error"
	payload["duration_ms"] = 0
	payload["duration_api_ms"] = 0
	payload["num_turns"] = 0
	payload["usage"] = map[string]any{}
	payload["result"] = projection.result
	payload["is_error"] = true
	if projection.terminalReason != "" {
		payload["terminal_reason"] = projection.terminalReason
	}
	if len(projection.errors) > 0 {
		payload["errors"] = projection.errors
	}
	result := protocol.Message(payload)
	return &result
}

func assistantTextFromEnvelope(envelope sdkprotocol.ConversationEnvelope) string {
	blocks := normalizeContentBlocks(envelope.Content)
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if normalizeString(block["type"]) != "text" {
			continue
		}
		text := normalizeString(block["text"])
		if text != "" {
			texts = append(texts, text)
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n\n"))
}
