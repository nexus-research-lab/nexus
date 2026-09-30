// INPUT: 模型生成的标题与自包含 HTML fragment。
// OUTPUT: show_widget 接收确认。
// POS: nexus MCP 中的生成式 UI 工具组；生成规则由 visualize Skill 提供，HTML 只由前端沙箱执行。
package visualize

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// MaxWidgetCodeBytes is the hard transport budget for one Generative UI
// fragment. Keeping the limit here makes an over-sized visual fail before it
// becomes a blank or partially persisted conversation block.
const MaxWidgetCodeBytes = 256 << 10

// MaxInlineImageBytes bounds the encoded data:image payload in one widget.
// Base64 expands the source bytes, so callers must measure the serialized URL.
const MaxInlineImageBytes = 192 << 10

var inlineImageDataURL = regexp.MustCompile(`(?i)data:image/[^;,\s]+;base64,[A-Za-z0-9+/=]+`)

// BuildTools 创建对所有 Agent 可用的生成式 UI 工具定义。
func BuildTools() []sdktool.Tool {
	return []sdktool.Tool{{
		Name:        "show_widget",
		Description: fmt.Sprintf("把自包含 HTML fragment 流式渲染到最终回复。仅在已加载 visualize Skill 且可视化优于正文或表格时调用；传入简短 title 与按短 style、可见内容、script 顺序组织的 widget_code。widget_code 不得超过 %d KiB UTF-8，内联 data:image URL 合计不得超过 %d KiB；照片预览每次最多一个 contact sheet 或六张图片。超限前先拆分。工具回执只代表接收，不代表每张图片已渲染。", MaxWidgetCodeBytes/1024, MaxInlineImageBytes/1024),
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
	if reason := widgetBudgetError(widgetCode); reason != "" {
		return sdktool.ToolResult{
			Content: []map[string]any{{
				"type": "text",
				"text": reason,
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
				"description": fmt.Sprintf("自包含 HTML fragment，不含 document 标签；短 style、可见内容、script 依次输出。可内联 CSS/JavaScript，也可加载任意 HTTPS 网络与 CDN 资源。最终 UTF-8 大小上限 %d KiB，内联 data:image URL 合计上限 %d KiB；照片预览最多一个 contact sheet 或六张图片。", MaxWidgetCodeBytes/1024, MaxInlineImageBytes/1024),
			},
		},
		"required":             []string{"title", "widget_code"},
		"additionalProperties": false,
	}
}

func widgetBudgetError(widgetCode string) string {
	if size := len([]byte(widgetCode)); size > MaxWidgetCodeBytes {
		return fmt.Sprintf("show_widget widget_code 超过 %d KiB UTF-8 上限（当前 %d KiB）；请先压缩图片或拆分预览批次。", MaxWidgetCodeBytes/1024, (size+1023)/1024)
	}
	inlineBytes := 0
	for _, match := range inlineImageDataURL.FindAllString(widgetCode, -1) {
		inlineBytes += len([]byte(match))
	}
	if inlineBytes > MaxInlineImageBytes {
		return fmt.Sprintf("show_widget 内联 data:image URL 超过 %d KiB 上限（当前 %d KiB）；请降低预览质量或拆分图片批次。", MaxInlineImageBytes/1024, (inlineBytes+1023)/1024)
	}
	return ""
}
