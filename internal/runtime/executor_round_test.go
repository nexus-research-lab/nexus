package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestExecuteRoundUsesInternalContextWhenSupported(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-context",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-context",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
		},
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content: "用户输入",
		ContextualInputs: []ContextualInputBlock{
			NewContextualInputBlock("goal", "Continue.", 0, map[string]string{"goal_id": "goal-1"}),
		},
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{TerminalStatus: "finished", ResultSubtype: "success"}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if len(client.contextInput) != 1 || client.contextInput[0].Name != "goal" || client.contextInput[0].Content != "Continue." {
		t.Fatalf("contextInput = %#v, want goal internal context", client.contextInput)
	}
	if len(client.queryPrompts) != 1 || client.queryPrompts[0] != "用户输入" {
		t.Fatalf("queryPrompts = %#v, want unmodified user input", client.queryPrompts)
	}
}

func TestExecuteRoundUsesTriggerForContextOnlyInternalTurn(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-context-only",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-context-only",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
		},
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content: "",
		ContextualInputs: []ContextualInputBlock{
			NewContextualInputBlock("goal", "Continue.", 0, map[string]string{"goal_id": "goal-1"}),
		},
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{TerminalStatus: "finished", ResultSubtype: "success"}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if len(client.contextInput) != 1 || client.contextInput[0].Name != "goal" || client.contextInput[0].Content != "Continue." {
		t.Fatalf("contextInput = %#v, want goal internal context", client.contextInput)
	}
	if len(client.queryPrompts) != 1 || client.queryPrompts[0] != contextOnlyTurnTrigger {
		t.Fatalf("queryPrompts = %#v, want context-only trigger", client.queryPrompts)
	}
}

func TestExecuteRoundFallsBackToUserContextPrefixWhenInternalContextUnsupported(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID:  "sdk-session-context-fallback",
		contextErr: agentclient.ErrUnsupportedCapability,
		messages:   make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-context-fallback",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
		},
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content: "用户输入",
		ContextualInputs: []ContextualInputBlock{
			NewContextualInputBlock("goal", "Continue.", 0, nil),
		},
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{TerminalStatus: "finished", ResultSubtype: "success"}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if len(client.queryPrompts) != 1 ||
		!strings.HasPrefix(client.queryPrompts[0], "<internal_context source=\"goal\">\nContinue.\n</internal_context>\n\n") ||
		!strings.Contains(client.queryPrompts[0], "用户输入") {
		t.Fatalf("queryPrompts = %#v, want context-prefixed user input", client.queryPrompts)
	}
}

func TestExecuteRoundFallsBackToStructuredContentPrefixWhenInternalContextUnsupported(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID:  "sdk-session-context-structured",
		contextErr: agentclient.ErrUnsupportedCapability,
		messages:   make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-context-structured",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
		},
	}
	close(client.messages)
	content := []map[string]any{{"type": "text", "text": "描述图片"}}

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content: content,
		ContextualInputs: []ContextualInputBlock{
			NewContextualInputBlock("goal", "Continue.", 0, nil),
		},
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{TerminalStatus: "finished", ResultSubtype: "success"}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if len(client.queryContent) != 1 {
		t.Fatalf("queryContent = %#v, want one structured payload", client.queryContent)
	}
	blocks, ok := client.queryContent[0].([]map[string]any)
	if !ok || len(blocks) != 2 || blocks[0]["text"] != "<internal_context source=\"goal\">\nContinue.\n</internal_context>" {
		t.Fatalf("queryContent[0] = %#v, want prepended context text block", client.queryContent[0])
	}
}

func TestExecuteRoundCompletesFromTerminalAssistantWhenResultMissing(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	close(client.messages)

	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{
				DurableMessages: []protocol.Message{{
					"message_id":   "assistant-1",
					"role":         "assistant",
					"is_complete":  true,
					"stop_reason":  "end_turn",
					"session_id":   "sdk-session-1",
					"content":      []map[string]any{{"type": "text", "text": "完成"}},
					"usage":        map[string]any{"input_tokens": 3, "output_tokens": 2},
					"round_id":     "round-1",
					"session_key":  "agent:test",
					"conversation": "unused",
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("terminal assistant 不应被判为 stream closed: %v", err)
	}
	if result.TerminalStatus != "finished" || result.ResultSubtype != "success" || !result.CompletedByAssistant {
		t.Fatalf("terminal assistant 终态不正确: %+v", result)
	}
}

func TestExecuteRoundKeepsAssistantCompletionWhenResultArrives(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 2),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult}

	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{
				{
					DurableMessages: []protocol.Message{{
						"message_id":  "assistant-1",
						"role":        "assistant",
						"is_complete": true,
						"stop_reason": "end_turn",
					}},
				},
				{
					DurableMessages: []protocol.Message{{
						"message_id": "result-1",
						"role":       "result",
						"subtype":    "success",
					}},
					TerminalStatus: "finished",
					ResultSubtype:  "success",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("result 到达后不应失败: %v", err)
	}
	if result.TerminalStatus != "finished" || result.ResultSubtype != "success" || !result.CompletedByAssistant {
		t.Fatalf("result 到达后应保留 assistant 完成状态: %+v", result)
	}
}

func TestExecuteRoundTreatsSuccessfulResultAsAssistantCompletion(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-result-only",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult}

	result, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{
				DurableMessages: []protocol.Message{{
					"message_id": "result-1",
					"role":       "result",
					"subtype":    "success",
					"result":     "完成",
				}},
				TerminalStatus: "finished",
				ResultSubtype:  "success",
			}},
		},
	})
	if err != nil {
		t.Fatalf("result-only 成功终态不应失败: %v", err)
	}
	if result.TerminalStatus != "finished" || result.ResultSubtype != "success" || !result.CompletedByAssistant {
		t.Fatalf("result-only 成功终态应触发 assistant 完成: %+v", result)
	}
}

func TestExecuteRoundKeepsWaitingForToolUseAssistant(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		waitErr:   errors.New("exit status 1"),
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "需要工具",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{
				DurableMessages: []protocol.Message{{
					"message_id":  "assistant-tool-1",
					"role":        "assistant",
					"is_complete": true,
					"stop_reason": "tool_use",
				}},
			}},
		},
	})
	if !errors.Is(err, ErrRoundStreamClosedBeforeTerminal) {
		t.Fatalf("tool_use assistant 不能提前当成终态: %v", err)
	}
}

func TestExecuteRoundReturnsInterruptedWhenContextCancelled(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage),
	}
	mapper := &fakeRoundExecutionMapper{}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := ExecuteRound(ctx, RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: mapper,
	})
	if !errors.Is(err, ErrRoundInterrupted) {
		t.Fatalf("期望返回 ErrRoundInterrupted，实际 %v", err)
	}
}

func TestExecuteRoundReturnsStreamClosedDiagnostics(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		waitErr:   errors.New("exit status 1"),
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: "sdk-session-1",
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{ID: "assistant-1"},
		},
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{}},
		},
	})
	if !errors.Is(err, ErrRoundStreamClosedBeforeTerminal) {
		t.Fatalf("期望 ErrRoundStreamClosedBeforeTerminal，实际 %v", err)
	}
	var streamErr *RoundStreamClosedError
	if !errors.As(err, &streamErr) {
		t.Fatalf("期望 RoundStreamClosedError，实际 %T %[1]v", err)
	}
	if streamErr.MessagesSeen != 1 ||
		streamErr.LastMessageType != string(sdkprotocol.MessageTypeAssistant) ||
		streamErr.LastSessionID != "sdk-session-1" ||
		streamErr.LastMessageID != "assistant-1" {
		t.Fatalf("stream close 诊断字段不正确: %+v", streamErr)
	}
	if !strings.Contains(streamErr.WaitError, "exit status 1") {
		t.Fatalf("stream close 缺少 wait error: %+v", streamErr)
	}
}

func TestExecuteRoundReturnsStreamReadErrorDiagnostics(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		streamErr: errors.New(
			"client: read message failed: process: decode stdout JSON message failed: unexpected EOF",
		),
		messages: make(chan sdkprotocol.ReceivedMessage),
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{},
	})
	if !errors.Is(err, ErrRoundStreamClosedBeforeTerminal) {
		t.Fatalf("期望 ErrRoundStreamClosedBeforeTerminal，实际 %v", err)
	}
	var streamErr *RoundStreamClosedError
	if !errors.As(err, &streamErr) {
		t.Fatalf("期望 RoundStreamClosedError，实际 %T %[1]v", err)
	}
	if !strings.Contains(streamErr.ReadError, "decode stdout JSON message failed") {
		t.Fatalf("stream close 缺少 read error: %+v", streamErr)
	}
	if !strings.Contains(err.Error(), "read_error=") {
		t.Fatalf("错误字符串缺少 read_error: %v", err)
	}
}

func TestExecuteRoundReturnsIdleTimeoutDiagnostics(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "content_block_delta",
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": "让我用 AskUserQuestion 来收集信息。",
				},
			},
		},
	}

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:       "创建定时任务",
		Client:      client,
		Mapper:      &fakeRoundExecutionMapper{results: []RoundMapResult{{}}},
		IdleTimeout: 10 * time.Millisecond,
	})
	if !errors.Is(err, ErrRoundStreamIdleTimeout) {
		t.Fatalf("期望 ErrRoundStreamIdleTimeout，实际 %v", err)
	}
	var timeoutErr *RoundStreamIdleTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("期望 RoundStreamIdleTimeoutError，实际 %T %[1]v", err)
	}
	if timeoutErr.MessagesSeen != 1 ||
		timeoutErr.LastMessageType != string(sdkprotocol.MessageTypeStreamEvent) ||
		timeoutErr.LastSessionID != "sdk-session-1" ||
		!strings.Contains(timeoutErr.LastMessageSummary, "thinking_delta") ||
		strings.Contains(timeoutErr.LastMessageSummary, "AskUserQuestion") {
		t.Fatalf("idle timeout 诊断字段不正确: %+v", timeoutErr)
	}
	if client.interrupts != 1 || client.disconnects != 1 {
		t.Fatalf("idle timeout 未中止 runtime client: interrupts=%d disconnects=%d", client.interrupts, client.disconnects)
	}
}
