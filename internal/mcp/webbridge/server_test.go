package webbridge

import (
	"context"
	"testing"

	webbridgesvc "github.com/nexus-research-lab/nexus/internal/service/webbridge"
)

func TestComputerStatusDoesNotRequireExtensionConnection(t *testing.T) {
	server := NewServer(webbridgesvc.NewService(), "session-a", "Agent A", nil)
	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "computer",
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
	if structured["connected"] != false || structured["protocol_version"] != webbridgesvc.ProtocolVersion {
		t.Fatalf("status = %+v", structured)
	}
}

func TestComputerSchemaIncludesCompleteActionSet(t *testing.T) {
	properties := computerSchema()["properties"].(map[string]any)
	action := properties["action"].(map[string]any)
	values := action["enum"].([]string)
	available := make(map[string]bool, len(values))
	for _, value := range values {
		available[value] = true
	}
	for _, expected := range []string{
		"navigate", "find_tab", "evaluate", "network", "snapshot", "click", "fill",
		"mouse_click", "cdp", "key_type", "send_keys", "screenshot", "save_as_pdf",
		"upload", "list_tabs", "close_tab", "close_session",
	} {
		if !available[expected] {
			t.Fatalf("computer schema 缺少 action %q", expected)
		}
	}
}
