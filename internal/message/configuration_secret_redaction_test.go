// INPUT: malformed configuration tool calls whose streamed or final input contains direct secret values.
// OUTPUT: stream and durable projections that retain safe placeholders but never expose direct secret material.
// POS: regression tests for the message projection side of the human-only configuration secret boundary.
package message

import (
	"encoding/json"
	"strings"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestConfigurationToolSecretsAreRedactedFromDurableAssistantMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:secret-redaction",
		AgentID:    "nexus",
		RoundID:    "round-secret-redaction",
	}, "sdk-secret-redaction")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-secret-redaction",
				StopReason: "tool_use",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:   "tool-secret-redaction",
						Name: "mcp__nexus_config__apply_nexus_configuration_change",
						Input: json.RawMessage(`{
							"domain":"providers",
							"input":{
								"auth_token":"topsecret-final-987",
								"client_secret":{"$secret":"provider.client_secret"}
							}
						}`),
					},
				},
			},
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("durable messages = %#v", output.DurableMessages)
	}
	assertConfigurationProjectionRedacted(t, output.DurableMessages[0])
}

func TestConfigurationToolSecretsAreRedactedFromStreamStartAndDelta(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:stream-secret-redaction",
		AgentID:    "nexus",
		RoundID:    "round-stream-secret-redaction",
	}, "sdk-stream-secret-redaction")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-stream-secret-redaction"},
		}},
	})

	start := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type": "tool_use",
				"id":   "tool-stream-secret-redaction",
				"name": "mcp__nexus_config__apply_nexus_configuration_change",
				"input": map[string]any{
					"domain": "providers",
					"input": map[string]any{
						"auth_token": "topsecret-start-987",
					},
				},
			},
		}},
	})
	if len(start.StreamEvents) != 1 {
		t.Fatalf("start stream events = %#v", start.StreamEvents)
	}
	assertConfigurationProjectionRedacted(t, start.StreamEvents[0].Data)

	deltaProcessor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:stream-delta-secret-redaction",
		AgentID:    "nexus",
		RoundID:    "round-stream-delta-secret-redaction",
	}, "sdk-stream-delta-secret-redaction")
	deltaProcessor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": "assistant-stream-delta-secret-redaction"},
		}},
	})
	deltaProcessor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    "tool-stream-delta-secret-redaction",
				"name":  "mcp__nexus_config__apply_nexus_configuration_change",
				"input": map[string]any{},
			},
		}},
	})
	delta := deltaProcessor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeStreamEvent,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type": "input_json_delta",
				"partial_json": `{
					"domain":"providers",
					"input":{"auth_token":"topsecret-delta-987"}
				}`,
			},
		}},
	})
	if len(delta.StreamEvents) != 1 {
		t.Fatalf("delta stream events = %#v", delta.StreamEvents)
	}
	assertConfigurationProjectionRedacted(t, delta.StreamEvents[0].Data)
}

func assertConfigurationProjectionRedacted(t *testing.T, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	for _, forbidden := range []string{
		"topsecret-final-987",
		"topsecret-start-987",
		"topsecret-delta-987",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("configuration projection leaked %q: %s", forbidden, payload)
		}
	}
}
