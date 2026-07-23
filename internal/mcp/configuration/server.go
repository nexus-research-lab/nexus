// INPUT: configuration 服务与当前主智能体 runtime 上下文。
// OUTPUT: nexus_config 进程内 MCP server。
// POS: configuration MCP 顶层装配入口。
package configurationmcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/configuration/tool"
)

// NewServer 创建配置控制 MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
