package message

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestAssistantMissedGoalCompletionCommand(t *testing.T) {
	tests := []struct {
		name               string
		text               string
		completionRecorded bool
		want               bool
	}{
		{name: "missing command after completion", text: "任务已经完成，但无法通过 nexus.command 调用 update_goal 来标记完成。", want: true},
		{name: "no completion claim", text: "I cannot run update_goal yet because more verification is needed."},
		{name: "final claim", text: "PPT 已完成并验证通过：9 页内容、298 行。", want: true},
		{name: "stage complete", text: "第一阶段已完成，下一步会继续进行 Goal 恢复链路检查。"},
		{name: "all stages verified", text: "所有阶段已完成并验证通过。", want: true},
		{name: "receipt recorded", text: "Goal has been completed.", completionRecorded: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := protocol.Message{"role": "assistant", "content": []map[string]any{{"type": "text", "text": test.text}}}
			if got := AssistantMissedGoalCompletionCommand(message, test.completionRecorded); got != test.want {
				t.Fatalf("AssistantMissedGoalCompletionCommand() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestProcessorRedactsPostActionObservationAndImageBlocks(t *testing.T) {
	processor := NewProcessor(MessageContext{
		SessionKey: "agent:nexus:ws:dm:cua-action-redaction",
		AgentID:    "nexus", RoundID: "round-cua-action-redaction",
	}, "")
	processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeAssistant,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{Content: []sdkprotocol.ContentBlock{
				sdkprotocol.ToolUseBlock{ID: "tool-action", Name: "mcp__nexus__command",
					Input: json.RawMessage(`{"domain":"computer","operation":"perform_action"}`)},
			}},
		},
	})
	content := json.RawMessage(`[{"type":"text","text":"{\"outcome\":\"applied\",\"dispatched\":true,\"observation\":{\"observation_id\":\"obs-next\",\"elements\":[{\"name\":\"PRIVATE AX TEXT\"}]}}"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"PRIVATE SCREENSHOT"}}]`)
	original := string(content)
	output := processor.Process(sdkprotocol.ReceivedMessage{
		Type: sdkprotocol.MessageTypeUser,
		User: &sdkprotocol.UserMessage{
			Message: sdkprotocol.ConversationEnvelope{Content: []sdkprotocol.ContentBlock{
				sdkprotocol.ToolResultBlock{ToolUseID: "tool-action", Content: content},
			}},
		},
	})
	encoded, err := json.Marshal(output.DurableMessages)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE AX TEXT", "PRIVATE SCREENSHOT"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("post-action transient content entered durable history")
		}
	}
	for _, evidence := range []string{"applied", "dispatched", "obs-next", "elements_count", "cua_transient_content_redacted"} {
		if !strings.Contains(string(encoded), evidence) {
			t.Fatalf("durable history lost %s", evidence)
		}
	}
	if string(content) != original {
		t.Fatal("durable redaction changed the original provider content")
	}
}
