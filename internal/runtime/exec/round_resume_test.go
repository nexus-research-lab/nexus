package exec

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// runtime 自唤醒的 turn 已在进程内开始：round 不能再发 query，
// 且交接前取出的 init 必须先于流中后续消息处理。
func TestExecuteRoundResumesRuntimeTurnWithoutQuery(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-wake",
		messages:  make(chan sdkprotocol.ReceivedMessage, 2),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant, SessionID: client.sessionID}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		Result:    &sdkprotocol.ResultMessage{Subtype: "success"},
	}
	close(client.messages)
	mapper := &fakeRoundExecutionMapper{results: []RoundMapResult{
		{DurableMessages: []protocol.Message{{"message_id": "init"}}},
		{DurableMessages: []protocol.Message{{"message_id": "assistant"}}},
		{TerminalStatus: "finished", ResultSubtype: "success"},
	}}
	var persisted []string
	afterQueryCalls := 0

	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content:           "Background command completed",
		ResumeRuntimeTurn: true,
		InitialMessages:   []sdkprotocol.ReceivedMessage{{Type: sdkprotocol.MessageTypeSystem, Subtype: "init"}},
		Client:            client,
		Mapper:            mapper,
		// 接管的 turn 同样要激活绑定、补发排队输入。
		AfterQuery: func() error { afterQueryCalls++; return nil },
		HandleDurableMessage: func(message protocol.Message) error {
			persisted = append(persisted, message["message_id"].(string))
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound resume error = %v", err)
	}
	if result.TerminalStatus != "finished" {
		t.Fatalf("terminal = %+v", result)
	}
	if len(client.queryPrompts) != 0 || len(client.queryContent) != 0 || client.clearCalls != 0 {
		t.Fatalf("resume 不应发送 query: prompts=%v content=%v clear=%d", client.queryPrompts, client.queryContent, client.clearCalls)
	}
	if afterQueryCalls != 1 {
		t.Fatalf("AfterQuery calls = %d, want 1", afterQueryCalls)
	}
	if len(persisted) != 2 || persisted[0] != "init" || persisted[1] != "assistant" {
		t.Fatalf("persisted = %v, 初始消息必须先处理", persisted)
	}
}
