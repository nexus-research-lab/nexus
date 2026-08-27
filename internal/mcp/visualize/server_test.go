package visualize

import (
	"context"
	"testing"
)

func TestVisualizeExposesOnlyShowWidget(t *testing.T) {
	tools := BuildTools()
	if len(tools) != 1 || tools[0].Name != "show_widget" {
		t.Fatalf("visualize tools = %+v", tools)
	}
	tool := tools[0]
	if !tool.AlwaysLoad || tool.Annotations == nil || !tool.Annotations.ReadOnly || !tool.Annotations.OpenWorld {
		t.Fatalf("show_widget must stay always-loaded, read-only, and open-world: %+v", tool)
	}
	properties, _ := tool.InputSchema["properties"].(map[string]any)
	if _, exists := properties["i_have_seen_read_me"]; exists {
		t.Fatalf("show_widget schema still exposes retired read-me handshake: %+v", properties)
	}

	result, err := tool.Handler(context.Background(), map[string]any{
		"title":       "参数曲线",
		"widget_code": `<svg><circle r="20" /></svg>`,
	})
	if err != nil || result.IsError || result.StructuredContent["accepted"] != true {
		t.Fatalf("show_widget result = %+v, err = %v", result, err)
	}
	if result.StructuredContent["rendered"] != nil {
		t.Fatalf("show_widget must not claim browser rendering succeeded: %+v", result)
	}

	invalid, err := tool.Handler(context.Background(), map[string]any{"title": "空界面"})
	if err != nil || !invalid.IsError {
		t.Fatalf("empty widget_code must fail: result=%+v err=%v", invalid, err)
	}
}
