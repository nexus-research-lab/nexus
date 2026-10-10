package message

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorAddsImagegenArtifactFromBashResult(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey:    "agent:nexus:ws:dm:test",
		AgentID:       "nexus",
		WorkspacePath: t.TempDir(),
		RoundID:       "round-imagegen-artifact",
		ParentID:      "round-imagegen-artifact",
	}, "")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-imagegen-artifact-1",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-bash-imagegen-1",
						Name:  "Bash",
						Input: json.RawMessage(`{"command":"nexusctl imagegen generate --prompt fox --file-name fox"}`),
					},
				},
			},
		},
	})

	imagegenOutput := `{"domain":"imagegen","action":"generate","item":{"provider":"azure","model":"gpt-image-2","path":"output/imagegen/fox.png","mime_type":"image/png","markdown":"![generated image](output/imagegen/fox.png)"},"payload_bytes":123}`
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolResultBlock{
						ToolUseID: "tool-bash-imagegen-1",
						Content:   json.RawMessage(strconv.Quote(imagegenOutput)),
						IsError:   false,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("imagegen artifact 未生成 durable assistant 消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 {
		t.Fatalf("imagegen artifact 内容块数量不正确: %+v", blocks)
	}
	artifact := blocks[2]
	if artifact["type"] != protocol.ContentBlockTypeWorkspaceFileArtifact {
		t.Fatalf("第三块应为 workspace_file_artifact: %+v", artifact)
	}
	if artifact["path"] != "output/imagegen/fox.png" || artifact["artifact_kind"] != protocol.WorkspaceFileArtifactKindImage {
		t.Fatalf("imagegen artifact 路径或类型不正确: %+v", artifact)
	}
	if artifact["mime_type"] != "image/png" || artifact["label"] != "生成图片" {
		t.Fatalf("imagegen artifact 元数据不正确: %+v", artifact)
	}
	if artifact["source_tool_use_id"] != "tool-bash-imagegen-1" || artifact["source_tool_name"] != "Bash" {
		t.Fatalf("imagegen artifact 来源工具不正确: %+v", artifact)
	}
}
