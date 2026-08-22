package browser

import (
	"context"
	"reflect"
	"slices"
	"strings"
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

func TestBrowserSchemaRequiresBatchActions(t *testing.T) {
	schema := browserSchema()
	properties := schema["properties"].(map[string]any)
	actions := properties["actions"].(map[string]any)
	items := actions["items"].(map[string]any)
	itemProperties := items["properties"].(map[string]any)
	values := itemProperties["action"].(map[string]any)["enum"].([]string)
	if expected := browsersvc.BatchActions(); !reflect.DeepEqual(values, expected) {
		t.Fatalf("batch schema actions = %v, want %v", values, expected)
	}

	found := false
	for _, rawAlternative := range schema["anyOf"].([]any) {
		alternative := rawAlternative.(map[string]any)
		alternativeProperties := alternative["properties"].(map[string]any)
		actionValues := alternativeProperties["action"].(map[string]any)["enum"].([]string)
		if slices.Contains(actionValues, "batch") {
			found = reflect.DeepEqual(alternative["required"], []string{"action", "actions"})
		}
	}
	if !found {
		t.Fatal("batch schema 应要求 action 和 actions")
	}
}

func TestRenderResultKeepsBrowserDataCompactForTheModel(t *testing.T) {
	snapshot := strings.Repeat("button \"Open\" @e1\n", 1_000)
	result := map[string]any{
		"snapshot":      snapshot,
		"snapshot_type": "full",
		"snapshot_id":   1,
		"nodes":         1_000,
		"total_nodes":   1_000,
		"refs":          1_000,
	}
	rendered := renderResult("snapshot", result)
	text := rendered.Content[0]["text"].(string)
	if strings.Contains(text, `"snapshot":`) || !strings.Contains(text, `"snapshot_type":"full"`) {
		t.Fatalf("snapshot model text = %q", text[:min(len(text), 200)])
	}
	if len(text) >= len(snapshot) || !strings.Contains(text, `"model_text_truncated":true`) {
		t.Fatalf("snapshot model text length = %d, raw = %d", len(text), len(snapshot))
	}
	if rendered.StructuredContent["snapshot"] != snapshot {
		t.Fatal("完整 snapshot 应保留在 structured content")
	}
	batch := renderResult("batch", map[string]any{
		"completed": 1, "final_snapshot": result,
	})
	batchText := batch.Content[0]["text"].(string)
	if strings.Contains(batchText, `"final_snapshot":`) || !strings.Contains(batchText, `"snapshot_type":"full"`) {
		t.Fatalf("batch model text = %q", batchText[:min(len(batchText), 200)])
	}

	page := renderResult("page_content", map[string]any{
		"format": "text", "url": "https://example.com", "content": "visible page text", "length": 17,
	})
	pageText := page.Content[0]["text"].(string)
	if strings.Contains(pageText, `"content":`) || !strings.HasSuffix(pageText, "\nvisible page text") {
		t.Fatalf("page_content model text = %q", pageText)
	}
}
