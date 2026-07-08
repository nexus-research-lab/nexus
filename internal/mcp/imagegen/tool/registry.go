// Package tool 定义 nexus_imagegen MCP 暴露的图片生成工具。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildAll 汇集全部图片生成工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return []sdktool.Tool{
		generate(svc, sctx),
		edit(svc, sctx),
	}
}
