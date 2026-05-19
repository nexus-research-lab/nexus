// Package connectormcp 提供 nexus_connectors MCP server 入口。
package connectormcp

import (
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/tool"
)

// NewServer 根据当前上下文构建 nexus_connectors MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdkmcp.SimpleSDKMCPServer {
	return sdkmcp.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
