package exec

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestTerminalRoundResultReturnsErrorMessage(t *testing.T) {
	tests := []struct {
		name    string
		message protocol.Message
		want    string
	}{
		{
			name: "result text",
			message: protocol.Message{
				"role":     "result",
				"subtype":  "error",
				"is_error": true,
				"result":   "Failed to authenticate. API Error: 401",
			},
			want: "Failed to authenticate. API Error: 401",
		},
		{
			name: "errors array",
			message: protocol.Message{
				"role":     "result",
				"subtype":  "error",
				"is_error": true,
				"errors":   []any{"client: stream closed before result message"},
			},
			want: "client: stream closed before result message",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := terminalRoundResult(RoundMapResult{
				DurableMessages: []protocol.Message{test.message},
				TerminalStatus:  "error",
				ResultSubtype:   "error",
			}, nil, nil, time.Time{})
			if result.TerminalStatus != "error" || result.ResultSubtype != "error" || result.ErrorMessage != test.want {
				t.Fatalf("result = %#v, want terminal error %q", result, test.want)
			}
		})
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
