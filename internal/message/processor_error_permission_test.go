package message

import (
	"encoding/json"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorPreservesResultPermissionDenials(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-denied",
		ParentID:   "round-denied",
	}, "sdk-session-denied")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-denied",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
			Result:  "无法完成搜索：WebSearch 未被允许",
			PermissionDenials: []sdkprotocol.PermissionDenial{{
				ToolName:  "WebSearch",
				ToolUseID: "tool-1",
				ToolInput: map[string]any{"query": "AI news"},
			}},
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("result durable 消息数量不正确: %+v", output.DurableMessages)
	}
	denials, ok := output.DurableMessages[0]["permission_denials"].([]map[string]any)
	if !ok || len(denials) != 1 {
		t.Fatalf("permission_denials 未保留: %+v", output.DurableMessages[0])
	}
	if denials[0]["tool_name"] != "WebSearch" || denials[0]["tool_use_id"] != "tool-1" {
		t.Fatalf("permission_denials 内容不正确: %+v", denials)
	}
	input, ok := denials[0]["tool_input"].(map[string]any)
	if !ok || input["query"] != "AI news" {
		t.Fatalf("permission_denials.tool_input 未保留: %+v", denials)
	}
}

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

func TestProcessorDoesNotPersistApiRetrySystemMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-api-retry",
		ParentID:   "round-api-retry",
	}, "sdk-session-api-retry")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeSystem,
		System: &sdkprotocol.SystemMessage{
			Subtype: "api_retry",
			Data: map[string]any{
				"message": "API 正在重试",
			},
		},
	})

	if len(output.DurableMessages) != 0 {
		t.Fatalf("api_retry 不应生成 durable 消息: %+v", output.DurableMessages)
	}
	if len(output.EphemeralMessages) != 1 {
		t.Fatalf("api_retry 应生成一条 ephemeral 消息: %+v", output)
	}
	if output.EphemeralMessages[0]["message_id"] != "system_api_retry_round-api-retry" {
		t.Fatalf("api_retry 应使用稳定 message_id: %+v", output.EphemeralMessages[0])
	}
}

func TestProcessorNormalizesSystemAPIErrorMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-api-error",
		ParentID:   "round-api-error",
	}, "sdk-session-api-error")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type:    sdkprotocol.MessageTypeSystem,
		Subtype: "api_error",
		System: &sdkprotocol.SystemMessage{
			Subtype: "api_error",
			Data: map[string]any{
				"retryAttempt": 4,
				"maxRetries":   11,
				"retryInMs":    3000,
				"error": map[string]any{
					"status": 529,
					"type":   "overloaded_error",
				},
			},
		},
	})

	if len(output.DurableMessages) != 0 || len(output.EphemeralMessages) != 1 {
		t.Fatalf("api_error 应只生成 ephemeral 消息: %+v", output)
	}
	message := output.EphemeralMessages[0]
	if message["content"] != "模型请求暂时受限，正在自动重试。" {
		t.Fatalf("content = %#v", message["content"])
	}
	metadata, ok := message["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata 类型不正确: %#v", message["metadata"])
	}
	for key, want := range map[string]any{
		"subtype":        "api_retry",
		"attempt":        4,
		"max_retries":    11,
		"retry_delay_ms": 3000,
		"error_status":   529,
		"error":          "rate_limit",
	} {
		if got := metadata[key]; got != want {
			t.Fatalf("%s = %#v, want %#v", key, got, want)
		}
	}
}

func TestProcessorPersistsCompactBoundarySystemMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-compact",
		ParentID:   "round-compact",
	}, "sdk-session-compact")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type:    sdkprotocol.MessageTypeSystem,
		Subtype: "compact_boundary",
		System: &sdkprotocol.SystemMessage{
			Subtype: "compact_boundary",
			Data: map[string]any{
				"compact_metadata": map[string]any{
					"trigger":    "auto",
					"pre_tokens": 120000,
				},
			},
		},
	})

	if len(output.EphemeralMessages) != 0 || len(output.DurableMessages) != 1 {
		t.Fatalf("compact_boundary 应生成 durable 消息: %+v", output)
	}
	message := output.DurableMessages[0]
	if message["message_id"] != "system_compact_boundary_round-compact" {
		t.Fatalf("message_id 不正确: %+v", message)
	}
	if message["content"] != "上下文已压缩" {
		t.Fatalf("content = %#v", message["content"])
	}
	metadata, ok := message["metadata"].(map[string]any)
	if !ok || metadata["subtype"] != "compact_boundary" {
		t.Fatalf("metadata 不正确: %+v", message["metadata"])
	}
	compactMetadata, ok := metadata["compact_metadata"].(map[string]any)
	if !ok || compactMetadata["trigger"] != "auto" || compactMetadata["pre_tokens"] != 120000 {
		t.Fatalf("compact_metadata 未保留: %+v", metadata)
	}
}

func TestProcessorEnrichesPermissionErrorCode(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-perm-code",
		ParentID:   "round-perm-code",
	}, "")

	// 注入 tool_use
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-456", Name: "AskUserQuestion"},
				},
			},
		},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-456",
						Content:   json.RawMessage(`"Permission channel unavailable"`),
						IsError:   true,
					},
				},
			},
		},
	})

	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if blocks[1]["error_code"] != "permission_channel_unavailable" {
		t.Fatalf("error_code 推断不正确: %+v", blocks[1])
	}
}
