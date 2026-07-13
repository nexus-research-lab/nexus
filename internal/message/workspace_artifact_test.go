package message

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorAddsWorkspaceFileArtifactForFileToolResult(t *testing.T) {
	workspacePath := t.TempDir()
	processor := NewProcessor(MessageContext{
		SessionKey:    "agent:nexus:ws:dm:test",
		AgentID:       "nexus",
		WorkspacePath: workspacePath,
		RoundID:       "round-file-artifact",
		ParentID:      "round-file-artifact",
	}, "")

	targetPath := filepath.Join(workspacePath, "reports", "summary.md")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-file-artifact-1",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-write-1",
						Name:  "Write",
						Input: json.RawMessage(`{"file_path":` + strconv.Quote(targetPath) + `}`),
					},
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
						ToolUseID: "tool-write-1",
						Content:   json.RawMessage(`"ok"`),
						IsError:   false,
					},
				},
			},
		},
	})

	if len(output.DurableMessages) != 1 {
		t.Fatalf("file artifact 未生成 durable assistant 消息: %+v", output)
	}
	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 3 {
		t.Fatalf("file artifact 内容块数量不正确: %+v", blocks)
	}
	artifact := blocks[2]
	if artifact["type"] != protocol.ContentBlockTypeWorkspaceFileArtifact {
		t.Fatalf("第三块应为 workspace_file_artifact: %+v", artifact)
	}
	if artifact["path"] != "reports/summary.md" || artifact["display_path"] != "reports/summary.md" {
		t.Fatalf("artifact 路径应转成 workspace 相对路径: %+v", artifact)
	}
	if artifact["source_tool_use_id"] != "tool-write-1" || artifact["source_tool_name"] != "Write" {
		t.Fatalf("artifact 来源工具不正确: %+v", artifact)
	}
	if artifact["workspace_agent_id"] != "nexus" || artifact["scope"] != protocol.WorkspaceFileArtifactScopeAgentWorkspace {
		t.Fatalf("artifact workspace 归属不正确: %+v", artifact)
	}
	if artifact["artifact_kind"] != protocol.WorkspaceFileArtifactKindMarkdown || artifact["mime_type"] != "text/markdown" {
		t.Fatalf("artifact 类型推断不正确: %+v", artifact)
	}
	if artifact["title"] != "summary.md" {
		t.Fatalf("artifact title 不正确: %+v", artifact)
	}
}

func TestProcessorInfersWorkspaceFileArtifactKinds(t *testing.T) {
	cases := []struct {
		path string
		kind string
		mime string
	}{
		{"output/site.html", protocol.WorkspaceFileArtifactKindHTML, "text/html"},
		{"output/flow.mmd", protocol.WorkspaceFileArtifactKindMermaid, "text/plain"},
		{"output/poster.png", protocol.WorkspaceFileArtifactKindImage, "image/png"},
		{"output/vector.svg", protocol.WorkspaceFileArtifactKindSVG, "image/svg+xml"},
		{"output/report.pdf", protocol.WorkspaceFileArtifactKindPDF, "application/pdf"},
	}

	for _, item := range cases {
		t.Run(item.path, func(t *testing.T) {
			kind, mimeType := workspaceFileArtifactKindAndMIME(item.path, "")
			if kind != item.kind || mimeType != item.mime {
				t.Fatalf("artifact 类型推断不正确: path=%s kind=%s mime=%s", item.path, kind, mimeType)
			}
		})
	}
}

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

func TestProcessorDoesNotAddWorkspaceFileArtifactForFailedToolResult(t *testing.T) {
	workspacePath := t.TempDir()
	processor := NewProcessor(MessageContext{
		SessionKey:    "agent:nexus:ws:dm:test",
		AgentID:       "nexus",
		WorkspacePath: workspacePath,
		RoundID:       "round-file-artifact-error",
		ParentID:      "round-file-artifact-error",
	}, "")

	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID: "assistant-file-artifact-error-1",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.ToolUseBlock{
						ID:    "tool-edit-1",
						Name:  "Edit",
						Input: json.RawMessage(`{"file_path":"notes.md"}`),
					},
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
						ToolUseID: "tool-edit-1",
						Content:   json.RawMessage(`"failed"`),
						IsError:   true,
					},
				},
			},
		},
	})

	blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
	if len(blocks) != 2 {
		t.Fatalf("失败 tool_result 不应追加 artifact: %+v", blocks)
	}
}
