// Package tool 定义 nexus MCP 暴露的图片生成工具。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool

import (
	"encoding/json"

	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

const (
	searchHintGenerateImage = "image generate 生成图片 位图 插画 照片 hero mockup product asset"
	searchHintEditImage     = "image edit 编辑图片 改图 mask 背景替换 修图 inpaint"
)

// BuildAll 汇集全部图片生成工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		generate(svc, sctx),
		edit(svc, sctx),
	}
}

func jsonResult(payload map[string]any) sdktool.ToolResult {
	data, err := json.Marshal(payload)
	if err != nil {
		return errorResult(err)
	}
	return sdktool.ToolResult{
		Content:           []map[string]any{{"type": "text", "text": string(data)}},
		StructuredContent: payload,
	}
}

func errorResult(err error) sdktool.ToolResult {
	return sdktool.ToolResult{
		Content: []map[string]any{{"type": "text", "text": err.Error()}},
		IsError: true,
	}
}
