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
