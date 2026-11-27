package exec

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
