package tool

import (
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
)

// BuildAll 汇集全部 connector MCP 工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []sdkmcp.Tool {
	return []sdkmcp.Tool{
		list(svc, sctx),
		call(svc, sctx),
	}
}
