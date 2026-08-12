package visualize

import (
	"context"
	"strings"
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
	readMe, err := tools[0].Handler(context.Background(), map[string]any{
		"modules": []any{"chart", "diagram", "chart"},
	})
	if err != nil || readMe.IsError {
		t.Fatalf("visualize_read_me result = %+v, err = %v", readMe, err)
	}
	guidelines, _ := readMe.Content[0]["text"].(string)
	for _, section := range []string{
		"# Nexus Generative UI",
		"## Chart module",
		"## Diagram module",
	} {
		if strings.Count(guidelines, section) != 1 {
			t.Fatalf("visualize_read_me must include %q once", section)
		}
	}
	if strings.Contains(guidelines, "## Interactive module") {
		t.Fatal("visualize_read_me returned an unrequested module")
	}
	for _, guidance := range []string{
		"Canvas APIs cannot resolve CSS var(...) strings",
		"Assigning canvas.width or canvas.height clears the bitmap",
		"tool result only confirms delivery to the client",
	} {
		if !strings.Contains(guidelines, guidance) {
			t.Fatalf("visualization guidelines missing %q", guidance)
		}
	}
	invalidReadMe, err := tools[0].Handler(context.Background(), map[string]any{
		"modules": []any{},
	})
	if err != nil || !invalidReadMe.IsError {
		t.Fatalf("empty read_me modules must fail: result=%+v err=%v", invalidReadMe, err)
	}

	result, err := tools[1].Handler(context.Background(), map[string]any{
		"i_have_seen_read_me": true,
		"title":               "参数曲线",
		"widget_code":         `<svg><circle r="20" /></svg>`,
	})
	if err != nil || result.IsError || result.StructuredContent["accepted"] != true {
		t.Fatalf("show_widget result = %+v, err = %v", result, err)
	}
	if result.StructuredContent["rendered"] != nil {
		t.Fatalf("show_widget must not claim browser rendering succeeded: %+v", result)
	}

	invalid, err := tools[1].Handler(context.Background(), map[string]any{
		"i_have_seen_read_me": true,
		"title":               "空界面",
	})
	if err != nil || !invalid.IsError {
		t.Fatalf("empty widget_code must fail: result=%+v err=%v", invalid, err)
	}
}
