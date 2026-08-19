package message

import (
	"encoding/json"
	"strconv"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProcessorAddsNexusResourceArtifactForCreateResults(t *testing.T) {
	cases := []struct {
		name           string
		output         string
		resourceKind   string
		resourceID     string
		conversationID string
	}{
		{
			name:         "agent",
			output:       `{"success":true,"domain":"agent","action":"create","item":{"agent_id":"agent-1","name":"需求助手","description":"澄清需求","vibe_tags":["用户视角","结构化"]}}`,
			resourceKind: protocol.NexusResourceKindAgent,
			resourceID:   "agent-1",
		},
		{
			name:           "room",
			output:         `{"success":true,"domain":"room","action":"create","initial_message":"@用户研究顾问 请开始评审","initial_target_agent_ids":["agent-2"],"item":{"room":{"id":"room-1","name":"产品评审室","description":"评审真实需求","avatar":"24"},"conversation":{"id":"conversation-1"},"member_agents":[{"agent_id":"agent-1","name":"产品搭档"},{"agent_id":"agent-2","name":"用户研究顾问"}]}}`,
			resourceKind:   protocol.NexusResourceKindRoom,
			resourceID:     "room-1",
			conversationID: "conversation-1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			processor := NewProcessor(MessageContext{
				SessionKey: "agent:nexus:ws:dm:test",
				AgentID:    "nexus",
				RoundID:    "round-resource-artifact",
				ParentID:   "round-resource-artifact",
			}, "")
			processor.Process(sdkprotocol.ReceivedMessage{
				Type: sdkprotocol.MessageTypeAssistant,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID: "assistant-resource-artifact",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.ToolUseBlock{
								ID:    "tool-nexus-create",
								Name:  "Bash",
								Input: json.RawMessage(`{"command":"nexusctl --json ` + tc.resourceKind + ` create"}`),
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
								ToolUseID: "tool-nexus-create",
								Content:   json.RawMessage(strconv.Quote(tc.output)),
							},
						},
					},
				},
			})

			if len(output.DurableMessages) != 1 {
				t.Fatalf("resource artifact durable message missing: %+v", output)
			}
			blocks, _ := output.DurableMessages[0]["content"].([]map[string]any)
			if len(blocks) != 3 {
				t.Fatalf("resource artifact block missing: %+v", blocks)
			}
			artifact := blocks[2]
			if artifact["type"] != protocol.ContentBlockTypeNexusResourceArtifact ||
				artifact["resource_kind"] != tc.resourceKind ||
				artifact["resource_id"] != tc.resourceID {
				t.Fatalf("resource artifact mismatch: %+v", artifact)
			}
			if artifact["conversation_id"] != tc.conversationID && tc.conversationID != "" {
				t.Fatalf("room conversation missing: %+v", artifact)
			}
			if tc.resourceKind == protocol.NexusResourceKindRoom {
				members, ok := artifact["members"].([]protocol.NexusResourceArtifactMember)
				if !ok || len(members) != 2 || members[0].Name != "产品搭档" {
					t.Fatalf("room card members missing: %+v", artifact)
				}
				if artifact["initial_message"] != "@用户研究顾问 请开始评审" {
					t.Fatalf("room initial message missing: %+v", artifact)
				}
				targets, ok := artifact["initial_target_agent_ids"].([]string)
				if !ok || len(targets) != 1 || targets[0] != "agent-2" {
					t.Fatalf("room initial targets missing: %+v", artifact)
				}
			}
		})
	}
}

func TestNexusResourcePayloadUsesStructuredShellOutput(t *testing.T) {
	payload := nexusResourcePayloadForToolResult(map[string]any{
		"content": "",
		"structured_output": map[string]any{
			"stdout": `{"success":true,"domain":"room","action":"create","item":{"room":{"id":"room-structured"},"conversation":{"id":"conversation-structured"}}}`,
		},
	})
	if normalizeString(payload["domain"]) != protocol.NexusResourceKindRoom ||
		normalizeString(payload["action"]) != "create" {
		t.Fatalf("structured shell output should create room card payload: %+v", payload)
	}
	if !isNexusResourceArtifactTool("powershell") {
		t.Fatal("shell tool matching should be case-insensitive")
	}
}

func TestProcessorIgnoresNonCreateNexusResourceResult(t *testing.T) {
	payload := map[string]any{
		"success": true,
		"domain":  "agent",
		"action":  "list",
		"items":   []any{},
	}
	encoded, _ := json.Marshal(payload)
	if result := firstNexusResourcePayload(string(encoded)); normalizeString(result["action"]) != "list" {
		t.Fatalf("test setup failed: %+v", result)
	}
}
