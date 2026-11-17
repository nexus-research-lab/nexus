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
			}, nil, time.Time{})
			if result.TerminalStatus != "error" || result.ResultSubtype != "error" || result.ErrorMessage != test.want {
				t.Fatalf("result = %#v, want terminal error %q", result, test.want)
			}
		})
	}
}

func TestExecuteRoundRejectsAssistantWithoutResult(t *testing.T) {
	for _, stopReason := range []string{"end_turn", "stop_sequence", "max_tokens", "tool_use"} {
		t.Run(stopReason, func(t *testing.T) {
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
							"stop_reason": stopReason,
						}},
					}},
				},
			})
			if !errors.Is(err, ErrRoundStreamClosedBeforeTerminal) {
				t.Fatalf("assistant 回复结束不能替代整轮 result: %v", err)
			}
		})
	}
}

// TestExecuteRoundWaitsForResultAfterAssistantContinues 覆盖回复结束后继续用工具的真实流边界。
func TestExecuteRoundWaitsForResultAfterAssistantContinues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &fakeRoundExecutionClient{messages: make(chan sdkprotocol.ReceivedMessage, 3)}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeAssistant}
	mapper := &fakeRoundExecutionMapper{results: []RoundMapResult{
		{DurableMessages: []protocol.Message{{"role": "assistant", "is_complete": true, "stop_reason": "end_turn"}}},
		{DurableMessages: []protocol.Message{{"role": "assistant", "is_complete": true, "stop_reason": "tool_use"}}},
		{TerminalStatus: "finished", ResultSubtype: "success", DurableMessages: []protocol.Message{{"role": "result", "subtype": "success"}}},
	}}
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
			client.messages <- sdkprotocol.ReceivedMessage{Type: sdkprotocol.MessageTypeResult, Result: &sdkprotocol.ResultMessage{
				Subtype: "success", Usage: map[string]any{"total_tokens": int64(7)},
			}}
		}
	}()
	result, err := ExecuteRound(ctx, RoundExecutionRequest{Query: "继续", Client: client, Mapper: mapper})
	if err != nil || mapper.index != 3 || result.TerminalStatus != "finished" || !result.CompletedByAssistant || result.Usage.TotalTokens != 7 {
		t.Fatalf("必须消费本轮 result 和 usage 后才能完成: mapped=%d result=%+v err=%v", mapper.index, result, err)
	}
}
