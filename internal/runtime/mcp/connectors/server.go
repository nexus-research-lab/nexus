// Package connectormcp 提供 nexus_connectors MCP server 入口。
package connectormcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/runtime/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/tool"
)

// NewServer 根据当前上下文构建 nexus_connectors MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
