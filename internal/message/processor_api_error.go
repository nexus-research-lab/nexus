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
	if !message.Assistant.IsAPIError && strings.TrimSpace(message.Assistant.Error) == "" && strings.TrimSpace(message.Assistant.APIError) == "" {
		return nil
	}
	text := firstNonEmpty(
		assistantTextFromEnvelope(message.Assistant.Message),
		message.Assistant.ErrorDetails,
		message.Assistant.APIError,
		message.Assistant.Error,
		"Runtime API request failed",
	)
	payload := baseMessageEnvelope(
		p.ctx,
		p.sessionID,
		firstNonEmpty(strings.TrimSpace(message.UUID), "result_"+p.ctx.RoundID),
		"result",
	)
	payload["subtype"] = "error"
	payload["duration_ms"] = 0
	payload["duration_api_ms"] = 0
	payload["num_turns"] = 0
	payload["usage"] = map[string]any{}
	payload["result"] = text
	payload["is_error"] = true
	if reason := firstNonEmpty(message.Assistant.Error, message.Assistant.APIError); reason != "" {
		payload["terminal_reason"] = reason
		payload["errors"] = []string{reason}
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
