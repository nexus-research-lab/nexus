package connectormcp

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/connectors/contract"
)

func listTools(t *testing.T) []map[string]any {
	t.Helper()
	server := NewServer(nil, contract.ServerContext{})
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", resp)
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatalf("tools not []map, got %T", result["tools"])
	}
	return tools
}

func TestToolsListIncludesDeferredMetadata(t *testing.T) {
	tools := listTools(t)
	if len(tools) == 0 {
		t.Fatal("expected connector tools")
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
		if _, hasAlwaysLoad := meta["anthropic/alwaysLoad"]; hasAlwaysLoad {
			t.Fatalf("%s should stay deferred", name)
		}
	}
}
