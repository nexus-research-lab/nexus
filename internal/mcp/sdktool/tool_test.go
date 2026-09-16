package sdktool

import (
	"context"
	"testing"
)

// Verify the real SDK server boundary rather than directly invoking a handler
// with manufactured CallContext: this catches incompatible Bridge dependencies.
func TestSDKMCPPreservesRuntimeToolUseIdentity(t *testing.T) {
	var received []string
	server := NewSimpleSDKMCPServer("nexus", "1", []Tool{{Name: "send_message", ContextHandler: func(_ context.Context, args map[string]any, call *CallContext) (ToolResult, error) {
		received = append(received, call.ToolUseID)
		return ToolResult{Content: []map[string]any{{"type": "text", "text": "ok"}}}, nil
	}}})
	for _, id := range []string{"real-call-a", "real-call-a", "real-call-b", ""} {
		meta := map[string]any{}
		if id != "" {
			meta["claudecode/toolUseId"] = id
		}
		response, err := server.HandleMessage(context.Background(), map[string]any{"jsonrpc": "2.0", "id": "call-tool", "method": "tools/call", "params": map[string]any{"name": "send_message", "_meta": meta, "arguments": map[string]any{"content": "same body", "tool_use_id": "forged"}}})
		if err != nil || response["error"] != nil {
			t.Fatalf("call: %v %v", response, err)
		}
	}
	want := []string{"real-call-a", "real-call-a", "real-call-b", ""}
	if len(received) != len(want) {
		t.Fatalf("received %v", received)
	}
	for i, id := range want {
		if received[i] != id {
			t.Fatalf("call %d identity = %q, want %q", i, received[i], id)
		}
	}
}
