// INPUT: 模型生成的标题与自包含 HTML fragment。
// OUTPUT: visualize_read_me 指南或 show_widget 接收确认。
// POS: Nexus 生成式 UI 的模型能力入口；实际 HTML 只由前端沙箱执行。
package visualize

import (
	"context"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// ServerName 是生成式 UI 内建 MCP server 的注册名。
const ServerName = "nexus_visualize"

const visualizationCoreGuidelines = `# Nexus Generative UI

Create a custom interactive visual only when it communicates the answer better than ordinary prose or a table.

Workflow
1. Call visualize_read_me before each widget with every relevant module. Call it again when the visualization category changes.
2. Call show_widget with i_have_seen_read_me=true, a concise title, and one self-contained HTML fragment in widget_code.
3. Put explanation and conclusions in the normal response around the widget. Do not repeat the visual as Markdown.
4. The tool result only confirms delivery to the client. Do not claim that rendering succeeded.

Widget contract
- Return a fragment only. Do not include html, head, or body tags.
- Inline the widget's CSS and JavaScript. SVG, Canvas, DOM, Web Components, and external CDN libraries are supported.
- Network access and external resources are allowed without a domain allowlist. Prefer established HTTPS CDNs.
- The fragment runs in an isolated iframe and cannot access the Nexus page, cookies, storage, or parent DOM.
- Streaming order is short style, visible content, then scripts last. Scripts run only after the complete tool input arrives. Keep native controls and static content useful before initialization.
- Before calling show_widget, check every inline script for unmatched quotes, backticks, brackets, and incomplete blocks. Prefer short functions over one monolithic script.
- Separate source data, derived calculations, rendering, and event binding. Avoid constructing large interfaces through nested template literals or long HTML string concatenation; update existing DOM nodes when practical.
- Use the smallest implementation that fully answers the request. Split independent visuals into separate show_widget calls with short prose between them, but keep one cohesive widget when its views share state.
- Make the layout responsive at 320px width and avoid fixed viewport dimensions.
- Use accessible labels, keyboard-operable controls, visible focus, and reduced-motion fallbacks.
- Keep interaction local to the widget. Do not assume a host API or postMessage bridge.

Nexus visual language
- Make the visual feel native to the surrounding answer: flat, quiet, compact, and content-first.
- Keep the outermost background transparent. Use neutral surfaces, one-pixel borders, and 8px or 12px radii; avoid gradients, glass, glow, heavy shadows, and decorative hero layouts.
- Use only 400 and 500 font weights. Body text is 14px to 16px, labels are at least 12px, and controls are compact rather than oversized.
- Use the accent color only for selection, focus, or the primary data series. Prefer one neutral ramp and at most two categorical color ramps.
- Do not recreate Nexus chrome or add a second page title inside the widget. widget_code contains the visual and its local controls, not duplicate prose.
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
- --nexus-chart-1 through --nexus-chart-5
- --nexus-font-sans
- --nexus-font-mono
- --nexus-radius-md
- --nexus-radius-lg

Use these variables for CSS and SVG. Canvas APIs cannot resolve CSS var(...) strings; read their computed values first.`

const interactiveGuidelines = `## Interactive module

- Begin with visible, meaningful markup. JavaScript enhances it after streaming; it must not be required to reveal the entire widget.
- Use native button, input, select, and range controls with explicit labels. Every visible control must change the visual immediately and support keyboard input.
- Keep one plain state object. Derive displayed values from it, then render through short idempotent functions.
- Bind events once with addEventListener. Do not mix inline handlers, duplicated listeners, and global mutable callbacks.
- Prefer changing textContent, attributes, classes, SVG paths, or chart data over replacing a large subtree with innerHTML.
- Animate the visualization, not the surrounding UI. Use 150-400ms transitions and honor prefers-reduced-motion.`

const chartGuidelines = `## Chart module

- Use SVG or native DOM for small charts. Use Chart.js only when axes, tooltips, or multiple dynamic series justify it.
- Wrap each canvas in a position:relative container with an explicit height. Do not set CSS height on canvas. Use responsive:true and maintainAspectRatio:false.
- Canvas cannot resolve CSS variables. Read --nexus-chart-1 through --nexus-chart-5 with getComputedStyle(document.documentElement).getPropertyValue(name).trim().
- Assigning canvas.width or canvas.height clears the bitmap and resets the context. Set the backing size only during initialization or a real resize, never inside draw or coordinate helpers.
- Give every canvas a unique id. Keep the chart instance and guard initialization so CDN onload plus an immediate fallback cannot create it twice.
- Load established UMD builds over HTTPS. Put the library script before the initializer, use onload to call a named init function, and also call it when the global already exists.
- Controls must update chart data and call chart.update(). Disable library legends when a compact HTML legend communicates values more clearly.
- Round displayed values consistently, label axes and units, and pad plot ranges so points and labels are not clipped.`

const diagramGuidelines = `## Diagram module

- Prefer one responsive SVG with width=100% and a complete viewBox. Put defs and arrow markers before visible nodes so streaming connectors are valid.
- Choose the structure that matches the idea: flow for sequence, hierarchy for ownership, cycle for feedback, matrix for two dimensions, timeline for change, or side-by-side for comparison.
- Keep node titles to five words when possible and at most four full-size nodes per row. Put detail in surrounding prose or an interactive inspector.
- Calculate the viewBox from the lowest element plus padding. Keep labels inside bounds and account for text-anchor direction.
- Connect edges from node boundaries, use a shared marker, and verify no edge crosses unrelated nodes or text.
- Use neutral structure plus no more than two categorical chart colors. Encode status with text or shape as well as color.
- For interactive diagrams, mutate classes and SVG attributes on existing elements instead of rebuilding the SVG.`

const mockupGuidelines = `## Mockup module

- Reproduce the requested product surface, not an entire decorative landing page. Omit browser chrome, fake sidebars, duplicate titles, and ornamental hero areas unless they are the subject.
- Use a clear reading order: compact controls, primary content, then secondary detail. Prefer dividers and whitespace over nested cards.
- Use CSS Grid for comparable metrics and Flexbox for compact controls. Keep labels and values aligned to common axes.
- Use one-pixel --nexus-border boundaries, --nexus-surface for restrained grouping, and 8px or 12px radii. Avoid gradients, glass, glow, and heavy shadows.
- Empty, loading, selected, warning, and error states must remain distinguishable in both light and dark themes.
- On narrow widths, reflow columns and allow tables or timelines to scroll only when their data cannot remain legible otherwise.`

const artGuidelines = `## Art module

- Prefer SVG for illustrations and finite diagrams; use Canvas for continuous animation, dense particles, or pixel-level drawing.
- For Canvas, initialize the backing store once per actual size change, scale for devicePixelRatio once, and keep coordinate conversion separate from drawing.
- Resolve Nexus color variables to concrete values before assigning fillStyle, strokeStyle, shadows, or gradients.
- Run animation through requestAnimationFrame, cap particle or object counts, and pause or simplify when prefers-reduced-motion is enabled.
- Keep controls and captions as accessible HTML outside Canvas. Do not make essential meaning depend on pixels alone.
- Keep the outer surface transparent and let the artwork, not decorative containers, carry the composition.`

var visualizationModuleGuidelines = map[string]string{
	"art":         artGuidelines,
	"chart":       chartGuidelines,
	"diagram":     diagramGuidelines,
	"interactive": interactiveGuidelines,
	"mockup":      mockupGuidelines,
}

var visualizationModuleNames = []string{
	"interactive",
	"chart",
	"mockup",
	"art",
	"diagram",
}

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
			Description: "按类别返回生成式 UI 规则。每个 widget 前选择所有相关模块；类别变化时重新调用。interactive=控件与模拟器，chart=数据图表，mockup=产品界面，art=SVG/Canvas 艺术，diagram=流程与结构图。",
			SearchHint:  "interactive visualization chart diagram dashboard simulator custom UI widget guidelines",
			AlwaysLoad:  true,
			InputSchema: visualizeReadMeSchema(),
			Annotations: readOnly,
			Handler:     loadVisualizationGuidelines,
		},
		{
			Name:        "show_widget",
			Description: "把自包含 HTML fragment 流式渲染到最终回复。调用前必须用 visualize_read_me 加载本次相关模块；输出按短 style、可见内容、script 的顺序，脚本保持短小并在提交前检查语法。",
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

func loadVisualizationGuidelines(
	_ context.Context,
	input map[string]any,
) (sdktool.ToolResult, error) {
	modules, ok := input["modules"].([]any)
	if !ok || len(modules) == 0 {
		return sdktool.ToolResult{
			Content: []map[string]any{{
				"type": "text",
				"text": "visualize_read_me requires at least one guideline module",
			}},
			IsError: true,
		}, nil
	}

	sections := []string{visualizationCoreGuidelines}
	seen := make(map[string]bool, len(modules))
	for _, value := range modules {
		module, ok := value.(string)
		guidelines, exists := visualizationModuleGuidelines[module]
		if !ok || !exists {
			return sdktool.ToolResult{
				Content: []map[string]any{{
					"type": "text",
					"text": "visualize_read_me received an unknown guideline module",
				}},
				IsError: true,
			}, nil
		}
		if seen[module] {
			continue
		}
		seen[module] = true
		sections = append(sections, guidelines)
	}
	return textResult(strings.Join(sections, "\n\n")), nil
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
	payload := map[string]any{"accepted": true}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": `{"accepted":true}`}},
		StructuredContent: payload,
	}, nil
}

func visualizeReadMeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"modules": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
					"enum": visualizationModuleNames,
				},
				"description": "本次界面涉及的全部类别，可组合选择。",
			},
		},
		"required":             []string{"modules"},
		"additionalProperties": false,
	}
}

func showWidgetSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"i_have_seen_read_me": map[string]any{
				"type":        "boolean",
				"description": "仅在本次 widget 前已按相关 modules 调用 visualize_read_me 时传 true。",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "界面的简短标题。",
			},
			"widget_code": map[string]any{
				"type":        "string",
				"description": "自包含 HTML fragment，不含 document 标签；短 style、可见内容、script 依次输出。可内联 CSS/JavaScript，也可加载任意 HTTPS 网络与 CDN 资源。",
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
