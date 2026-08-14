// Package feishudocxmcp 提供 nexus_feishu_docx MCP server 入口。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package feishudocxmcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/feishudocx/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/feishudocx/tool"
)

// NewServer 根据当前上下文构建 nexus_feishu_docx MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
