// 本文件验证记忆附件只把引用摘要挂到后续 Assistant。
package message

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestProcessorAttachesRelevantMemorySummariesToAssistant(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:test",
		AgentID:    "nexus",
		RoundID:    "round-memory-reference",
	}, "sdk-session-memory")

	attachmentOutput := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAttachment,
		Attachment: &sdkprotocol.AttachmentMessage{
			Type: "relevant_memories",
			Additional: map[string]any{"memories": []any{
				map[string]any{
					"content": "---\ndescription: 发布前检查签名清单\ntype: project\n---\n敏感正文不应复制到消息。",
					"path":    "/private/workspace/memory/release_checklist.md",
				},
			}},
		},
	})
	if len(attachmentOutput.DurableMessages) != 0 {
		t.Fatalf("relevant_memories attachment 生成了独立消息: %+v", attachmentOutput)
	}

	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{
			ID:         "assistant-memory-reference",
			StopReason: "end_turn",
			Content: []sdkprotocol.ContentBlock{sdkprotocol.TextBlock{
				Text: "发布检查已完成。",
			}},
		}},
	})
	if len(output.DurableMessages) != 1 {
		t.Fatalf("assistant durable messages = %+v", output.DurableMessages)
	}
	references, ok := output.DurableMessages[0]["recalled_memories"].([]map[string]any)
	if !ok || len(references) != 1 {
		t.Fatalf("recalled_memories = %#v", output.DurableMessages[0]["recalled_memories"])
	}
	if references[0]["name"] != "release checklist" || references[0]["description"] != "发布前检查签名清单" {
		t.Fatalf("memory reference = %#v", references[0])
	}
	if _, leaked := references[0]["content"]; leaked {
		t.Fatalf("memory reference 泄漏正文: %#v", references[0])
	}
	if _, leaked := references[0]["path"]; leaked {
		t.Fatalf("memory reference 泄漏绝对路径: %#v", references[0])
	}
}
