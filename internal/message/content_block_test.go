package message

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestNormalizeContentBlocksPreservesImagePayload(t *testing.T) {
	blocks := normalizeContentBlocks([]sdkprotocol.ContentBlock{
		sdkprotocol.ImageBlock{
			Data:     "ZmFrZS1pbWFnZQ==",
			MIMEType: "image/png",
		},
	})
	if len(blocks) != 1 {
		t.Fatalf("image block 数量不正确: %+v", blocks)
	}
	if blocks[0]["type"] != "image" || blocks[0]["data"] != "ZmFrZS1pbWFnZQ==" || blocks[0]["mime_type"] != "image/png" {
		t.Fatalf("image block 未保留 data/mime_type: %+v", blocks[0])
	}
}

func TestProcessorNormalizesServerToolAliases(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-alias",
		ParentID:   "round-alias",
	}, "")

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-alias-1",
						Name:  "SearchWeb",
						Input: json.RawMessage(`{"query":"test"}`),
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("durable 消息数量不正确: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "tool_use" {
		t.Fatalf("server_tool_use 未被映射为 tool_use: %+v", blocks)
	}
}

func TestNormalizeContentBlockMapsServerToolAliases(t *testing.T) {
	block := normalizeContentBlock(map[string]any{
		"type": "server_tool_use",
		"id":   "t1",
		"name": "WebSearch",
	})
	if block["type"] != "tool_use" {
		t.Fatalf("server_tool_use 未映射为 tool_use: %+v", block)
	}

	block = normalizeContentBlock(map[string]any{
		"type":        "server_tool_result",
		"tool_use_id": "t1",
		"content":     "result",
		"is_error":    false,
	})
	if block["type"] != "tool_result" {
		t.Fatalf("server_tool_result 未映射为 tool_result: %+v", block)
	}
}
