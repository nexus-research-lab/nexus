package exec

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

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

type discardableRoundExecutionClient struct {
	*fakeRoundExecutionClient
	discards int
}

func (c *discardableRoundExecutionClient) DiscardUncleanSession() {
	c.discards++
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

func (c *fakeRoundExecutionClient) Retire() {}

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

type fakeRoundIdlePauseState struct {
	mu      sync.Mutex
	paused  bool
	changed chan struct{}
}

func newFakeRoundIdlePauseState() *fakeRoundIdlePauseState {
	return &fakeRoundIdlePauseState{changed: make(chan struct{})}
}

func (s *fakeRoundIdlePauseState) Snapshot() (bool, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused, s.changed
}

func (s *fakeRoundIdlePauseState) SetPaused(paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused == paused {
		return
	}
	s.paused = paused
	close(s.changed)
	s.changed = make(chan struct{})
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

func TestDisconnectUncleanRoundClientUsesNonblockingDiscard(t *testing.T) {
	client := &discardableRoundExecutionClient{fakeRoundExecutionClient: &fakeRoundExecutionClient{}}

	disconnectUncleanRoundClient(client)

	if client.discards != 1 {
		t.Fatalf("未收口 SDK session 应原子隔离一次，discards=%d", client.discards)
	}
	if client.disconnects != 0 {
		t.Fatalf("支持异步隔离时不应阻塞 round 等待 Disconnect，disconnects=%d", client.disconnects)
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

func TestExecuteRoundDoesNotInventUserTextForContextOnlyTurn(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-context-only",
		messages:  make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-context-only",
		Result:    &sdkprotocol.ResultMessage{Subtype: "success"},
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content: "",
		ContextualInputs: []ContextualInputBlock{
			runtimectx.NewContextualInputBlock("goal", "hidden goal", 0, nil),
		},
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{TerminalStatus: "finished", ResultSubtype: "success"}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if len(client.contextInput) != 1 || client.contextInput[0].Content != "hidden goal" {
		t.Fatalf("contextInput = %#v, want buffered hidden context", client.contextInput)
	}
	if len(client.queryPrompts) != 1 || client.queryPrompts[0] != "" {
		t.Fatalf("queryPrompts = %#v, want untouched empty user text", client.queryPrompts)
	}
}

func TestExecuteRoundReturnsInterruptedWhenStreamAbortedAfterToolUse(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		streamErr: agentclient.ErrAborted,
		messages:  make(chan sdkprotocol.ReceivedMessage, 3),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": "tool_use",
				},
			},
		},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{"type": "message_stop"},
		},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeToolProgress,
		SessionID: "sdk-session-1",
		ToolProgress: &sdkprotocol.ToolProgressMessage{
			ToolName: "Agent",
		},
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "需要 Agent",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{}, {}, {}},
		},
	})
	if !errors.Is(err, ErrRoundInterrupted) {
		t.Fatalf("期望返回 ErrRoundInterrupted，实际 %v", err)
	}
	if errors.Is(err, ErrRoundStreamClosedBeforeTerminal) {
		t.Fatalf("abort 不应归类为 stream closed: %v", err)
	}
}

func TestExecuteRoundReturnsStreamClosedDiagnostics(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		streamErr: errors.New(
			"client: read message failed: process: decode stdout JSON message failed: unexpected EOF",
		),
		waitErr:  errors.New("exit status 1"),
		messages: make(chan sdkprotocol.ReceivedMessage, 1),
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
	if !strings.Contains(streamErr.ReadError, "decode stdout JSON message failed") {
		t.Fatalf("stream close 缺少 read error: %+v", streamErr)
	}
	if !strings.Contains(err.Error(), "read_error=") {
		t.Fatalf("错误字符串缺少 read_error: %v", err)
	}
}

func TestExecuteRoundPausesIdleTimeoutUntilInteractionResolves(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID:    "sdk-session-paused-idle",
		messages:     make(chan sdkprotocol.ReceivedMessage),
		receiveStart: make(chan struct{}, 1),
	}
	idlePause := newFakeRoundIdlePauseState()
	done := make(chan error, 1)
	go func() {
		_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
			Query:          "等待用户确认",
			Client:         client,
			Mapper:         &fakeRoundExecutionMapper{},
			IdleTimeout:    100 * time.Millisecond,
			IdlePauseState: idlePause.Snapshot,
		})
		done <- err
	}()

	select {
	case <-client.receiveStart:
	case <-time.After(time.Second):
		t.Fatal("round 未开始接收 runtime 消息")
	}
	idlePause.SetPaused(true)
	select {
	case err := <-done:
		t.Fatalf("人工交互待确认期间不应触发 idle timeout: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	idlePause.SetPaused(false)
	select {
	case err := <-done:
		t.Fatalf("人工交互结束后应重新获得完整 idle 窗口: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	select {
	case err := <-done:
		if !errors.Is(err, ErrRoundStreamIdleTimeout) {
			t.Fatalf("恢复计时后的错误不正确: %v", err)
		}
		if client.interrupts != 1 || client.disconnects != 1 {
			t.Fatalf("恢复后的真实 idle timeout 应中止 client: interrupts=%d disconnects=%d", client.interrupts, client.disconnects)
		}
	case <-time.After(time.Second):
		t.Fatal("人工交互结束后 idle timer 未恢复")
	}
}
