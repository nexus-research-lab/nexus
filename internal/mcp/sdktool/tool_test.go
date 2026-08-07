package sdktool

import (
	"context"
	"testing"
)

func TestSimpleSDKMCPServerInvokesContextHandler(t *testing.T) {
	var captured *CallContext
	server := NewSimpleSDKMCPServer("test", "1.0.0", []Tool{{
		Name:        "mutate",
		Description: "mutate",
		InputSchema: map[string]any{"type": "object"},
		ContextHandler: func(
			_ context.Context,
			_ map[string]any,
			callContext *CallContext,
		) (ToolResult, error) {
			captured = callContext
			return ToolResult{}, nil
		},
	}})
	_, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "mutate",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if captured == nil {
		t.Fatal("context handler did not receive a call context")
	}
}
