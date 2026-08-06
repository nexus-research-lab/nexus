// INPUT: Execution Orchestration service 与当前 runtime identity。
// OUTPUT: 固定注册十二个模型语义工具的 nexus_execution SDK MCP server。
// POS: Execution MCP 装配入口；机器状态迁移保留在 service/hooks。
package executionmcp

import (
	"github.com/nexus-research-lab/nexus/internal/mcp/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/execution/tool"
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
)

// NewServer builds a session-bound Execution Orchestration MCP server.
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, contract.ServerVersion, tool.BuildAll(svc, sctx))
}
