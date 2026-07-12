package automationmcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/tool"
)

// NewServer 根据当前会话上下文构建 nexus_automation MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "2.0.0", tool.BuildAll(svc, sctx))
}
