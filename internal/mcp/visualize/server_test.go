package visualize

import (
	"context"
	"testing"
)

func TestVisualizeToolsExposeGuidelinesAndRenderWidget(t *testing.T) {
	tools := buildTools()
	if len(tools) != 2 || tools[0].Name != "visualize_read_me" || tools[1].Name != "show_widget" {
		t.Fatalf("visualize tools = %+v", tools)
	}
	for _, tool := range tools {
		if !tool.AlwaysLoad || tool.Annotations == nil || !tool.Annotations.ReadOnly {
			t.Fatalf("tool %q must stay always-loaded and read-only", tool.Name)
		}
	}

	result, err := tools[1].Handler(context.Background(), map[string]any{
		"i_have_seen_read_me": true,
		"title":               "参数曲线",
		"widget_code":         `<svg><circle r="20" /></svg>`,
	})
	if err != nil || result.IsError || result.StructuredContent["rendered"] != true {
		t.Fatalf("show_widget result = %+v, err = %v", result, err)
	}

	invalid, err := tools[1].Handler(context.Background(), map[string]any{
		"i_have_seen_read_me": true,
		"title":               "空界面",
	})
	if err != nil || !invalid.IsError {
		t.Fatalf("empty widget_code must fail: result=%+v err=%v", invalid, err)
	}
}
