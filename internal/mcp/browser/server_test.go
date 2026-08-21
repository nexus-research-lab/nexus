package browser

import (
	"context"
	"reflect"
	"slices"
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

func TestBrowserSchemaConstrainsCommandsByAction(t *testing.T) {
	schema := browserSchema()
	alternatives := schema["anyOf"].([]any)
	want := map[string][]string{
		"network":   {"start", "stop", "list", "detail"},
		"console":   {"start", "stop", "list"},
		"dialog":    {"get", "accept", "dismiss"},
		"downloads": {"list", "wait", "show"},
		"clipboard": {"read", "write"},
	}
	got := make(map[string][]string, len(want))
	for _, rawAlternative := range alternatives {
		alternative := rawAlternative.(map[string]any)
		properties := alternative["properties"].(map[string]any)
		actions := properties["action"].(map[string]any)["enum"].([]string)
		commands, constrained := properties["cmd"]
		if !constrained {
			for action := range want {
				if slices.Contains(actions, action) {
					t.Fatalf("普通 action 分支不应包含命令 action %q", action)
				}
			}
			continue
		}
		if len(actions) != 1 {
			t.Fatalf("命令分支 action = %v", actions)
		}
		got[actions[0]] = commands.(map[string]any)["enum"].([]string)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("browser command constraints = %v, want %v", got, want)
	}
}
