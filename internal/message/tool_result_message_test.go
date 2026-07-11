package message

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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

func TestProcessorHandlesToolResultMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-tool-result",
		ParentID:   "round-tool-result",
	}, "")

	// 先注入一个 tool_use，使 enrich 阶段能查到工具名
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-tool-result-1",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{ID: "tool-123", Name: "AskUserQuestion"},
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
						ToolUseID: "tool-123",
						Content:   json.RawMessage(`"Permission request timeout"`),
						IsError:   true,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("tool result 未生成 durable assistant 消息: %+v", output)
	}
	assistantMessage := output.DurableMessages[0]
	if assistantMessage["role"] != "assistant" || assistantMessage["is_complete"] != true {
		t.Fatalf("tool result 生成的 assistant 消息不正确: %+v", assistantMessage)
	}
	blocks, _ := assistantMessage["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("tool result 未正确并入 content: %+v", blocks)
	}
	if blocks[1]["type"] != "tool_result" {
		t.Fatalf("第二块应为 tool_result: %+v", blocks[1])
	}
	if blocks[1]["error_code"] != "permission_request_timeout" {
		t.Fatalf("tool result 未正确附加 error_code: %+v", blocks[1])
	}
}

func TestProcessorDropsUnmatchedSuccessfulToolResultMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-unmatched-tool-result",
		ParentID:   "round-unmatched-tool-result",
	}, "")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "missing-tool",
						Content:   json.RawMessage(`"ok"`),
						IsError:   false,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 0 {
		t.Fatalf("无匹配 tool_use 的成功 tool_result 不应生成 durable 消息: %+v", output.DurableMessages)
	}
}

func TestProcessorKeepsUnmatchedErrorToolResultMessage(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-unmatched-tool-error",
		ParentID:   "round-unmatched-tool-error",
	}, "")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "missing-tool",
						Content:   json.RawMessage(`"failed"`),
						IsError:   true,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("无匹配 tool_use 的错误 tool_result 应保留诊断消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "tool_result" || blocks[0]["is_error"] != true {
		t.Fatalf("错误 tool_result 内容不正确: %+v", blocks)
	}
}
