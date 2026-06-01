package goalmcp

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/goal/contract"
)

func TestToolsListIncludesModelVisibleMetadata(t *testing.T) {
	server := NewServer(nil, contract.ServerContext{})
	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", response)
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatalf("tools not []map, got %T", result["tools"])
	}
	if len(tools) != 3 {
		t.Fatalf("tools count = %d, want 3", len(tools))
	}
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		meta, ok := tool["_meta"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing _meta", name)
		}
		hint, _ := meta["anthropic/searchHint"].(string)
		if strings.TrimSpace(hint) == "" {
			t.Fatalf("%s missing anthropic/searchHint", name)
		}
		if alwaysLoad, _ := meta["anthropic/alwaysLoad"].(bool); !alwaysLoad {
			t.Fatalf("%s should be anthropic/alwaysLoad", name)
		}
	}
}
