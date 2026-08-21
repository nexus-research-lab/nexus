package browser

import (
	"context"
	"reflect"
	"testing"

	browsersvc "github.com/nexus-research-lab/nexus/internal/service/browser"
)

func TestBrowserStatusDoesNotRequireExtensionConnection(t *testing.T) {
	server := NewServer(browsersvc.NewService(), "session-a", "round-a", "Agent A", nil)
	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "browser",
			"arguments": map[string]any{
				"action": "status",
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleMessage error = %v", err)
	}
	result := response["result"].(map[string]any)
	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("status 不应失败: %+v", result)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["connected"] != false || structured["protocol_version"] != browsersvc.ProtocolVersion {
		t.Fatalf("status = %+v", structured)
	}
}

func TestBrowserSchemaIncludesCompleteActionSet(t *testing.T) {
	properties := browserSchema()["properties"].(map[string]any)
	action := properties["action"].(map[string]any)
	values := action["enum"].([]string)
	if expected := browsersvc.SupportedActions(); !reflect.DeepEqual(values, expected) {
		t.Fatalf("browser schema actions = %v, want %v", values, expected)
	}
}
