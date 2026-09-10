package message

import (
	"encoding/json"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestFileDeliveryPersistsExactAgentRound(t *testing.T) {
	for _, agent := range []string{"researcher", "designer"} {
		t.Run(agent, func(t *testing.T) {
			p := NewProcessor(MessageContext{AgentID: agent, AgentRoundID: agent + "-round", RoomID: "room", ConversationID: "conversation", RoundID: "root", WorkspacePath: t.TempDir()}, "")
			p.Process(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant, Assistant: &sdkprotocol.AssistantMessage{Message: sdkprotocol.ConversationEnvelope{ID: "reply-" + agent, Content: []sdkprotocol.ContentBlock{
				sdkprotocol.ToolUseBlock{ID: "delivery", Name: "mcp__nexus__deliver_files", Input: json.RawMessage(`{"paths":["report.pptx","data.xlsx"]}`)},
			}}}})
			text, _ := json.Marshal(map[string]any{"kind": "file_delivery", "version": 1, "agent_id": agent, "agent_round_id": agent + "-round", "paths": []string{"report.pptx", "data.xlsx"}})
			content, _ := json.Marshal(string(text))
			out := p.Process(sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeUser, User: &sdkprotocol.UserMessage{Message: sdkprotocol.ConversationEnvelope{Content: []sdkprotocol.ContentBlock{
				sdkprotocol.ToolResultBlock{ToolUseID: "delivery", Content: content},
			}}}})
			if len(out.DurableMessages) != 1 {
				t.Fatalf("missing durable delivery: %+v", out)
			}
			blocks := out.DurableMessages[0]["content"].([]map[string]any)
			if len(blocks) != 4 {
				t.Fatalf("missing output files: %+v", blocks)
			}
			for _, block := range blocks[2:] {
				if block["workspace_agent_id"] != agent || block["producer_agent_id"] != agent || block["source_agent_round_id"] != agent+"-round" || block["role"] != "deliverable" {
					t.Fatalf("wrong provenance: %+v", block)
				}
			}
		})
	}
}

func TestFileDeliveryRejectsUntrustedOrInvalidEvidence(t *testing.T) {
	for _, test := range []struct {
		name, tool, agent, round, path string
		failed                         bool
	}{
		{"shell text", "Bash", "a", "r", "out.pdf", false},
		{"other MCP", "mcp__other__deliver_files", "a", "r", "out.pdf", false},
		{"other agent", "nexus.deliver_files", "b", "r", "out.pdf", false},
		{"old round", "nexus.deliver_files", "a", "old", "out.pdf", false},
		{"outside", "nexus.deliver_files", "a", "r", "../out.pdf", false},
		{"empty", "nexus.deliver_files", "a", "r", "", false},
		{"failed", "nexus.deliver_files", "a", "r", "out.pdf", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := NewProcessor(MessageContext{AgentID: "a", AgentRoundID: "r", WorkspacePath: t.TempDir()}, "")
			p.segment.AppendToolResults([]map[string]any{{"type": "tool_use", "id": "tool", "name": test.tool}})
			data, _ := json.Marshal(map[string]any{"kind": "file_delivery", "version": 1, "agent_id": test.agent, "agent_round_id": test.round, "paths": []string{"valid.pdf", test.path}})
			if got := p.workspaceFileArtifactsForToolResult(map[string]any{"tool_use_id": "tool", "content": string(data), "is_error": test.failed}); len(got) != 0 {
				t.Fatalf("accepted invalid evidence: %+v", got)
			}
		})
	}
}
