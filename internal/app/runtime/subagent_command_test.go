package runtime

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	subagentcommand "github.com/nexus-research-lab/nexus/internal/mcp/command/subagent"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

func TestSubagentCommandUsesSDKMetadataAndPropagatesNativeError(t *testing.T) {
	calls := 0
	handler := subagentcommand.NewHandler(command.Actor{OwnerUserID: "owner", AgentID: "agent", SessionKey: "parent", RoundID: "round", LeaseSessionKey: "parent", LeaseRoundID: "round", SourceContextType: "agent"}, func(_ context.Context, id, operation string, _ map[string]any) (map[string]any, error) {
		calls++
		if id != "sdk-tool" || operation != "spawn" {
			t.Fatalf("caller=%q operation=%q", id, operation)
		}
		return map[string]any{"is_error": true, "content": "assignment revoked"}, nil
	})
	server := newRoundScopedMCPServer(context.Background(), sdktool.NewSimpleSDKMCPServer("nexus", "1", []sdktool.Tool{command.NewTool(handler)}))
	params := map[string]any{"name": "command", "arguments": map[string]any{"domain": "subagent", "action": "invoke", "operation": "spawn", "request_id": "request-1", "input": map[string]any{"description": "task", "prompt": "evidence"}}}
	request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": params}
	response, err := server.HandleMessage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || response["result"].(map[string]any)["isError"] != true {
		t.Fatalf("missing metadata accepted: %#v", response)
	}
	params["_meta"] = map[string]any{"claudecode/toolUseId": "sdk-tool"}
	response, err = server.HandleMessage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || response["result"].(map[string]any)["isError"] != true {
		t.Fatalf("native rejection lost: %#v", response)
	}
}
