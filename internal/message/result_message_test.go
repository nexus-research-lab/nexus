package message

import (
	"testing"

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

func TestProcessorProjectsMissingResultPayloadAsVisibleError(t *testing.T) {
	processor := NewProcessor(MessageContext{RoundID: "round-missing-result"}, "session-missing-result")
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeResult,
		UUID: "result-missing-payload",
	})
	if output.TerminalStatus != "error" || output.ResultSubtype != "error" {
		t.Fatalf("missing result terminal state = %+v", output)
	}
	if len(output.DurableMessages) != 1 || output.DurableMessages[0]["is_error"] != true {
		t.Fatalf("missing result should produce visible error: %+v", output.DurableMessages)
	}
}
