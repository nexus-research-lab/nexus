package message

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorMergesSequentialAssistantSnapshots(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-merge",
		ParentID:   "round-merge",
	}, "sdk-session-merge")

	first := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ThinkingBlock{Thinking: "先想一下"},
				},
			},
		},
	})
	if len(first.DurableMessages) != 1 {
		t.Fatalf("首次 assistant 快照未输出 durable 消息: %+v", first)
	}
	if first.DurableMessages[0]["is_complete"] != false {
		t.Fatalf("中间 assistant 快照不应提前标记完成: %+v", first.DurableMessages[0])
	}

	second := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         "assistant-merge-1",
				Model:      "glm-5-turbo",
				StopReason: "end_turn",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "最终回答"},
				},
			},
		},
	})
	if len(second.DurableMessages) != 1 {
		t.Fatalf("第二次 assistant 快照未输出 durable 消息: %+v", second)
	}
	if second.DurableMessages[0]["is_complete"] != true {
		t.Fatalf("终态 assistant 快照应标记完成: %+v", second.DurableMessages[0])
	}
	blocks, _ := second.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("assistant 快照未合并 thinking 与 text: %+v", second.DurableMessages[0])
	}
	if blocks[0]["type"] != "thinking" || blocks[1]["type"] != "text" {
		t.Fatalf("assistant 内容块顺序不正确: %+v", blocks)
	}
	if blocks[0]["thinking"] != "先想一下" || blocks[1]["text"] != "最终回答" {
		t.Fatalf("assistant 内容块未正确保留: %+v", blocks)
	}
}

func TestProcessorMergesSequentialAssistantToolUseSnapshots(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-tool-use-merge",
		ParentID:   "round-tool-use-merge",
	}, "sdk-session-tool-use-merge")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-tool-use-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: "看看当前权限状态："},
				},
			},
		},
	})

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-tool-use-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-connectors",
						Name:  "mcp__nexus_connectors__connector_list",
						Input: json.RawMessage(`{}`),
					},
				},
			},
		},
	})

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    "assistant-tool-use-merge-1",
				Model: "glm-5-turbo",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-automation",
						Name:  "mcp__nexus_automation__list_scheduled_tasks",
						Input: json.RawMessage(`{}`),
					},
				},
			},
		},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("第二个 tool_use 快照未输出 durable 消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 {
		t.Fatalf("assistant 快照未保留两个 tool_use: %+v", output.DurableMessages[0])
	}
	if blocks[1]["type"] != "tool_use" || blocks[1]["id"] != "tool-connectors" {
		t.Fatalf("第一个 tool_use 被覆盖: %+v", blocks)
	}
	if blocks[2]["type"] != "tool_use" || blocks[2]["id"] != "tool-automation" {
		t.Fatalf("第二个 tool_use 未追加: %+v", blocks)
	}
}
