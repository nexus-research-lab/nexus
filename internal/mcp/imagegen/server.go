// Package imagegenmcp 提供 nexus MCP 的图片生成工具组。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package imagegenmcp

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/imagegen/tool"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// BuildTools 根据当前 Agent 会话上下文构建工具定义。
func BuildTools(svc contract.Service, sctx contract.ServerContext) []sdktool.Tool {
	return tool.BuildAll(svc, sctx)
}
