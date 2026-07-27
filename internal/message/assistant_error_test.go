package message

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorTreatsAssistantAPIErrorAsTerminalErrorResult(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-api-error",
	}, "sdk-session-api-error")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Error:      "authentication_failed",
			IsAPIError: true,
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-api-error",
				Model:      "<synthetic>",
				StopReason: "stop_sequence",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "Failed to authenticate. API Error: 401 invalid key"},
				},
			},
		},
	})

	if output.TerminalStatus != "error" || output.ResultSubtype != "error" {
		t.Fatalf("API error assistant should terminate as error: %+v", output)
	}
	if len(output.DurableMessages) != 1 {
		t.Fatalf("expected one durable result message, got %+v", output.DurableMessages)
	}
	result := output.DurableMessages[0]
	if protocol.MessageRole(result) != "result" || result["is_error"] != true {
		t.Fatalf("API error should be projected as result error: %+v", result)
	}
	if result["result"] != "Failed to authenticate. API Error: 401 invalid key" {
		t.Fatalf("unexpected API error text: %+v", result)
	}
	if result["terminal_reason"] != "authentication_failed" {
		t.Fatalf("missing terminal reason: %+v", result)
	}
}

func TestProcessorNormalizesProviderContentFilterAssistantError(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-content-filtered",
	}, "sdk-session-content-filtered")

	const providerMessage = "[1301][系统检测到输入或生成内容可能包含不安全或敏感内容，请您避免输入易产生敏感内容的提示语]"
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Error:      "invalid_request",
			IsAPIError: true,
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-content-filtered",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: providerMessage},
				},
			},
		},
	})

	if output.TerminalStatus != "error" || output.ResultSubtype != "error" {
		t.Fatalf("content filter should remain a terminal error: %+v", output)
	}
	if len(output.DurableMessages) != 1 {
		t.Fatalf("expected one durable result message, got %+v", output.DurableMessages)
	}
	result := output.DurableMessages[0]
	if result["result"] != contentFilteredDisplayText {
		t.Fatalf("raw Provider error leaked into visible result: %+v", result)
	}
	if result["terminal_reason"] != contentFilteredTerminalReason {
		t.Fatalf("terminal reason was not normalized: %+v", result)
	}
	errors, ok := result["errors"].([]string)
	if !ok || len(errors) != 1 || errors[0] != contentFilteredTerminalReason {
		t.Fatalf("error details were not normalized: %+v", result)
	}
	projected := ProjectResultMessage(nil, result)
	if projected == nil || ExtractAssistantDisplayText(projected) != contentFilteredDisplayText {
		t.Fatalf("fallback copy was not projected into the conversation: %+v", projected)
	}
}
