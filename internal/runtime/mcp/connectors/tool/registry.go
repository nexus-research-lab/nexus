package tool

import (
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/connectors/contract"
)

// BuildAll 汇集全部 connector MCP 工具。
func BuildAll(svc contract.Service, sctx contract.ServerContext) []agentclient.MCPTool {
	return []agentclient.MCPTool{
		list(svc, sctx),
		call(svc, sctx),
	}
}
