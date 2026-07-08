package exec

import (
	"context"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

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
			runtimectx.NewContextualInputBlock("goal", "Continue.", 0, map[string]string{"goal_id": "goal-1"}),
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

func TestExecuteRoundInlinesContextOnlyInternalTurn(t *testing.T) {
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
			runtimectx.NewContextualInputBlock("goal", "Continue.", 0, map[string]string{"goal_id": "goal-1"}),
		},
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{TerminalStatus: "finished", ResultSubtype: "success"}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if len(client.contextInput) != 0 {
		t.Fatalf("contextInput = %#v, want context-only turn inlined", client.contextInput)
	}
	wantPrompt := "<internal_context source=\"goal\">\nContinue.\n</internal_context>"
	if len(client.queryPrompts) != 1 || client.queryPrompts[0] != wantPrompt {
		t.Fatalf("queryPrompts = %#v, want inlined internal context", client.queryPrompts)
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
			runtimectx.NewContextualInputBlock("goal", "Continue.", 0, nil),
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
			runtimectx.NewContextualInputBlock("goal", "Continue.", 0, nil),
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

func TestExecuteRoundLeavesUnknownContentShapeWhenInternalContextUnsupported(t *testing.T) {
	client := &fakeRoundExecutionClient{
		sessionID:  "sdk-session-context-unknown",
		contextErr: agentclient.ErrUnsupportedCapability,
		messages:   make(chan sdkprotocol.ReceivedMessage, 1),
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      "result-context-unknown",
		Result: &sdkprotocol.ResultMessage{
			Subtype: "success",
		},
	}
	close(client.messages)
	content := map[string]any{"prompt": "用户输入"}

	_, err := ExecuteRound(context.Background(), RoundExecutionRequest{
		Content: content,
		ContextualInputs: []ContextualInputBlock{
			runtimectx.NewContextualInputBlock("goal", "Continue.", 0, nil),
		},
		Client: client,
		Mapper: &fakeRoundExecutionMapper{
			results: []RoundMapResult{{TerminalStatus: "finished", ResultSubtype: "success"}},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteRound 失败: %v", err)
	}
	if len(client.queryPrompts) != 0 || len(client.queryContent) != 1 {
		t.Fatalf("queryPrompts=%#v queryContent=%#v, want one unchanged content payload", client.queryPrompts, client.queryContent)
	}
	got, ok := client.queryContent[0].(map[string]any)
	if !ok || got["prompt"] != "用户输入" || got["text"] != nil {
		t.Fatalf("queryContent[0] = %#v, want original map without injected text", client.queryContent[0])
	}
}
