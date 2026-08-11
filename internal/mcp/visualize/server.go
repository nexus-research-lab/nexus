// INPUT: 模型生成的标题与自包含 HTML fragment。
// OUTPUT: visualize_read_me 指南或 show_widget 渲染确认。
// POS: Nexus 生成式 UI 的模型能力入口；实际 HTML 只由前端沙箱执行。
package visualize

import (
	"context"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// ServerName 是生成式 UI 内建 MCP server 的注册名。
const ServerName = "nexus_visualize"

const visualizationGuidelines = `Create a custom interactive visual only when it communicates the answer better than ordinary prose or a table.

Workflow
1. Call show_widget with i_have_seen_read_me=true, a concise title, and one self-contained HTML fragment in widget_code.
2. Return the widget instead of repeating the same visualization in Markdown.

Widget contract
- Return a fragment only. Do not include html, head, or body tags.
- Inline the widget's CSS and JavaScript. SVG, Canvas, DOM, Web Components, and external CDN libraries are supported.
- Network access and external resources are allowed without a domain allowlist. Prefer established HTTPS CDNs.
- The fragment runs in an isolated iframe and cannot access the Nexus page, cookies, storage, or parent DOM.
- Completed markup and inline interactions remain live while streaming, but script elements run only after the complete tool input arrives. Put scripts after the markup and keep native controls useful before script initialization.
- Make the layout responsive at 320px width and avoid fixed viewport dimensions.
- Use accessible labels, keyboard-operable controls, visible focus, and reduced-motion fallbacks.
- Keep interaction local to the widget. Do not assume a host API or postMessage bridge.

Nexus visual language
- Make the visual feel native to the surrounding answer: flat, quiet, compact, and content-first.
- Keep the outermost background transparent. Use neutral surfaces, one-pixel borders, and 8px or 12px radii; avoid gradients, glass, glow, heavy shadows, and decorative hero layouts.
- Use only 400 and 500 font weights. Body text is 14px to 16px, labels are at least 12px, and controls are compact rather than oversized.
- Use the accent color only for selection, focus, or the primary data series. Prefer one neutral ramp and at most two categorical color ramps.
- Put explanations, introductions, and summaries in the normal assistant response. widget_code contains the visual and its local controls, not duplicate prose or a second page title.
- Let content determine height. Do not create nested scrolling, fixed-position overlays, or viewport-sized shells.

Theme variables
- --nexus-background
- --nexus-surface
- --nexus-surface-hover
- --nexus-text
- --nexus-muted
- --nexus-border
- --nexus-accent
- --nexus-accent-contrast
- --nexus-font-sans
- --nexus-font-mono
- --nexus-radius-md
- --nexus-radius-lg

Use these variables for the main palette, then derive secondary colors with color-mix().`

// NewServer 创建对所有 Agent 可用的 nexus_visualize MCP server。
func NewServer() *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(ServerName, "1.0.0", buildTools())
}

func buildTools() []sdktool.Tool {
	readOnly := &sdktool.ToolAnnotations{
		ReadOnly:     true,
		ReadOnlyHint: true,
	}
	return []sdktool.Tool{
		{
			Name:        "visualize_read_me",
			Description: "返回创建对话内交互式图表、图解、模拟器和自定义界面的完整规则。首次调用 show_widget 前必须先读取。",
			SearchHint:  "interactive visualization chart diagram dashboard simulator custom UI widget guidelines",
			AlwaysLoad:  true,
			InputSchema: emptyObjectSchema(),
			Annotations: readOnly,
			Handler: func(context.Context, map[string]any) (sdktool.ToolResult, error) {
				return textResult(visualizationGuidelines), nil
			},
		},
		{
			Name:        "show_widget",
			Description: "把模型生成的 HTML fragment 流式渲染为当前对话中的交互界面。适用于图表、图解、仪表盘、模拟器和视觉解释。",
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
		},
	}
}

func showWidget(_ context.Context, input map[string]any) (sdktool.ToolResult, error) {
	seenReadMe, _ := input["i_have_seen_read_me"].(bool)
	title, _ := input["title"].(string)
	widgetCode, _ := input["widget_code"].(string)
	if !seenReadMe || strings.TrimSpace(title) == "" || strings.TrimSpace(widgetCode) == "" {
		return sdktool.ToolResult{
			Content: []map[string]any{{
				"type": "text",
				"text": "show_widget requires visualize_read_me, a title, and non-empty widget_code",
			}},
			IsError: true,
		}, nil
	}
	payload := map[string]any{"rendered": true}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": `{"rendered":true}`}},
		StructuredContent: payload,
	}, nil
}

func emptyObjectSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

func showWidgetSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"i_have_seen_read_me": map[string]any{
				"type":        "boolean",
				"description": "调用 visualize_read_me 后固定传 true。",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "界面的简短标题。",
			},
			"widget_code": map[string]any{
				"type":        "string",
				"description": "自包含 HTML fragment，可内联 CSS/JavaScript，也可加载网络与 CDN 资源。",
			},
		},
		"required": []string{
			"i_have_seen_read_me",
			"title",
			"widget_code",
		},
		"additionalProperties": false,
	}
}

func textResult(text string) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": text}},
	}
}
