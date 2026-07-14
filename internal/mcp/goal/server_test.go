package goalmcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/goal/contract"
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
	if len(tools) != 4 {
		t.Fatalf("tools count = %d, want 4", len(tools))
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
		if _, ok := meta["anthropic/alwaysLoad"]; ok {
			t.Fatalf("%s should stay deferred", name)
		}
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing inputSchema", name)
		}
		encoded, err := json.Marshal(schema)
		if err != nil {
			t.Fatalf("%s inputSchema is not JSON-serializable: %v", name, err)
		}
		if strings.Contains(string(encoded), `"required":null`) {
			t.Fatalf("%s inputSchema marshaled invalid required:null: %s", name, encoded)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("%s inputSchema did not round-trip as JSON: %v", name, err)
		}
		if _, ok := decoded["required"].([]any); !ok {
			t.Fatalf("%s inputSchema.required = %#v, want JSON array", name, decoded["required"])
		}
	}
}
