package message

import (
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

func TestProcessorTreatsSuccessSubtypeWithErrorFlagAsFailure(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-error-flag",
		ParentID:   "round-error-flag",
	}, "sdk-session-error")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-error-flag",
		Result: &sdkprotocol.ResultMessage{
			Subtype:          "success",
			IsError:          true,
			Result:           "provider reported an error",
			ModelUsage:       map[string]any{"glm-5.2": map[string]any{"input_tokens": 42}},
			StructuredOutput: map[string]any{"status": "failed"},
			FastModeState:    "cooldown",
		},
	})

	if output.ResultSubtype != "error" || output.TerminalStatus != "error" {
		t.Fatalf("output terminal = (%q, %q), want error", output.ResultSubtype, output.TerminalStatus)
	}
	if len(output.DurableMessages) != 1 {
		t.Fatalf("durable messages = %+v", output.DurableMessages)
	}
	result := output.DurableMessages[0]
	if result["is_error"] != true || result["subtype"] != "error" || result["runtime_subtype"] != "success" {
		t.Fatalf("result error semantics = %+v", result)
	}
	if result["fast_mode_state"] != "cooldown" || result["structured_output"] == nil || result["model_usage"] == nil {
		t.Fatalf("result diagnostics missing = %+v", result)
	}
}

func TestProcessorNormalizesProviderContentFilterResultError(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:room:test",
		RoomID:     "room-test",
		AgentID:    "agent-test",
		RoundID:    "round-content-filtered",
	}, "sdk-session-content-filtered")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-content-filtered",
		Result: &sdkprotocol.ResultMessage{
			Subtype:        "error",
			IsError:        true,
			Result:         "request rejected",
			StopReason:     "sensitive",
			TerminalReason: "invalid_request",
			Errors:         []string{"generation stopped by Provider"},
		},
	})

	if output.TerminalStatus != "error" || output.ResultSubtype != "error" {
		t.Fatalf("content filter should remain a terminal error: %+v", output)
	}
	result := output.DurableMessages[0]
	if result["result"] != contentFilteredDisplayText {
		t.Fatalf("terminal result did not use fallback copy: %+v", result)
	}
	if result["terminal_reason"] != contentFilteredTerminalReason {
		t.Fatalf("terminal reason was not normalized: %+v", result)
	}
	if result["stop_reason"] != "error" {
		t.Fatalf("Provider stop reason was not normalized: %+v", result)
	}
	errors, ok := result["errors"].([]string)
	if !ok || len(errors) != 1 || errors[0] != contentFilteredTerminalReason {
		t.Fatalf("raw terminal error should not remain user-visible: %+v", result)
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

func TestProcessorNormalizesClaudeCodeErrorSubtype(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-error-subtype"}, "")
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{
			Subtype: "error_max_turns",
		},
	})
	if output.ResultSubtype != "error" || output.TerminalStatus != "error" {
		t.Fatalf("CC error subtype = (%q, %q), want error", output.ResultSubtype, output.TerminalStatus)
	}
}

func TestProcessorNormalizesHookStoppedResultError(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-hook-stopped"}, "")
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "error_hook_stopped",
			IsError:    true,
			StopReason: "hook_stopped",
			Errors:     []string{"Tool execution stopped by hook"},
		},
	})
	if output.ResultSubtype != "error" || output.TerminalStatus != "error" {
		t.Fatalf("hook stopped terminal = (%q, %q), want error", output.ResultSubtype, output.TerminalStatus)
	}
	result := output.DurableMessages[0]
	if result["runtime_subtype"] != "error_hook_stopped" {
		t.Fatalf("runtime subtype 未保留: %+v", result)
	}
	errors, ok := result["errors"].([]string)
	if !ok || len(errors) != 1 || errors[0] != hookStoppedDisplayError {
		t.Fatalf("hook stopped 错误未使用友好文案: %+v", result)
	}
	if result["result"] != "" {
		t.Fatalf("hook stopped 不应把内部英文错误投影为正文: %+v", result)
	}
}

func TestProcessorProjectsDecodedEmptyResultPayload(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-empty-result"}, "session-empty-result")
	decoded, err := sdkprotocol.DecodeMessage(map[string]any{
		"type": "result",
		"uuid": "result-empty-payload",
	})
	if err != nil {
		t.Fatalf("DecodeMessage() error = %v", err)
	}
	output := processor.Process(decoded)
	if output.TerminalStatus != "finished" || output.ResultSubtype != "success" {
		t.Fatalf("empty result terminal state = %+v", output)
	}
	if len(output.DurableMessages) != 1 || output.DurableMessages[0]["is_error"] != false {
		t.Fatalf("empty result should produce a successful terminal message: %+v", output.DurableMessages)
	}
}

func TestNormalizeInterruptedOutputHidesInternalInterruptSentinel(t *testing.T) {
	output := Output{
		ResultSubtype:  "error",
		TerminalStatus: "error",
		DurableMessages: []protocol.Message{{
			"role":     "result",
			"subtype":  "error",
			"is_error": true,
			"result":   "runtime canceled",
		}},
	}

	NormalizeInterruptedOutput(&output, InterruptWithoutMessage)

	if output.ResultSubtype != "interrupted" || output.TerminalStatus != "interrupted" {
		t.Fatalf("内部中断应收口为 interrupted: %+v", output)
	}
	result := output.DurableMessages[0]
	if result["is_error"] != false || result["subtype"] != "interrupted" {
		t.Fatalf("中断 result 语义不正确: %+v", result)
	}
	if _, exists := result["result"]; exists {
		t.Fatalf("内部中断哨兵不应投影为 result: %+v", result)
	}
}
