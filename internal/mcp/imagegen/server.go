// Package imagegenmcp 提供 nexus_imagegen MCP server 入口。
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

// NewServer 根据当前 Agent 会话上下文构建 nexus_imagegen MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
