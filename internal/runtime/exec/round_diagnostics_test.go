package exec

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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

func TestExecuteRoundReturnsInterruptedWhenSDKAborted(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		queryErr:  agentclient.ErrAborted,
		messages:  make(chan sdkprotocol.ReceivedMessage),
	}

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "你好",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{},
	})
	if !errors.Is(err, ErrRoundInterrupted) {
		t.Fatalf("期望返回 ErrRoundInterrupted，实际 %v", err)
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

func TestExecuteRoundReturnsLastStreamStopDiagnostics(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 4),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: "sdk-session-1",
		Stream: &sdkprotocol.StreamEvent{
			Event: map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    "assistant-1",
					"model": "kimi-k2.6",
				},
			},
		},
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
			Event: map[string]any{
				"type": "message_stop",
			},
		},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeTaskProgress,
		SessionID: "sdk-session-1",
	}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "需要工具",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{}, {}, {}, {}},
		},
	})
	if !errors.Is(err, ErrRoundStreamClosedBeforeTerminal) {
		t.Fatalf("期望 ErrRoundStreamClosedBeforeTerminal，实际 %v", err)
	}
	var streamErr *RoundStreamClosedError
	if !errors.As(err, &streamErr) {
		t.Fatalf("期望 RoundStreamClosedError，实际 %T %[1]v", err)
	}
	stop := streamErr.LastStreamStop
	if !stop.Observed ||
		stop.MessageIndex != 3 ||
		stop.MessagesAfter != 1 ||
		stop.ProgressMessagesAfter != 1 ||
		stop.ConversationMessagesAfter != 0 ||
		stop.PassiveMessagesAfter != 0 ||
		stop.UnknownMessagesAfter != 0 ||
		stop.StopReason != "tool_use" ||
		stop.SessionID != "sdk-session-1" ||
		stop.MessageID != "assistant-1" ||
		stop.Model != "kimi-k2.6" {
		t.Fatalf("message_stop 诊断字段不正确: %+v", stop)
	}
	if !strings.Contains(err.Error(), "messages_after_last_stream_stop=1") {
		t.Fatalf("错误字符串缺少 message_stop 诊断: %v", err)
	}
	if !strings.Contains(err.Error(), "progress_after_last_stream_stop=1") {
		t.Fatalf("错误字符串缺少 message_stop 分类诊断: %v", err)
	}
	fields := RoundStreamStopDiagnosticLogFields(stop)
	if len(fields) == 0 {
		t.Fatalf("message_stop 日志字段为空: %+v", stop)
	}
}

func TestExecuteRoundClassifiesMessagesAfterLastStreamStop(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 6),
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
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeToolProgress}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeToolUseSummary}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeUnknown}
	close(client.messages)

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Query:  "需要工具",
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{}, {}, {}, {}, {}},
		},
	})
	if !errors.Is(err, ErrRoundStreamClosedBeforeTerminal) {
		t.Fatalf("期望 ErrRoundStreamClosedBeforeTerminal，实际 %v", err)
	}
	var streamErr *RoundStreamClosedError
	if !errors.As(err, &streamErr) {
		t.Fatalf("期望 RoundStreamClosedError，实际 %T %[1]v", err)
	}
	stop := streamErr.LastStreamStop
	if stop.MessagesAfter != 3 ||
		stop.ProgressMessagesAfter != 1 ||
		stop.PassiveMessagesAfter != 1 ||
		stop.UnknownMessagesAfter != 1 ||
		stop.ConversationMessagesAfter != 0 {
		t.Fatalf("message_stop 后消息分类不正确: %+v", stop)
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
