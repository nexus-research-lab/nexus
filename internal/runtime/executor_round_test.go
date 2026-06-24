package runtime

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type fakeRoundExecutionClient struct {
	sessionID    string
	queryErr     error
	contextErr   error
	streamErr    error
	waitErr      error
	messages     chan sdkprotocol.ReceivedMessage
	interrupts   int
	disconnects  int
	queryPrompts []string
	queryContent []any
	contextInput []ContextualInputBlock
}

func (c *fakeRoundExecutionClient) Connect(context.Context) error { return nil }

func (c *fakeRoundExecutionClient) Query(_ context.Context, prompt string) error {
	c.queryPrompts = append(c.queryPrompts, prompt)
	return c.queryErr
}

func (c *fakeRoundExecutionClient) QueryContent(_ context.Context, content any) error {
	c.queryContent = append(c.queryContent, content)
	return c.queryErr
}

func (c *fakeRoundExecutionClient) SetNextTurnContext(_ context.Context, blocks []ContextualInputBlock) error {
	c.contextInput = append([]ContextualInputBlock(nil), blocks...)
	return c.contextErr
}

func (c *fakeRoundExecutionClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeRoundExecutionClient) Interrupt(context.Context) error {
	c.interrupts++
	return nil
}

func (c *fakeRoundExecutionClient) StopTask(context.Context, string) error { return nil }

func (c *fakeRoundExecutionClient) SendTaskMessage(context.Context, string, string, string) error {
	return nil
}

func (c *fakeRoundExecutionClient) Disconnect(context.Context) error {
	c.disconnects++
	return nil
}

func (c *fakeRoundExecutionClient) Wait() error { return c.waitErr }

func (c *fakeRoundExecutionClient) StreamError() error { return c.streamErr }

func (c *fakeRoundExecutionClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *fakeRoundExecutionClient) SessionID() string { return c.sessionID }

type fakeRoundExecutionMapper struct {
	sessionID string
	results   []RoundMapResult
	err       error
	index     int
}

func (m *fakeRoundExecutionMapper) Map(
	sdkprotocol.ReceivedMessage,
	...string,
) (RoundMapResult, error) {
	if m.err != nil {
		return RoundMapResult{}, m.err
	}
	if m.index >= len(m.results) {
		return RoundMapResult{}, nil
	}
	result := m.results[m.index]
	m.index++
	return result, nil
}

func (m *fakeRoundExecutionMapper) SessionID() string {
	return m.sessionID
}

func TestExecuteRoundPersistsDurableMessagesAndEvents(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 2),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult}

	mapper := &fakeRoundExecutionMapper{
		results: []RoundMapResult{
			{
				DurableMessages: []protocol.Message{
					{"message_id": "assistant-1", "role": "assistant"},
				},
				Events: []protocol.EventMessage{
					protocol.NewEvent(protocol.EventTypeMessage, map[string]any{"message_id": "assistant-1"}),
				},
			},
			{
				DurableMessages: []protocol.Message{
					{"message_id": "result-1", "role": "result", "subtype": "success"},
				},
				Events: []protocol.EventMessage{
					protocol.NewEvent(protocol.EventTypeRoundStatus, map[string]any{"status": "finished"}),
				},
				TerminalStatus: "finished",
				ResultSubtype:  "success",
			},
		},
	}

	synced := make([]string, 0, 2)
	handled := make([]map[string]any, 0, 2)
	emitted := make([]protocol.EventMessage, 0, 2)
	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: mapper,
		SyncSessionID: func(sessionID string) error {
			synced = append(synced, sessionID)
			return nil
		},
		HandleDurableMessage: func(messageValue protocol.Message) error {
			copied := make(map[string]any, len(messageValue))
			for key, value := range messageValue {
				copied[key] = value
			}
			handled = append(handled, copied)
			return nil
		},
		EmitEvent: func(event protocol.EventMessage) error {
			emitted = append(emitted, event)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if result.TerminalStatus != "finished" || result.ResultSubtype != "success" {
		t.Fatalf("终态结果不正确: %+v", result)
	}
	if len(synced) != 2 {
		t.Fatalf("session_id 同步次数不正确: %+v", synced)
	}
	if synced[0] != "sdk-session-1" {
		t.Fatalf("同步的 session_id 不正确: %+v", synced)
	}
	if len(handled) != 2 {
		t.Fatalf("durable 消息处理次数不正确: %+v", handled)
	}
	for _, messageValue := range handled {
		if messageValue["session_id"] != "sdk-session-1" {
			t.Fatalf("durable 消息未补齐 session_id: %+v", messageValue)
		}
	}
	if len(emitted) != 2 {
		t.Fatalf("事件扇出次数不正确: %+v", emitted)
	}
}

func TestExecuteRoundReturnsTerminalErrorMessage(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-error",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	close(client.messages)

	mapper := &fakeRoundExecutionMapper{
		results: []RoundMapResult{{
			DurableMessages: []protocol.Message{
				{
					"message_id": "result-error",
					"role":       "result",
					"subtype":    "error",
					"is_error":   true,
					"result":     "Failed to authenticate. API Error: 401",
				},
			},
			TerminalStatus: "error",
			ResultSubtype:  "error",
		}},
	}

	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "continue",
		Client: client,
		Mapper: mapper,
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if result.TerminalStatus != "error" || result.ResultSubtype != "error" {
		t.Fatalf("result = %+v, want terminal error", result)
	}
	if result.ErrorMessage != "Failed to authenticate. API Error: 401" {
		t.Fatalf("ErrorMessage = %q", result.ErrorMessage)
	}
}

func TestExecuteRoundReturnsTerminalErrorMessageFromErrorsArray(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-error",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult}
	close(client.messages)

	mapper := &fakeRoundExecutionMapper{
		results: []RoundMapResult{{
			DurableMessages: []protocol.Message{
				{
					"message_id": "result-error",
					"role":       "result",
					"subtype":    "error",
					"is_error":   true,
					"errors":     []any{"client: stream closed before result message"},
				},
			},
			TerminalStatus: "error",
			ResultSubtype:  "error",
		}},
	}

	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "continue",
		Client: client,
		Mapper: mapper,
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if result.ErrorMessage != "client: stream closed before result message" {
		t.Fatalf("ErrorMessage = %q", result.ErrorMessage)
	}
}

func TestExecuteRoundUsesStructuredContent(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-structured",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-structured",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
		},
	}
	close(client.messages)
	mapper := &fakeRoundExecutionMapper{
		results: []RoundMapResult{{
			TerminalStatus: "finished",
			ResultSubtype:  "success",
		}},
	}
	content := []map[string]any{
		{"type": "text", "text": "描述图片"},
		{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": "image/png",
				"data":       "ZmFrZQ==",
			},
		},
	}

	if _, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content: content,
		Client:  client,
		Mapper:  mapper,
	}); err != nil {
		t.Fatalf("ExecuteRound 结构化输入失败: %v", err)
	}
	if len(client.queryPrompts) != 0 {
		t.Fatalf("结构化输入不应走纯文本 Query: %+v", client.queryPrompts)
	}
	if len(client.queryContent) != 1 {
		t.Fatalf("结构化输入未走 QueryContent: %+v", client.queryContent)
	}
}
