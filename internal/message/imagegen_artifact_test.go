package message

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorAddsImagegenArtifactFromMCPResult(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey:    "agent:nexus:ws:dm:test",
		AgentID:       "nexus",
		WorkspacePath: t.TempDir(),
		RoundID:       "round-imagegen-mcp-artifact",
		ParentID:      "round-imagegen-mcp-artifact",
	}, "")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-imagegen-mcp-artifact-1",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-imagegen-1",
						Name:  "mcp__nexus_imagegen__generate_image",
						Input: json.RawMessage(`{"prompt":"fox","file_name":"fox"}`),
					},
				},
			},
		},
	})

	imagegenOutput := `{"domain":"imagegen","action":"generate_image","item":{"provider":"openai","model":"gpt-image","path":"output/imagegen/fox.png","mime_type":"image/png","markdown":"![generated image](output/imagegen/fox.png)"},"payload_bytes":123}`
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-imagegen-1",
						Content:   json.RawMessage(strconv.Quote(imagegenOutput)),
						IsError:   false,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("imagegen MCP artifact 未生成 durable assistant 消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 {
		t.Fatalf("imagegen MCP artifact 内容块数量不正确: %+v", blocks)
	}
	artifact := blocks[2]
	if artifact["type"] != protocol.ContentBlockTypeWorkspaceFileArtifact {
		t.Fatalf("第三块应为 workspace_file_artifact: %+v", artifact)
	}
	if artifact["path"] != "output/imagegen/fox.png" || artifact["artifact_kind"] != protocol.WorkspaceFileArtifactKindImage {
		t.Fatalf("imagegen MCP artifact 路径或类型不正确: %+v", artifact)
	}
	if artifact["source_tool_use_id"] != "tool-imagegen-1" ||
		artifact["source_tool_name"] != "mcp__nexus_imagegen__generate_image" {
		t.Fatalf("imagegen MCP artifact 来源工具不正确: %+v", artifact)
	}
}
