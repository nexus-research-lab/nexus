// INPUT: 模型生成的标题与自包含 HTML fragment。
// OUTPUT: show_widget 接收确认。
// POS: nexus MCP 中的生成式 UI 工具组；生成规则由 visualize Skill 提供，HTML 只由前端沙箱执行。
package visualize

import (
	"context"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildTools 创建对所有 Agent 可用的生成式 UI 工具定义。
func BuildTools() []sdktool.Tool {
	return []sdktool.Tool{{
		Name:        "show_widget",
		Description: "把自包含 HTML fragment 流式渲染到最终回复。仅在已加载 visualize Skill 且可视化优于正文或表格时调用；传入简短 title 与按短 style、可见内容、script 顺序组织的 widget_code。",
		SearchHint:  "render show interactive visualization chart diagram dashboard simulator HTML widget",
		AlwaysLoad:  true,
		InputSchema: showWidgetSchema(),
		Annotations: &sdktool.ToolAnnotations{
			ReadOnly:      true,
			ReadOnlyHint:  true,
			OpenWorld:     true,
			OpenWorldHint: true,
		},
		Handler: showWidget,
	}}
}

func showWidget(_ context.Context, input map[string]any) (sdktool.ToolResult, error) {
	title, _ := input["title"].(string)
	widgetCode, _ := input["widget_code"].(string)
	if strings.TrimSpace(title) == "" || strings.TrimSpace(widgetCode) == "" {
		return sdktool.ToolResult{
			Content: []map[string]any{{
				"type": "text",
				"text": "show_widget requires a title and non-empty widget_code",
			}},
			IsError: true,
		}, nil
	}
	payload := map[string]any{"accepted": true}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": `{"accepted":true}`}},
		StructuredContent: payload,
	}, nil
}

func showWidgetSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "界面的简短标题。",
			},
			"widget_code": map[string]any{
				"type":        "string",
				"description": "自包含 HTML fragment，不含 document 标签；短 style、可见内容、script 依次输出。可内联 CSS/JavaScript，也可加载任意 HTTPS 网络与 CDN 资源。",
			},
		},
		"required":             []string{"title", "widget_code"},
		"additionalProperties": false,
	}
}
