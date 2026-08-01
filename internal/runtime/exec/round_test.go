package exec

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
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
	clearCalls   int
	receiveStart chan struct{}
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

func (c *fakeRoundExecutionClient) ClearNextTurnContext(context.Context) error {
	c.clearCalls++
	return c.contextErr
}

func (c *fakeRoundExecutionClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	if c.receiveStart != nil {
		select {
		case c.receiveStart <- struct{}{}:
		default:
		}
	}
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

func (c *fakeRoundExecutionClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *fakeRoundExecutionClient) SetPermissionMode(context.Context, sdkpermission.Mode) error {
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

func TestExecuteRoundConsumesDelayedTerminalAfterExplicitInterrupt(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID:    "sdk-session-interrupted",
		messages:     make(chan sdkprotocol.ReceivedMessage, 1),
		receiveStart: make(chan struct{}, 1),
	}
	mapper := &fakeRoundExecutionMapper{}
	ctx, cancel := context.WithCancel(context.Background())
	type executionOutcome struct {
		result RoundExecutionResult
		err    error
	}
	done := make(chan executionOutcome, 1)
	go func() {
		result, err := ExecuteRound(ctx, RoundExecutionRequest{
			Query:  "long-running request",
			Client: client,
			Mapper: mapper,
			InterruptReason: func() string {
				return "user stopped"
			},
		})
		done <- executionOutcome{result: result, err: err}
	}()

	select {
	case <-client.receiveStart:
	case <-time.After(time.Second):
		t.Fatal("round 未开始接收 runtime 消息")
	}
	cancel()

	select {
	case outcome := <-done:
		t.Fatalf("显式中断不应在旧回合 result 到达前释放消息流: result=%+v err=%v", outcome.result, outcome.err)
	case <-time.After(30 * time.Millisecond):
	}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	select {
	case outcome := <-done:
		t.Fatalf("assistant 终态不能替代旧回合的 wire result: result=%+v err=%v", outcome.result, outcome.err)
	case <-time.After(30 * time.Millisecond):
	}

	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		Result: &sdkprotocol.ResultMessage{
			Subtype: "interrupted",
			Usage: map[string]any{
				"input_tokens":  12,
				"output_tokens": 3,
				"total_tokens":  15,
			},
		},
	}

	select {
	case outcome := <-done:
		if !errors.Is(outcome.err, ErrRoundInterrupted) {
			t.Fatalf("消费迟到 terminal 后错误不正确: %v", outcome.err)
		}
		if outcome.result.TerminalStatus != "" || outcome.result.CompletedByAssistant {
			t.Fatalf("排空阶段不应把 runtime terminal 重新投影为正常终态: %+v", outcome.result)
		}
		if outcome.result.Usage.TotalTokens != 15 {
			t.Fatalf("排空阶段应保留 provider usage: %+v", outcome.result.Usage)
		}
		if mapper.index != 0 {
			t.Fatalf("排空阶段不应映射或公开迟到消息，mapper calls=%d", mapper.index)
		}
	case <-time.After(time.Second):
		t.Fatal("迟到 terminal 到达后 round 未结束")
	}
}

func TestRoundReceiveIgnoresReadyAssistantFallbackAfterExplicitInterrupt(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-interrupted-assistant",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	execution, err := newRoundExecution(context.Background(), RoundExecutionRequest{
		Client: client,
		Mapper: &fakeRoundExecutionMapper{},
		InterruptReason: func() string {
			return "user stopped"
		},
	})
	if err != nil {
		t.Fatalf("创建 round execution 失败: %v", err)
	}
	execution.assistantTerminalResult = &RoundExecutionResult{
		TerminalStatus:       "finished",
		ResultSubtype:        "success",
		CompletedByAssistant: true,
	}
	ready := make(chan time.Time)
	close(ready)
	execution.assistantTerminalTimer = ready

	type executionOutcome struct {
		result RoundExecutionResult
		err    error
	}
	done := make(chan executionOutcome, 1)
	go func() {
		result, receiveErr := execution.receive()
		done <- executionOutcome{result: result, err: receiveErr}
	}()
	select {
	case outcome := <-done:
		t.Fatalf("显式中断不应走 assistant fallback: result=%+v err=%v", outcome.result, outcome.err)
	case <-time.After(30 * time.Millisecond):
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{Subtype: "interrupted"},
	}
	select {
	case outcome := <-done:
		if !errors.Is(outcome.err, ErrRoundInterrupted) {
			t.Fatalf("assistant fallback 排空错误不正确: %v", outcome.err)
		}
		if outcome.result.CompletedByAssistant || outcome.result.TerminalStatus != "" {
			t.Fatalf("assistant fallback 泄漏为正常终态: %+v", outcome.result)
		}
	case <-time.After(time.Second):
		t.Fatal("assistant fallback 排空 result 后未结束")
	}
}

func TestRoundReceiveWaitsForTerminalAfterIdleTimeoutWithExplicitInterrupt(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-interrupted-idle",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	execution, err := newRoundExecution(context.Background(), RoundExecutionRequest{
		Client:      client,
		Mapper:      &fakeRoundExecutionMapper{},
		IdleTimeout: 10 * time.Millisecond,
		InterruptReason: func() string {
			return "user stopped"
		},
	})
	if err != nil {
		t.Fatalf("创建 round execution 失败: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, receiveErr := execution.receive()
		done <- receiveErr
	}()
	select {
	case receiveErr := <-done:
		t.Fatalf("显式中断不应走 idle timeout: %v", receiveErr)
	case <-time.After(50 * time.Millisecond):
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:   sdkprotocol.MessageTypeResult,
		Result: &sdkprotocol.ResultMessage{Subtype: "interrupted"},
	}
	select {
	case receiveErr := <-done:
		if !errors.Is(receiveErr, ErrRoundInterrupted) {
			t.Fatalf("idle timeout 排空错误不正确: %v", receiveErr)
		}
	case <-time.After(time.Second):
		t.Fatal("idle timeout 排空 result 后未结束")
	}
}

func TestExecuteRoundDisconnectsWhenInterruptedTerminalNeverArrives(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID:    "sdk-session-unclean",
		messages:     make(chan sdkprotocol.ReceivedMessage),
		receiveStart: make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := ExecuteRound(ctx, RoundExecutionRequest{
			Query:                      "long-running request",
			Client:                     client,
			Mapper:                     &fakeRoundExecutionMapper{},
			InterruptedTerminalTimeout: 20 * time.Millisecond,
			InterruptReason: func() string {
				return "user stopped"
			},
		})
		done <- err
	}()

	select {
	case <-client.receiveStart:
	case <-time.After(time.Second):
		t.Fatal("round 未开始接收 runtime 消息")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, ErrRoundInterrupted) {
			t.Fatalf("terminal 超时错误不正确: %v", err)
		}
		if client.disconnects != 1 {
			t.Fatalf("未收口 client 必须断开，disconnects=%d", client.disconnects)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal 超时后 round 未结束")
	}
}

func TestExecuteRoundDisconnectsWhenInterruptedStreamCloses(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID:    "sdk-session-closed-unclean",
		messages:     make(chan sdkprotocol.ReceivedMessage),
		receiveStart: make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := ExecuteRound(ctx, RoundExecutionRequest{
			Query:  "long-running request",
			Client: client,
			Mapper: &fakeRoundExecutionMapper{},
			InterruptReason: func() string {
				return "user stopped"
			},
		})
		done <- err
	}()

	select {
	case <-client.receiveStart:
	case <-time.After(time.Second):
		t.Fatal("round 未开始接收 runtime 消息")
	}
	cancel()
	close(client.messages)

	select {
	case err := <-done:
		if !errors.Is(err, ErrRoundInterrupted) {
			t.Fatalf("stream 关闭后的中断错误不正确: %v", err)
		}
		if client.disconnects != 1 {
			t.Fatalf("无 terminal 即关闭的 client 必须断开，disconnects=%d", client.disconnects)
		}
	case <-time.After(time.Second):
		t.Fatal("stream 关闭后 round 未结束")
	}
}

func TestExecuteRoundPreservesTerminalUsageWhenLocalProcessingFails(t *testing.T) {
	localErr := errors.New("local terminal processing failed")
	for _, stage := range []string{"map", "sync", "persist", "emit"} {
		t.Run(stage, func(t *testing.T) {
			client := &fakeRoundExecutionClient{
				sessionID: "sdk-session-terminal-failure",
				messages:  make(chan sdkprotocol.ReceivedMessage, 1),
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-terminal-failure",
				Result: &sdkprotocol.ResultMessage{
					Subtype: "success",
					Usage: map[string]any{
						"input_tokens":  int64(10),
						"output_tokens": int64(5),
						"total_tokens":  int64(15),
					},
				},
			}

			mapper := &fakeRoundExecutionMapper{
				results: []RoundMapResult{{
					DurableMessages: []protocol.Message{{
						"message_id": "result-terminal-failure",
						"role":       "result",
						"subtype":    "success",
					}},
					Events: []protocol.EventMessage{
						protocol.NewEvent(protocol.EventTypeRoundStatus, map[string]any{"status": "finished"}),
					},
					TerminalStatus: "finished",
					ResultSubtype:  "success",
				}},
			}
			if stage == "map" {
				mapper.err = localErr
			}

			request := RoundExecutionRequest{
				Query:  "continue",
				Client: client,
				Mapper: mapper,
				SyncSessionID: func(string) error {
					if stage == "sync" {
						return localErr
					}
					return nil
				},
				HandleDurableMessage: func(protocol.Message) error {
					if stage == "persist" {
						return localErr
					}
					return nil
				},
				EmitEvent: func(protocol.EventMessage) error {
					if stage == "emit" {
						return localErr
					}
					return nil
				},
			}

			result, err := ExecuteRound(context.Background(), request)
			if !errors.Is(err, localErr) {
				t.Fatalf("ExecuteRound() error = %v, want %v", err, localErr)
			}
			if result.Usage.InputTokens != 10 ||
				result.Usage.OutputTokens != 5 ||
				result.Usage.TotalTokens != 15 {
				t.Fatalf("result usage = %#v, want preserved provider total 15", result.Usage)
			}
			if result.Usage.Raw == nil {
				t.Fatalf("result usage raw = nil, explicit provider total presence was lost")
			}
			if result.TerminalStatus != "" || result.CompletedByAssistant {
				t.Fatalf("local failure result leaked successful terminal state: %+v", result)
			}
		})
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

func TestExecuteRoundKeepsAtomicSlashInputFreeOfContext(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-command",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		Result:    &sdkprotocol.ResultMessage{Subtype: "success"},
	}
	close(client.messages)
	mapper := &fakeRoundExecutionMapper{
		results: []RoundMapResult{{
			TerminalStatus: "finished",
			ResultSubtype:  "success",
		}},
	}

	if _, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content:     "/model sonnet",
		AtomicInput: true,
		ContextualInputs: []ContextualInputBlock{{
			Name:    "goal",
			Content: "must not reach the command",
		}},
		Client: client,
		Mapper: mapper,
	}); err != nil {
		t.Fatalf("ExecuteRound atomic command error = %v", err)
	}
	if client.clearCalls != 1 ||
		len(client.contextInput) != 0 ||
		len(client.queryPrompts) != 1 ||
		client.queryPrompts[0] != "/model sonnet" {
		t.Fatalf(
			"atomic command calls = clear:%d context:%#v prompts:%#v",
			client.clearCalls,
			client.contextInput,
			client.queryPrompts,
		)
	}
}
